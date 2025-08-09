# Keystone Logging

Simple structured JSON logging. One approach, all observability needs.

## Log Format

```json
{"time":"2025-08-09T01:22:39Z","level":"INFO","msg":"tenant_initialized","tenant":"api","backend_count":1,"component":"gateway"}
```

## Quick Queries

```bash
# Errors
jq 'select(.level=="ERROR")' keystone.log

# Health checks
jq 'select(.msg | test("health_check_"))' keystone.log

# By component
jq 'select(.component=="gateway")' keystone.log

# By tenant
jq 'select(.tenant=="api")' keystone.log
```

## Metrics from Logs

```bash
# Request rate
jq -r '.tenant' keystone.log | sort | uniq -c

# Error rate
jq -r 'select(.level=="ERROR") | .time' keystone.log | wc -l

# Backend health
jq -r 'select(.msg=="backend_healthy") | .backend' keystone.log
```

## Simple Monitoring

```bash
# Basic alert script
#!/bin/bash
ERRORS=$(tail -100 keystone.log | jq -r 'select(.level=="ERROR")')
[ -n "$ERRORS" ] && echo "Errors detected" | mail ops@company.com
```

## ELK/Grafana

Works with any log aggregator. Send JSON logs, query by fields. Done.

## Key Events

- `server_starting` - Gateway starts
- `tenant_initialized` - Tenant loaded
- `backend_healthy/unhealthy` - Health status
- `health_check_passed/failed` - Individual checks
- `circuit_breaker_state_change` - Circuit breaker events
- `proxy_error` - Backend failures

Simple. Effective. Complete observability.
