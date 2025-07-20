-- Keystone Gateway Database Initialization
-- This script sets up basic tables for development and testing

-- Create database if not exists (for development)
-- Note: This will be executed in the context of the POSTGRES_DB

-- =============================================================================
-- GATEWAY CONFIGURATION TABLES
-- =============================================================================

-- Table for storing dynamic tenant configurations
CREATE TABLE IF NOT EXISTS tenants (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    domains TEXT[] NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    active BOOLEAN DEFAULT true,
    config JSONB
);

-- Table for backend services
CREATE TABLE IF NOT EXISTS backend_services (
    id SERIAL PRIMARY KEY,
    tenant_id INTEGER REFERENCES tenants(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    url VARCHAR(500) NOT NULL,
    health_endpoint VARCHAR(200),
    weight INTEGER DEFAULT 100,
    circuit_breaker_enabled BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    active BOOLEAN DEFAULT true
);

-- =============================================================================
-- MONITORING AND METRICS TABLES
-- =============================================================================

-- Table for storing request metrics
CREATE TABLE IF NOT EXISTS request_metrics (
    id SERIAL PRIMARY KEY,
    tenant_id INTEGER REFERENCES tenants(id),
    backend_service_id INTEGER REFERENCES backend_services(id),
    method VARCHAR(10) NOT NULL,
    path VARCHAR(500) NOT NULL,
    status_code INTEGER NOT NULL,
    response_time_ms INTEGER NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    user_agent TEXT,
    remote_ip INET,
    request_size INTEGER,
    response_size INTEGER
);

-- Table for health check results
CREATE TABLE IF NOT EXISTS health_checks (
    id SERIAL PRIMARY KEY,
    backend_service_id INTEGER REFERENCES backend_services(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL, -- 'healthy', 'unhealthy', 'unknown'
    response_time_ms INTEGER,
    error_message TEXT,
    checked_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- =============================================================================
-- RATE LIMITING TABLES
-- =============================================================================

-- Table for rate limiting rules
CREATE TABLE IF NOT EXISTS rate_limit_rules (
    id SERIAL PRIMARY KEY,
    tenant_id INTEGER REFERENCES tenants(id) ON DELETE CASCADE,
    path_pattern VARCHAR(500) NOT NULL,
    requests_per_second INTEGER NOT NULL,
    burst_limit INTEGER NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    active BOOLEAN DEFAULT true
);

-- =============================================================================
-- LUA SCRIPTS MANAGEMENT
-- =============================================================================

-- Table for storing and versioning Lua scripts
CREATE TABLE IF NOT EXISTS lua_scripts (
    id SERIAL PRIMARY KEY,
    tenant_id INTEGER REFERENCES tenants(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(50) NOT NULL,
    script_content TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    active BOOLEAN DEFAULT false,
    UNIQUE(tenant_id, name, version)
);

-- =============================================================================
-- AUDIT AND LOGGING TABLES
-- =============================================================================

-- Table for configuration changes audit
CREATE TABLE IF NOT EXISTS config_audit (
    id SERIAL PRIMARY KEY,
    table_name VARCHAR(100) NOT NULL,
    record_id INTEGER NOT NULL,
    action VARCHAR(20) NOT NULL, -- 'INSERT', 'UPDATE', 'DELETE'
    old_values JSONB,
    new_values JSONB,
    changed_by VARCHAR(255),
    changed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- =============================================================================
-- INDEXES FOR PERFORMANCE
-- =============================================================================

-- Indexes for request_metrics (for analytics queries)
CREATE INDEX IF NOT EXISTS idx_request_metrics_timestamp ON request_metrics(timestamp);
CREATE INDEX IF NOT EXISTS idx_request_metrics_tenant ON request_metrics(tenant_id);
CREATE INDEX IF NOT EXISTS idx_request_metrics_status ON request_metrics(status_code);
CREATE INDEX IF NOT EXISTS idx_request_metrics_path ON request_metrics(path);

-- Indexes for health_checks
CREATE INDEX IF NOT EXISTS idx_health_checks_service ON health_checks(backend_service_id);
CREATE INDEX IF NOT EXISTS idx_health_checks_timestamp ON health_checks(checked_at);
CREATE INDEX IF NOT EXISTS idx_health_checks_status ON health_checks(status);

-- Indexes for tenants
CREATE INDEX IF NOT EXISTS idx_tenants_domains ON tenants USING GIN(domains);
CREATE INDEX IF NOT EXISTS idx_tenants_active ON tenants(active);

-- =============================================================================
-- SAMPLE DATA FOR DEVELOPMENT
-- =============================================================================

-- Insert sample tenant for development
INSERT INTO tenants (name, domains, config) VALUES 
('development', ARRAY['localhost', '127.0.0.1', 'dev.localhost'], '{
    "features": {
        "lua_routing": true,
        "metrics": true,
        "rate_limiting": false
    },
    "settings": {
        "timeout": "30s",
        "retries": 3
    }
}'::jsonb)
ON CONFLICT (name) DO NOTHING;

-- Insert sample backend services for development
INSERT INTO backend_services (tenant_id, name, url, health_endpoint, weight) VALUES 
((SELECT id FROM tenants WHERE name = 'development'), 'mock-api', 'http://mock-backend-1:80', '/health', 100),
((SELECT id FROM tenants WHERE name = 'development'), 'mock-static', 'http://mock-backend-2:80', '/health', 100),
((SELECT id FROM tenants WHERE name = 'development'), 'mock-node', 'http://mock-backend-3:8080', '/health', 100)
ON CONFLICT DO NOTHING;

-- Insert sample rate limiting rules
INSERT INTO rate_limit_rules (tenant_id, path_pattern, requests_per_second, burst_limit) VALUES
((SELECT id FROM tenants WHERE name = 'development'), '/api/*', 100, 200),
((SELECT id FROM tenants WHERE name = 'development'), '/admin/*', 10, 20)
ON CONFLICT DO NOTHING;

-- =============================================================================
-- FUNCTIONS AND TRIGGERS
-- =============================================================================

-- Function to update the updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Triggers for automatic timestamp updates
CREATE TRIGGER update_tenants_updated_at 
    BEFORE UPDATE ON tenants 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_backend_services_updated_at 
    BEFORE UPDATE ON backend_services 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Function for audit logging
CREATE OR REPLACE FUNCTION audit_trigger_function()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        INSERT INTO config_audit (table_name, record_id, action, old_values, changed_by)
        VALUES (TG_TABLE_NAME, OLD.id, TG_OP, row_to_json(OLD), current_user);
        RETURN OLD;
    ELSIF TG_OP = 'UPDATE' THEN
        INSERT INTO config_audit (table_name, record_id, action, old_values, new_values, changed_by)
        VALUES (TG_TABLE_NAME, NEW.id, TG_OP, row_to_json(OLD), row_to_json(NEW), current_user);
        RETURN NEW;
    ELSIF TG_OP = 'INSERT' THEN
        INSERT INTO config_audit (table_name, record_id, action, new_values, changed_by)
        VALUES (TG_TABLE_NAME, NEW.id, TG_OP, row_to_json(NEW), current_user);
        RETURN NEW;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- Create audit triggers for important tables
CREATE TRIGGER tenants_audit_trigger
    AFTER INSERT OR UPDATE OR DELETE ON tenants
    FOR EACH ROW EXECUTE FUNCTION audit_trigger_function();

CREATE TRIGGER backend_services_audit_trigger
    AFTER INSERT OR UPDATE OR DELETE ON backend_services
    FOR EACH ROW EXECUTE FUNCTION audit_trigger_function();

-- =============================================================================
-- CLEANUP PROCEDURES
-- =============================================================================

-- Function to clean up old metrics (call periodically)
CREATE OR REPLACE FUNCTION cleanup_old_metrics(days_to_keep INTEGER DEFAULT 30)
RETURNS INTEGER AS $$
DECLARE
    deleted_count INTEGER;
BEGIN
    DELETE FROM request_metrics 
    WHERE timestamp < CURRENT_TIMESTAMP - INTERVAL '1 day' * days_to_keep;
    
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    
    DELETE FROM health_checks 
    WHERE checked_at < CURRENT_TIMESTAMP - INTERVAL '1 day' * days_to_keep;
    
    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- Grant permissions for the development user
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO dev;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO dev;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO dev;