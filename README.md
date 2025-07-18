# Keystone Gateway 🚀

**Das smarte Reverse Proxy-Gateway für KMUs**

Ein leichtgewichtiges, in Go geschriebenes Reverse-Proxy-System mit Health-basiertem Load Balancing und Multi-Tenant-Unterstützung – perfekt für Agenturen, KMUs & DevOps-Teams.

## 🔧 Was ist Keystone Gateway?

Keystone Gateway ist eine intelligente Reverse-Proxy-Lösung, die speziell für kleine und mittlere Unternehmen entwickelt wurde. Es bietet erweiterte Funktionen wie Health-Checks und Multi-Tenant-Unterstützung in einem einfach zu konfigurierenden Paket.

## 💡 Hauptfunktionen auf einen Blick

### 🔁 Health-basiertes Load Balancing
- Verteilt Anfragen nur an gesunde Backends (regelmäßige HTTP-Checks)
- Keine Downtime durch kranke Services
- Automatisches Failover bei Service-Ausfällen

### 🏢 Multi-Tenant-Unterstützung
- Strukturierte Trennung pro Kunde/Mandant
- Routing nach Pfadpräfix (z. B. `/kunde1/`, `/agenturX/`)
- Isolierte Service-Konfiguration pro Tenant

### 📄 Einfache YAML-Konfiguration
- Kein komplizierter Caddyfile oder JSON
- Klar & lesbar – ideal für DevOps-Automation
- Hot-Reload von Konfigurationsänderungen

### ⚙️ In Go entwickelt – Docker-ready
- Schnell, portabel, minimaler Ressourcenverbrauch
- Lässt sich einfach in bestehende Setups integrieren
- Single Binary ohne externe Abhängigkeiten

### 📊 Monitoring mit Prometheus (optional)
- Export von Metriken zu Health-Status & Traffic
- Ideal für Grafana & Alerting
- Detaillierte Performance-Überwachung

### 🧩 Ideal in Kombination mit Caddy
- Caddy als TLS-fähiger Entry Proxy
- Keystone übernimmt internes, intelligentes Routing
- Beste Performance durch spezialisierte Aufgabenteilung

## 🚀 Quick Start

### Installation & Start

```bash
# Repository klonen
git clone https://github.com/ygalsk/keystone-gateway.git
cd keystone-gateway

# Build und Start
make build
make start

# Mit Test-Backends
make test
```

## 🛠️ Makefile Commands

Simple commands for Docker Swarm deployment:

```bash
make help      # Available commands
make build     # Build the application
make start     # Deploy gateway stack
make test      # Deploy with test backends
make stop      # Remove the stack
make logs      # Show service logs
make status    # Show stack status
make clean     # Clean up everything
```

## 📖 Verwendung

### Routing-Beispiele

- `http://localhost:8080/acme/` → Routes zu acme-agency Services
- `http://localhost:8080/beta/` → Routes zu beta-shop Services

### Health-Checks

Das Gateway überwacht automatisch die konfigurierten Health-Endpoints:
- `/health` für acme-agency Services (alle 10 Sekunden)
- `/status` für beta-shop Services

## 🏗️ Architektur

```
Internet → Caddy (TLS) → Keystone Gateway → Backend Services
                              ↓
                        Health Monitoring
                              ↓
                        Load Balancing Logic
```

## ✅ Warum Keystone Gateway statt nur Caddy oder NGINX?

| Feature | Caddy/NGINX | Keystone Gateway |
|---------|-------------|------------------|
| Health-basierte Entscheidungen | ❌ | ✅ |
| Multi-Tenant-Logik | ❌ | ✅ |
| Einfache Konfiguration | ⚠️ | ✅ |
| Go-Performance | ❌ | ✅ |
| Speziell für KMUs | ❌ | ✅ |

**Keystone Gateway**: intelligent, modular, einfach konfigurierbar – speziell für kleine Teams & pragmatische Projekte.

## 🔧 Konfigurationsoptionen

### Tenant-Konfiguration

