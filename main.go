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
    Tenants []Tenant `yaml:"tenants"`
}

type Tenant struct {
    Name       string    `yaml:"name"`
    PathPrefix string    `yaml:"path_prefix,omitempty"` // e.g. "/acme/" - now optional
    Domains    []string  `yaml:"domains,omitempty"`     // NEW: e.g. ["app.example.com", "www.app.example.com"]
    Interval   int       `yaml:"health_interval"`       // seconds, optional (default 10)
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
// Configuration validation
// -------------------------

func isValidDomain(domain string) bool {
    // Simple domain validation - no empty strings, no spaces, contains a dot
    return domain != "" && !strings.Contains(domain, " ") && strings.Contains(domain, ".")
}

func validateTenant(t Tenant) error {
    // Must have either domains OR path_prefix (or both)
    if len(t.Domains) == 0 && t.PathPrefix == "" {
        return fmt.Errorf("tenant '%s' must specify either domains or path_prefix", t.Name)
    }
    
    // Validate domain formats
    for _, domain := range t.Domains {
        if !isValidDomain(domain) {
            return fmt.Errorf("invalid domain format: %s", domain)
        }
    }
    
    // Validate path_prefix format (existing logic)
    if t.PathPrefix != "" {
        if !strings.HasPrefix(t.PathPrefix, "/") || !strings.HasSuffix(t.PathPrefix, "/") {
            return fmt.Errorf("path_prefix must start and end with '/'")
        }
    }
    
    return nil
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
    
    // Validate each tenant
    for _, tenant := range cfg.Tenants {
        if err := validateTenant(tenant); err != nil {
            return nil, err
        }
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
// HTTP handler with host-based routing
// -------------------------

func extractHost(hostHeader string) string {
    // Remove port if present
    if colonIndex := strings.Index(hostHeader, ":"); colonIndex != -1 {
        return hostHeader[:colonIndex]
    }
    return hostHeader
}

func makeHandler(pathRouters map[string]*tenantRouter, hostRouters map[string]*tenantRouter, hybridRouters map[string]map[string]*tenantRouter) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        host := extractHost(r.Host)
        path := r.URL.Path
        
        var matched *tenantRouter
        var matchedPrefix string
        
        // Priority 1: Host + Path combination (hybrid routing)
        if hostPathMap, exists := hybridRouters[host]; exists {
            for prefix, rt := range hostPathMap {
                if strings.HasPrefix(path, prefix) {
                    if len(prefix) > len(matchedPrefix) {
                        matchedPrefix = prefix
                        matched = rt
                    }
                }
            }
        }
        
        // Priority 2: Host-only routing
        if matched == nil {
            if rt, exists := hostRouters[host]; exists {
                matched = rt
                matchedPrefix = "" // No prefix stripping for host-only routing
            }
        }
        
        // Priority 3: Path-only routing (backward compatibility)
        if matched == nil {
            for prefix, rt := range pathRouters {
                if strings.HasPrefix(path, prefix) {
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
        
        // Rewrite path: strip tenant prefix only for path-based routing
        proxy.Director = func(req *http.Request) {
            req.URL.Scheme = backend.url.Scheme
            req.URL.Host = backend.url.Host
            if matchedPrefix != "" {
                newPath := strings.TrimPrefix(req.URL.Path, matchedPrefix)
                // Ensure we always have a valid path
                if newPath == "" {
                    newPath = "/"
                }
                req.URL.Path = newPath
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
    cfgPath := flag.String("config", "config.yaml", "path to YAML config")
    addr := flag.String("addr", ":8080", "listen address")
    flag.Parse()

    cfg, err := loadConfig(*cfgPath)
    if err != nil {
        log.Fatalf("failed to load config: %v", err)
    }

    // Initialize routing tables
    pathRouters := make(map[string]*tenantRouter)     // path_prefix -> router
    hostRouters := make(map[string]*tenantRouter)     // domain -> router  
    hybridRouters := make(map[string]map[string]*tenantRouter) // domain -> (path_prefix -> router)

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
        
        // Route tenant based on configuration
        if len(t.Domains) > 0 && t.PathPrefix != "" {
            // Hybrid routing: both host and path
            for _, domain := range t.Domains {
                if hybridRouters[domain] == nil {
                    hybridRouters[domain] = make(map[string]*tenantRouter)
                }
                hybridRouters[domain][t.PathPrefix] = tr
                log.Printf("tenant %s: hybrid routing for domain %s with path %s", t.Name, domain, t.PathPrefix)
            }
        } else if len(t.Domains) > 0 {
            // Host-only routing
            for _, domain := range t.Domains {
                hostRouters[domain] = tr
                log.Printf("tenant %s: host-based routing for domain %s", t.Name, domain)
            }
        } else if t.PathPrefix != "" {
            // Path-only routing (backward compatibility)
            pathRouters[t.PathPrefix] = tr
            log.Printf("tenant %s: path-based routing for prefix %s", t.Name, t.PathPrefix)
        }
        
        // start health checks per tenant
        startHealthChecks(t, tr)
        log.Printf("tenant %s loaded with %d service(s)", t.Name, len(tr.backends))
    }

    http.HandleFunc("/", makeHandler(pathRouters, hostRouters, hybridRouters))

    log.Printf("Keystone Gateway listening on %s", *addr)
    if err := http.ListenAndServe(*addr, nil); err != nil {
        log.Fatal(err)
    }
}