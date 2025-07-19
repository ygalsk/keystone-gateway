# Keystone Gateway

**Der intelligente Reverse Proxy für moderne DevOps-Teams**

[![Go Version](https://img.shields.io/badge/go-1.19+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Version](https://img.shields.io/badge/version-v1.2.0-orange.svg)](https://github.com/ygalsk/keystone-gateway/releases)

---

## 🎯 Was ist Keystone Gateway?

Keystone Gateway ist ein **leichtgewichtiger, erweiterbarer Reverse Proxy**, der speziell für KMUs und DevOps-Teams entwickelt wurde. Er kombiniert Einfachheit mit Flexibilität durch eine einzigartige **Lua-Scripting-Architektur**.

### 🚀 Kernphilosophie

```
┌─────────────────────────────────────────────┐
│            Keystone Gateway Core            │
│  • Schnelles Routing (300+ req/sec)        │
│  • Einfache YAML-Konfiguration             │
│  • Health-basiertes Load Balancing         │
│  • Single Binary Deployment                │
└─────────────────┬───────────────────────────┘
                  │ Optional
┌─────────────────▼───────────────────────────┐
│           Lua Script Layer                  │
│  • CI/CD Pipeline Integration               │
│  • Canary Deployments                      │
│  • Custom Business Logic                   │
│  • Community-driven Features               │
└─────────────────────────────────────────────┘
```

### ✨ Was macht Keystone einzigartig?

- **🎯 Simplicity First**: Einfache Nutzung ohne Scripting-Zwang
- **⚡ Lua-Powered**: Optionale enterprise-grade Features via Lua-Scripts
- **🔧 Community-driven**: Erweitere Funktionen ohne Core-Änderungen
- **📦 Single Binary**: Deployment in Sekunden, keine Dependencies
- **🏢 Multi-Tenant**: Perfekt für Agenturen und KMUs

---

## 🚀 Quick Start

### Installation

```bash
# Option 1: Download Binary
wget https://github.com/ygalsk/keystone-gateway/releases/latest/download/keystone-gateway-linux-amd64
chmod +x keystone-gateway-linux-amd64
sudo mv keystone-gateway-linux-amd64 /usr/local/bin/keystone-gateway

# Option 2: Build from Source
git clone https://github.com/ygalsk/keystone-gateway.git
cd keystone-gateway
go build -o keystone-gateway main.go
```

### Basis-Konfiguration

```yaml
# config.yaml
tenants:
  - name: "my-app"
    domains: ["app.example.com"]
    services:
      - name: "backend"
        url: "http://localhost:3000"
        health: "/health"
```

### Starten

```bash
keystone-gateway -config config.yaml -addr :8080
```

**🎉 Fertig!** Deine App ist unter `http://localhost:8080` erreichbar.

---

## 🛠️ Erweiterte Features (Optional)

### Lua-Scripting für CI/CD

Aktiviere enterprise-grade Features ohne Core-Komplexität:

```yaml
# Advanced config with Lua script
tenants:
  - name: "production-api"
    domains: ["api.example.com"]
    lua_script: "scripts/canary-deployment.lua"  # Optional!
    services:
      - name: "api-stable"
        url: "http://api-v1.0:8080"
        labels:
          version: "stable"
      - name: "api-canary"
        url: "http://api-v1.1:8080"
        labels:
          version: "canary"
```

```lua
-- scripts/canary-deployment.lua
function on_route_request(request, backends)
    local version = request.headers["X-Version"] or "stable"
    
    if version == "canary" then
        return filter_backends(backends, "canary")
    else
        return filter_backends(backends, "stable") 
    end
end
```

### Unterstützte Lua-Features (Coming Soon)

- 🚀 **Canary Deployments**: Progressive traffic shifting
- 🔄 **Blue/Green Deployments**: Zero-downtime releases
- 🔗 **CI/CD Integration**: GitLab, GitHub, Jenkins webhooks
- 🎯 **A/B Testing**: Custom traffic routing logic
- 🔧 **Request Transformation**: Headers, paths, authentication

---

## 📊 Performance & Monitoring

### Performance-Ziele

| Version | Req/sec | Latency | Memory | Features |
|---------|---------|---------|---------|----------|
| v1.2.0  | 159     | 6.3ms   | 8MB     | Host routing ✅ |
| v1.2.1  | 300+    | <5ms    | 10MB    | Chi router ⏳ |
| v1.3.0  | 500+    | <4ms    | 12MB    | Lua scripts 🔮 |

### Health Monitoring

```bash
# Check gateway status
curl http://localhost:8080/health

# Performance metrics
curl http://localhost:8080/metrics
```

---

## 🌟 Use Cases

### 🏢 **Für Agenturen**
- Multi-Client-Hosting auf einer Infrastruktur
- Einfache Subdomain-basierte Trennung
- Zentrale Health-Überwachung

### 🚀 **Für DevOps-Teams**
- Canary Deployments via Lua-Scripts
- CI/CD Pipeline Integration
- Blue/Green Deployment Automation

### 🏭 **Für KMUs**
- Einfache Load Balancer Alternative
- Keine Enterprise-Lizenzkosten
- Single Binary Deployment

### 🔧 **Für Entwickler**
- lokale Multi-Service-Entwicklung
- Einfache Microservice-Orchestrierung
- API Gateway für Prototyping

---

## 📚 Dokumentation

### 🎯 **Roadmap & Planung**
- [**MACHBARE_ROADMAP.md**](docs/MACHBARE_ROADMAP.md) - Aktuelle Entwicklungsplanung mit Lua-Vision
- [**PROJECT_SUMMARY.md**](docs/PROJECT_SUMMARY.md) - Vollständiger Projektüberblick

### 🏗️ **Implementierung & Entwicklung**
- [**FRAMEWORK_ANALYSIS.md**](docs/FRAMEWORK_ANALYSIS.md) - Chi Router vs. stdlib Analyse
- [**v1.2.1-CHI-PLAN.md**](docs/v1.2.1-CHI-PLAN.md) - Chi Router Integration Plan
- [**PERFORMANCE.md**](docs/PERFORMANCE.md) - Benchmarks und Optimierungen

### 📋 **Vollständige Dokumentation**
Siehe [**docs/README.md**](docs/README.md) für alle verfügbaren Dokumente.

---

## 🤝 Community & Support

### 🔗 Links
- **GitHub**: https://github.com/ygalsk/keystone-gateway
- **Issues**: https://github.com/ygalsk/keystone-gateway/issues
- **Releases**: https://github.com/ygalsk/keystone-gateway/releases

### 💡 Contributing
1. Fork das Repository
2. Erstelle einen Feature Branch
3. Committe deine Änderungen
4. Erstelle eine Pull Request

### 🐛 Bug Reports
Nutze GitHub Issues für Bug Reports und Feature Requests.

---

## 📜 License

MIT License - siehe [LICENSE](LICENSE) für Details.

---

## 🎯 Roadmap 2025

- **Q3 2025**: Chi Router Integration + Performance (v1.3.0)
- **Q4 2025**: Wildcard Domains + Monitoring (v1.4.0)
- **Q1 2026**: Lua Scripting Engine (v2.0.0)
- **Q2 2026**: Community Scripts & Ecosystem (v2.0.0+)

**Vision**: Der einzige Reverse Proxy, den KMUs und DevOps-Teams jemals brauchen werden.

---

*Made with ❤️ for the DevOps community*