```yaml
tenants:
  - name: "tenant-name"           # Eindeutiger Tenant-Name
    path_prefix: "/prefix/"       # URL-Präfix für Routing
    health_interval: 30           # Health-Check Intervall in Sekunden
    services:
      - name: "service-name"      # Service-Bezeichnung
        url: "http://host:port"   # Backend-URL
        health: "/health"         # Health-Check Endpoint
```

## 🐳 Docker Support

### Fertiges Docker Image von Docker Hub

```bash
# Neueste Version direkt verwenden
docker run -d -p 8080:8080 \
  -v $(pwd)/configs:/app/configs \
  ygalsk/keystone-gateway:latest

# Spezifische Version
docker run -d -p 8080:8080 \
  -v $(pwd)/configs:/app/configs \
  ygalsk/keystone-gateway:v1.0
```

### Selbst bauen (Multi-Stage Alpine Dockerfile)

```bash
# Docker Image bauen (mit Makefile - empfohlen)
make docker-build

# Container starten
make docker-run

# Oder manuell:
docker build -t keystone-gateway .
docker run -d -p 8080:8080 \
  -v $(pwd)/configs:/app/configs \
  keystone-gateway

# Debug-Zugang (Alpine-Vorteil)
make docker-shell
# oder: docker exec -it keystone-gateway-dev sh
```

### Docker Compose (Empfohlen für Development)

```yaml
version: '3.8'
services:
  keystone-gateway:
    image: ygalsk/keystone-gateway:latest
    # build: .  # Uncomment um lokal zu bauen
    ports:
      - "8080:8080"
    volumes:
      - ./configs:/app/configs:ro
    restart: unless-stopped
    environment:
      - LOG_LEVEL=info
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
```

### 🔒 Alpine vs Distroless - Warum Alpine?

| Feature | Alpine Linux | Distroless |
|---------|-------------|------------|
| **Debugging** | ✅ Shell-Zugang für Troubleshooting | ❌ Kein Shell |
| **Flexibilität** | ✅ Runtime-Tools installierbar | ❌ Statische Runtime |
| **Monitoring** | ✅ Einfache Agent-Installation | ❌ Kompliziert |
| **DevOps-Friendly** | ✅ Bekannte Linux-Tools | ❌ Eingeschränkt |
| **Sicherheit** | ✅ Regelmäßige Updates | ✅ Minimal Surface |
| **Image-Größe** | ~8MB final | ~5MB final |

**Unsere Alpine-Implementation bietet:**
- 🛡️ **Hardened Security**: Non-root User, dumb-init, minimal packages
- 🔧 **Debug-Freundlich**: Shell-Zugang für Produktions-Troubleshooting  
- 📦 **Optimiert**: UPX-komprimierte Binary, Layer-Caching
- 🏥 **Health-Checks**: Integrierte Container-Gesundheitsprüfung
- ⚡ **Performance**: Aktuelle Go 1.22 + Alpine 3.19

## 📈 Monitoring & Metriken

Keystone Gateway exportiert Prometheus-Metriken für:
- Request-Anzahl pro Tenant/Service
- Response-Zeiten
- Health-Check Status
- Error-Rates

## 🤝 Contributing

1. Fork das Repository
2. Erstelle einen Feature-Branch (`git checkout -b feature/AmazingFeature`)
3. Committe deine Änderungen (`git commit -m 'Add some AmazingFeature'`)
4. Push zum Branch (`git push origin feature/AmazingFeature`)
5. Öffne eine Pull Request

## 📝 Lizenz

Dieses Projekt steht unter der MIT-Lizenz. Siehe `LICENSE` Datei für Details.

## 🆘 Support

- 📧 Email: kontakt@keystone-gateway.dev
- 🐛 Issues: [GitHub Issues](https://github.com/ygalsk/keystone-gateway/issues)
- 📖 Dokumentation: [Wiki](https://github.com/ygalsk/keystone-gateway/wiki)

---

**Made with ❤️ for the DevOps Community**
