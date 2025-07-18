package main

import (
    "flag"
    "fmt"
    "log"
    "net/http"
    "net/http/httputil"
    "net/url"
    "os"
    "strings"
    "sync/atomic"
    "time"

    "gopkg.in/yaml.v3"
)

// -------------------------
// Configuration structures
// -------------------------

type Config struct {
    Gateway GatewayConfig `yaml:"gateway"`
    Tenants []Tenant      `yaml:"tenants"`
    Headers HeaderConfig  `yaml:"headers"`
}

type GatewayConfig struct {
    Port           int           `yaml:"port"`
    HealthTimeout  time.Duration `yaml:"health_timeout"`
}

type HeaderConfig struct {
    TenantHeader string `yaml:"tenant_header"`
}

type Tenant struct {
    Name           string    `yaml:"name"`
    PathPrefix     string    `yaml:"path_prefix"`
    Host           string    `yaml:"host"`
    HealthInterval int       `yaml:"health_interval"`
    Services       []Service `yaml:"services"`
}

type Service struct {
    Name   string `yaml:"name"`
    URL    string `yaml:"url"`
    Health string `yaml:"health"`
}

// -------------------------
// Runtime backend object
// -------------------------

type backend struct {
    url   *url.URL
    alive atomic.Bool
}

type tenantRouter struct {
    backends []*backend
    rr       uint64
}

// -------------------------
// Load YAML configuration
// -------------------------

func loadConfig(path string) (*Config, error) {
    f, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    var cfg Config
    err = yaml.Unmarshal(f, &cfg)
    return &cfg, err
}

// -------------------------
// Health checking
// -------------------------

func checkHealth(b *backend, healthPath string, timeout time.Duration) {
    client := &http.Client{Timeout: timeout}
    resp, err := client.Get(b.url.String() + healthPath)
    if err != nil {
        b.alive.Store(false)
        return
    }
    defer resp.Body.Close()
    
    if resp.StatusCode >= 200 && resp.StatusCode < 300 {
        b.alive.Store(true)
    } else {
        b.alive.Store(false)
    }
}

func startHealthChecks(t Tenant, router *tenantRouter) {
    interval := time.Duration(t.HealthInterval) * time.Second
    if interval == 0 {
        interval = 30 * time.Second
    }
    
    go func() {
        ticker := time.NewTicker(interval)
        defer ticker.Stop()
        
        for {
            for i, b := range router.backends {
                go checkHealth(b, t.Services[i].Health, 5*time.Second)
            }
            <-ticker.C
        }
    }()
}

// -------------------------
// Reverse proxy logic
// -------------------------

func (tr *tenantRouter) nextBackend() *backend {
    total := len(tr.backends)
    if total == 0 {
        return nil
    }
    
    // Try to find a healthy backend
    for i := 0; i < total; i++ {
        idx := atomic.AddUint64(&tr.rr, 1) % uint64(total)
        if tr.backends[idx].alive.Load() {
            return tr.backends[idx]
        }
    }
    
    // Fallback: return first even if unhealthy
    return tr.backends[0]
}

// -------------------------
// HTTP handler
// -------------------------

func makeHandler(routers map[string]*tenantRouter, tenants []Tenant, headers HeaderConfig) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var matched *tenantRouter
        var matchedPrefix string
        
        // 1. Header-basierte Tenant-Erkennung
        if headers.TenantHeader != "" {
            tenantName := r.Header.Get(headers.TenantHeader)
            if tenantName != "" {
                for _, t := range tenants {
                    if t.Name == tenantName {
                        matched = routers[t.PathPrefix]
                        matchedPrefix = t.PathPrefix
                        break
                    }
                }
            }
        }
        
        // 2. Host-basierte Tenant-Erkennung
        if matched == nil {
            host := r.Host
            if colonIndex := strings.Index(host, ":"); colonIndex != -1 {
                host = host[:colonIndex]
            }
            
            for _, t := range tenants {
                if t.Host != "" && t.Host == host {
                    matched = routers[t.PathPrefix]
                    matchedPrefix = t.PathPrefix
                    break
                }
            }
        }
        
        // 3. Path-basierte Tenant-Erkennung
        if matched == nil {
            for prefix, rt := range routers {
                if strings.HasPrefix(r.URL.Path, prefix) {
                    if len(prefix) > len(matchedPrefix) {
                        matchedPrefix = prefix
                        matched = rt
                    }
                }
            }
        }
        
        if matched == nil {
            http.NotFound(w, r)
            return
        }
        
        backend := matched.nextBackend()
        if backend == nil {
            http.Error(w, "no backend available", http.StatusBadGateway)
            return
        }
        
        proxy := httputil.NewSingleHostReverseProxy(backend.url)
        originalPrefix := matchedPrefix
        proxy.Director = func(req *http.Request) {
            req.URL.Scheme = backend.url.Scheme
            req.URL.Host = backend.url.Host
            if strings.HasPrefix(req.URL.Path, originalPrefix) {
                req.URL.Path = strings.TrimPrefix(req.URL.Path, originalPrefix)
            }
            if backend.url.RawQuery == "" || req.URL.RawQuery == "" {
                req.URL.RawQuery = backend.url.RawQuery + req.URL.RawQuery
            } else {
                req.URL.RawQuery = backend.url.RawQuery + "&" + req.URL.RawQuery
            }
        }
        proxy.ServeHTTP(w, r)
    }
}

// -------------------------
// Main
// -------------------------

func main() {
    cfgPath := flag.String("config", "configs/config.yaml", "path to YAML config")
    addr := flag.String("addr", ":8080", "listen address")
    flag.Parse()

    cfg, err := loadConfig(*cfgPath)
    if err != nil {
        log.Fatalf("failed to load config: %v", err)
    }

    if cfg.Gateway.Port != 0 {
        *addr = fmt.Sprintf(":%d", cfg.Gateway.Port)
    }

    routers := make(map[string]*tenantRouter)

    for _, t := range cfg.Tenants {
        tr := &tenantRouter{}
        for _, svc := range t.Services {
            u, err := url.Parse(svc.URL)
            if err != nil {
                log.Fatalf("invalid service url: %v", err)
            }
            b := &backend{url: u}
            b.alive.Store(false)
            tr.backends = append(tr.backends, b)
        }
        routers[t.PathPrefix] = tr
        startHealthChecks(t, tr)
        log.Printf("tenant %s loaded with %d service(s) [path:%s, host:%s]", 
                   t.Name, len(tr.backends), t.PathPrefix, t.Host)
    }

    http.HandleFunc("/", makeHandler(routers, cfg.Tenants, cfg.Headers))

    log.Printf("Keystone Gateway listening on %s", *addr)
    if err := http.ListenAndServe(*addr, nil); err != nil {
        log.Fatal(err)
    }
}