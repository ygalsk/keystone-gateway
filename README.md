# Keystone Gateway Documentation

**Version:** 1.2.0  
**Author:** Daniel Kremer  
**Website:** https://keystone-gateway.dev

## 📋 Inhaltsverzeichnis

1. [Überblick](#überblick)
2. [Installation](#installation)
3. [Konfiguration](#konfiguration)
4. [Host-Based Routing](#host-based-routing)
5. [Deployment](#deployment)
6. [Monitoring](#monitoring)
7. [Troubleshooting](#troubleshooting)
8. [API Referenz](#api-referenz)
9. [Best Practices](#best-practices)
10. [Beispiele](#beispiele)
11. [FAQ](#faq)

---

## 🚀 Überblick

Keystone Gateway ist ein intelligenter Reverse Proxy, der speziell für KMUs entwickelt wurde. Es kombiniert die Einfachheit von traditionellen Proxies mit erweiterten Features wie Health-Checks, Multi-Tenant-Unterstützung und **Host-Based Routing**.

### ✨ Neue Features in v1.2.0

- **🌐 Host-Based Routing**: Route basierend auf Domain/Hostname
- **🔗 Hybrid Routing**: Kombiniere Host- und Path-basiertes Routing
- **📋 Multi-Domain Support**: Mehrere Domains pro Tenant
- **🔄 100% Backward Compatible**: Bestehende Konfigurationen funktionieren unverändert

### Warum Keystone Gateway?

- **🎯 KMU-fokussiert**: Keine Enterprise-Komplexität
- **⚡ Performance**: Go-basiert, minimal Overhead
- **🔧 Einfach**: YAML-Konfiguration, keine Scripting-Sprachen
- **🏥 Zuverlässig**: Health-basiertes Load Balancing
- **🏢 Multi-Tenant**: Perfekt für Agenturen
- **🌐 Flexibles Routing**: Path-, Host- und Hybrid-Routing

### Architektur

```
Internet → TLS-Proxy (Caddy/Nginx) → Keystone Gateway → Backend Services
                                           ↓
                    Host/Path/Hybrid Routing + Health Monitoring
                                           ↓
                                    Load Balancing
```

---

## 📦 Installation

### Systemanforderungen

- **OS**: Linux, macOS, Windows
- **RAM**: Minimum 64MB, empfohlen 128MB
- **CPU**: Single Core ausreichend
- **Disk**: 10MB für Binary + Config

### Option 1: Docker (Empfohlen)

```bash
# Neueste Version
docker run -d \
  --name keystone-gateway \
  -p 8080:8080 \
  -v $(pwd)/configs:/app/configs:ro \
  ygalsk/keystone-gateway:latest

# Spezifische Version
docker run -d \
  --name keystone-gateway \
  -p 8080:8080 \
  -v $(pwd)/configs:/app/configs:ro \
  ygalsk/keystone-gateway:v1.1.0
```

### Option 2: Binary Download

```bash
# Linux x64
wget https://github.com/ygalsk/keystone-gateway/releases/latest/download/keystone-gateway-linux-amd64
chmod +x keystone-gateway-linux-amd64
mv keystone-gateway-linux-amd64 /usr/local/bin/keystone-gateway

# macOS
wget https://github.com/ygalsk/keystone-gateway/releases/latest/download/keystone-gateway-darwin-amd64
chmod +x keystone-gateway-darwin-amd64
mv keystone-gateway-darwin-amd64 /usr/local/bin/keystone-gateway
```

### Option 3: Aus Source kompilieren

```bash
git clone https://github.com/ygalsk/keystone-gateway.git
cd keystone-gateway
go build -o keystone-gateway main.go
```

---

## ⚙️ Konfiguration

### Grundlegende Konfiguration

Keystone Gateway verwendet YAML-Dateien für die Konfiguration. Die Standard-Konfigurationsdatei ist `config.yaml`.

```yaml
# config.yaml
tenants:
  - name: "beispiel-tenant"
    path_prefix: "/api/"
    health_interval: 30
    services:
      - name: "web-service"
        url: "http://localhost:3000"
        health: "/health"
```

### Vollständige Konfigurationsoptionen

```yaml
# Vollständige config.yaml
tenants:
  - name: "production-app"              # Eindeutiger Tenant-Name
    path_prefix: "/prod/"               # URL-Präfix (muss mit / beginnen und enden)
    health_interval: 30                 # Health-Check Intervall in Sekunden (optional, default: 10)
    services:
      - name: "primary-server"          # Service-Name für Logging
        url: "http://web-1:8080"        # Backend-URL
        health: "/health"               # Health-Check Endpoint
      - name: "backup-server"
        url: "http://web-2:8080"
        health: "/status"
        
  - name: "staging-env"
    path_prefix: "/staging/"
    health_interval: 60                 # Längere Intervalle für Staging
    services:
      - name: "staging-app"
        url: "http://staging:3000"
        health: "/ping"
```

### Konfiguration validieren

```bash
# Konfiguration testen
keystone-gateway -config config.yaml -validate

# Trocken-Lauf (lädt Config, startet aber nicht)
keystone-gateway -config config.yaml -dry-run
```

---

## 🚀 Deployment

### Docker Compose (Empfohlen)

```yaml
# docker-compose.yml
version: '3.8'

services:
  keystone-gateway:
    image: ygalsk/keystone-gateway:latest
    container_name: keystone-gateway
    ports:
      - "8080:8080"
    volumes:
      - ./configs:/app/configs:ro
      - ./logs:/app/logs
    environment:
      - TZ=Europe/Berlin
      - LOG_LEVEL=info
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s
    restart: unless-stopped
    networks:
      - gateway-network

  # Beispiel Backend
  backend-service:
    image: nginx:alpine
    container_name: backend-service
    networks:
      - gateway-network

networks:
  gateway-network:
    driver: bridge
```

### Docker Swarm

```bash
# Swarm initialisieren
docker swarm init

# Stack deployen
docker stack deploy -c docker-compose.yml keystone

# Status prüfen
docker stack services keystone
docker service logs keystone_keystone-gateway
```

### Makefile Commands

Das Repository enthält nützliche Makefile-Befehle:

```bash
make help      # Alle verfügbaren Befehle anzeigen
make build     # Docker Image bauen
make start     # Gateway starten
make test      # End-to-End Tests ausführen
make stop      # Gateway stoppen
make logs      # Logs anzeigen
make status    # Service Status
make clean     # Aufräumen
```

### Systemd Service

```ini
# /etc/systemd/system/keystone-gateway.service
[Unit]
Description=Keystone Gateway
After=network.target

[Service]
Type=simple
User=keystone
Group=keystone
WorkingDirectory=/opt/keystone-gateway
ExecStart=/usr/local/bin/keystone-gateway -config /etc/keystone-gateway/config.yaml
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

```bash
# Service aktivieren
sudo systemctl daemon-reload
sudo systemctl enable keystone-gateway
sudo systemctl start keystone-gateway
sudo systemctl status keystone-gateway
```

---

## 📊 Monitoring

### Health Checks

Keystone Gateway bietet mehrere Health-Check Endpoints:

```bash
# Gateway Health
curl http://localhost:8080/health

# Detailed Status (JSON)
curl http://localhost:8080/status

# Metrics (Prometheus Format)
curl http://localhost:8080/metrics
```

### Logging

```yaml
# Log-Levels: debug, info, warn, error
environment:
  - LOG_LEVEL=info
  - LOG_FORMAT=json  # json oder text
```

### Prometheus Integration

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'keystone-gateway'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
    scrape_interval: 30s
```

### Grafana Dashboard

```json
{
  "dashboard": {
    "title": "Keystone Gateway",
    "panels": [
      {
        "title": "Requests per Second",
        "targets": [
          {
            "expr": "rate(keystone_http_requests_total[5m])"
          }
        ]
      },
      {
        "title": "Backend Health",
        "targets": [
          {
            "expr": "keystone_backend_health_status"
          }
        ]
      }
    ]
  }
}
```

---

## 🔧 Troubleshooting

### Häufige Probleme

#### 1. Backend nicht erreichbar

```bash
# Symptom: 502 Bad Gateway
# Lösung: Backend-URL prüfen
docker exec keystone-gateway wget -qO- http://backend:8080/health

# Logs prüfen
docker logs keystone-gateway
```

#### 2. Health-Checks fehlschlagen

```yaml
# Problem: Falscher Health-Endpoint
services:
  - name: "problematic-service"
    url: "http://backend:8080"
    health: "/health"  # Existiert dieser Endpoint?

# Lösung: Endpoint testen
curl http://backend:8080/health
```

#### 3. Konfiguration wird nicht geladen

```bash
# Volumes prüfen
docker inspect keystone-gateway | grep -A 5 Mounts

# Berechtigung prüfen
ls -la configs/
```

### Debug-Modus

```bash
# Detaillierte Logs
docker run -e LOG_LEVEL=debug ygalsk/keystone-gateway:latest

# In Container einsteigen
docker exec -it keystone-gateway sh
```

### Performance-Probleme

```bash
# Ressourcen-Verbrauch prüfen
docker stats keystone-gateway

# Verbindungen prüfen
netstat -an | grep 8080
```

---

## 📚 API Referenz

### Health Endpoints

| Endpoint | Methode | Beschreibung |
|----------|---------|-------------|
| `/health` | GET | Gateway Health Status |
| `/status` | GET | Detaillierter Status (JSON) |
| `/metrics` | GET | Prometheus Metriken |

### Beispiel Responses

```json
// GET /status
{
  "status": "healthy",
  "version": "1.1.0",
  "uptime": "2h30m15s",
  "tenants": [
    {
      "name": "production-app",
      "path_prefix": "/prod/",
      "services": [
        {
          "name": "primary-server",
          "url": "http://web-1:8080",
          "healthy": true,
          "last_check": "2025-01-18T10:30:00Z"
        }
      ]
    }
  ]
}
```

---

## 🏆 Best Practices

### Konfiguration

1. **Verwende aussagekräftige Namen**
   ```yaml
   tenants:
     - name: "customer-portal"  # ✅ Gut
     - name: "tenant1"          # ❌ Schlecht
   ```

2. **Health-Check Intervalle anpassen**
   ```yaml
   # Production: Häufige Checks
   health_interval: 10
   
   # Staging: Weniger häufig
   health_interval: 60
   ```

3. **Mehrere Backends pro Tenant**
   ```yaml
   services:
     - name: "primary"
       url: "http://web-1:8080"
       health: "/health"
     - name: "backup"
       url: "http://web-2:8080"
       health: "/health"
   ```

### Deployment

1. **Immer spezifische Versionen verwenden**
   ```yaml
   image: ygalsk/keystone-gateway:v1.1.0  # ✅ Gut
   image: ygalsk/keystone-gateway:latest  # ❌ Für Production
   ```

2. **Resource Limits setzen**
   ```yaml
   deploy:
     resources:
       limits:
         cpus: '0.50'
         memory: 128M
   ```

3. **Health Checks konfigurieren**
   ```yaml
   healthcheck:
     test: ["CMD", "wget", "--quiet", "--spider", "http://localhost:8080/health"]
     interval: 30s
     timeout: 10s
     retries: 3
   ```

### Monitoring

1. **Strukturierte Logs**
   ```bash
   LOG_FORMAT=json
   LOG_LEVEL=info
   ```

2. **Metrics sammeln**
   ```yaml
   # Prometheus scrape config
   scrape_interval: 30s
   scrape_timeout: 10s
   ```

---

## 💡 Beispiele

### Agentur-Setup

```yaml
# Mehrere Kunden verwalten
tenants:
  - name: "kunde-a-website"
    path_prefix: "/kunde-a/"
    health_interval: 30
    services:
      - name: "wordpress"
        url: "http://kunde-a-wp:80"
        health: "/wp-admin/admin-ajax.php"
      - name: "backup-server"
        url: "http://kunde-a-backup:80"
        health: "/health"
        
  - name: "kunde-b-shop"
    path_prefix: "/kunde-b/"
    health_interval: 15
    services:
      - name: "shopware"
        url: "http://kunde-b-shop:80"
        health: "/health"
```

### Staging/Production Setup

```yaml
tenants:
  - name: "production"
    path_prefix: "/api/"
    health_interval: 10
    services:
      - name: "api-server-1"
        url: "http://api-prod-1:8080"
        health: "/health"
      - name: "api-server-2"
        url: "http://api-prod-2:8080"
        health: "/health"
        
  - name: "staging"
    path_prefix: "/staging-api/"
    health_interval: 60
    services:
      - name: "staging-api"
        url: "http://api-staging:8080"
        health: "/health"
```

### Microservices Setup

```yaml
tenants:
  - name: "user-service"
    path_prefix: "/users/"
    health_interval: 20
    services:
      - name: "user-api"
        url: "http://user-service:3000"
        health: "/health"
        
  - name: "order-service"
    path_prefix: "/orders/"
    health_interval: 20
    services:
      - name: "order-api"
        url: "http://order-service:3000"
        health: "/health"
        
  - name: "payment-service"
    path_prefix: "/payments/"
    health_interval: 10  # Kritischer Service
    services:
      - name: "payment-api"
        url: "http://payment-service:3000"
        health: "/health"
```

---

## ❓ FAQ

### Allgemeine Fragen

**Q: Kann Keystone Gateway TLS terminieren?**
A: Nein, Keystone Gateway ist für internes Routing gedacht. Verwende Caddy oder Nginx als TLS-Proxy davor.

**Q: Wie viele Tenants kann ich haben?**
A: Unbegrenzt. Jeder Tenant hat minimalen Overhead.

**Q: Funktioniert es mit Kubernetes?**
A: Ja, aber Docker Swarm ist einfacher für KMUs.

**Q: Kann ich WebSockets proxyen?**
A: Ja, WebSockets werden automatisch unterstützt.

### Technische Fragen

**Q: Wie funktioniert das Load Balancing?**
A: Round-Robin zwischen gesunden Backends. Ungesunde werden übersprungen.

**Q: Was passiert wenn alle Backends down sind?**
A: Der erste Backend wird als Fallback verwendet (kann 502 zurückgeben).

**Q: Kann ich Health-Checks deaktivieren?**
A: Nein, Health-Checks sind ein Kernfeature von Keystone Gateway.

**Q: Unterstützt es HTTP/2?**
A: Ja, HTTP/2 wird automatisch unterstützt.

### Deployment-Fragen

**Q: Wie aktualisiere ich die Konfiguration?**
A: Einfach die config.yaml ändern und Container neustarten. Hot-Reload kommt in v1.2.

**Q: Kann ich mehrere Keystone Gateways load-balancen?**
A: Ja, einfach mehrere Instanzen hinter einem Load Balancer.

**Q: Wie sichere ich Keystone Gateway?**
A: Läuft als non-root User im Container. Zusätzlich TLS-Proxy verwenden.

---

## 🚀 Nächste Schritte

1. **[Playground testen](https://play.keystone-gateway.dev)**
2. **[GitHub Repository](https://github.com/ygalsk/keystone-gateway)**
3. **[Community beitreten](https://github.com/ygalsk/keystone-gateway/discussions)**

## 📞 Support

- **Email:** kontakt@keystone-gateway.dev
- **GitHub Issues:** https://github.com/ygalsk/keystone-gateway/issues
- **Documentation:** https://docs.keystone-gateway.dev

---

*Diese Dokumentation wird kontinuierlich aktualisiert. Letzte Änderung: Januar 2025*