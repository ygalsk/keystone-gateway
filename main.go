package main

import (
    "flag"
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
    Tenants []Tenant `yaml:"tenants"`
}

type Tenant struct {
    Name       string    `yaml:"name"`
    PathPrefix string    `yaml:"path_prefix"` // e.g. "/acme/"
    Interval   int       `yaml:"health_interval"` // seconds, optional (default 10)
    Services   []Service `yaml:"services"`
}

type Service struct {
    Name   string `yaml:"name"`
    URL    string `yaml:"url"`    // e.g. "http://127.0.0.1:8080"
    Health string `yaml:"health"` // e.g. "/health" (relative)
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
    rr       uint64 // round‑robin counter
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
    if err := yaml.Unmarshal(f, &cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}

// -------------------------
// Health checking
// -------------------------

func startHealthChecks(t Tenant, router *tenantRouter) {
    interval := time.Duration(t.Interval) * time.Second
    if interval == 0 {
        interval = 10 * time.Second
    }
    go func() {
        ticker := time.NewTicker(interval)
        defer ticker.Stop()
        for {
            for i, svc := range t.Services {
                b := router.backends[i]
                target := svc.URL + "/" + strings.TrimPrefix(svc.Health, "/")
                client := http.Client{Timeout: 3 * time.Second}
                resp, err := client.Get(target)
                if err != nil || resp.StatusCode >= 400 {
                    b.alive.Store(false)
                } else {
                    b.alive.Store(true)
                }
                if resp != nil {
                    resp.Body.Close()
                }
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
    for i := 0; i < total; i++ {
        idx := int(atomic.AddUint64(&tr.rr, 1) % uint64(total))
        b := tr.backends[idx]
        if b.alive.Load() {
            return b
        }
    }
    // fallback: return first even if unhealthy
    return tr.backends[0]
}

// -------------------------
// HTTP handler
// -------------------------

func makeHandler(routers map[string]*tenantRouter) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Match tenant by longest prefix
        var matched *tenantRouter
        var matchedPrefix string
        for prefix, rt := range routers {
            if strings.HasPrefix(r.URL.Path, prefix) {
                if len(prefix) > len(matchedPrefix) {
                    matchedPrefix = prefix
                    matched = rt
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
        // Rewrite path: strip tenant prefix
        originalPrefix := matchedPrefix
        proxy.Director = func(req *http.Request) {
            req.URL.Scheme = backend.url.Scheme
            req.URL.Host = backend.url.Host
            req.URL.Path = strings.TrimPrefix(req.URL.Path, originalPrefix)
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
    cfgPath := flag.String("config", "config.yaml", "path to YAML config")
    addr := flag.String("addr", ":8080", "listen address")
    flag.Parse()

    cfg, err := loadConfig(*cfgPath)
    if err != nil {
        log.Fatalf("failed to load config: %v", err)
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
            // assume unhealthy until first check
            b.alive.Store(false)
            tr.backends = append(tr.backends, b)
        }
        routers[t.PathPrefix] = tr
        // start health checks per tenant
        startHealthChecks(t, tr)
        log.Printf("tenant %s loaded with %d service(s)", t.Name, len(tr.backends))
    }

    http.HandleFunc("/", makeHandler(routers))

    log.Printf("Keystone Gateway listening on %s", *addr)
    if err := http.ListenAndServe(*addr, nil); err != nil {
        log.Fatal(err)
    }
}