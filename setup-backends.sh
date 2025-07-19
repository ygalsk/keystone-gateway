#!/bin/bash
# setup-backends.sh - Setup mock backend services for Keystone Gateway testing

set -e

echo "🚀 Setting up Keystone Gateway Mock Backends"
echo "============================================="

# Create directory structure
echo "📁 Creating directory structure..."
mkdir -p mock-backends/{demo,api,auth,status,grafana/provisioning,postgres}

# Demo Backend (Nginx)
echo "🌐 Setting up Demo Backend..."
cat > mock-backends/demo/index.html << 'EOF'
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Keystone Demo Application</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; background: #f5f5f5; }
        .container { max-width: 800px; margin: 0 auto; background: white; padding: 30px; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        h1 { color: #333; border-bottom: 3px solid #007acc; padding-bottom: 10px; }
        .status { background: #e8f5e8; padding: 15px; border-radius: 5px; border-left: 4px solid #4caf50; margin: 20px 0; }
        .info { background: #e3f2fd; padding: 15px; border-radius: 5px; border-left: 4px solid #2196f3; margin: 20px 0; }
        pre { background: #f8f8f8; padding: 15px; border-radius: 5px; overflow-x: auto; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🌟 Keystone Gateway Demo Application</h1>
        
        <div class="status">
            <strong>✅ Demo Service Online</strong><br>
            Service: demo.keystone-gateway.dev<br>
            Backend: Docker Container (Port 3001)<br>
            Status: Healthy
        </div>

        <div class="info">
            <strong>🔗 Available Endpoints:</strong><br>
            • <a href="/dashboard">Dashboard</a><br>
            • <a href="/users">User Management</a><br>
            • <a href="/reports">Reports</a><br>
            • <a href="/health">Health Check</a>
        </div>

        <h2>📊 Performance Testing</h2>
        <p>This demo backend is optimized for performance testing with:</p>
        <ul>
            <li>Static content delivery via Nginx</li>
            <li>Health check endpoint</li>
            <li>Simulated dashboard and user pages</li>
            <li>Low latency responses</li>
        </ul>

        <pre>Timestamp: <span id="timestamp"></span>
Backend: demo-backend:3001
Gateway: keystone-gateway:8080</pre>
    </div>

    <script>
        document.getElementById('timestamp').textContent = new Date().toISOString();
        setInterval(() => {
            document.getElementById('timestamp').textContent = new Date().toISOString();
        }, 1000);
    </script>
</body>
</html>
EOF

cat > mock-backends/demo/health << 'EOF'
OK
EOF

cat > mock-backends/demo/dashboard << 'EOF'
<!DOCTYPE html>
<html><head><title>Demo Dashboard</title></head>
<body><h1>Demo Dashboard</h1><p>Dashboard is running!</p></body></html>
EOF

cat > mock-backends/demo/nginx.conf << 'EOF'
server {
    listen 80;
    server_name localhost;
    root /usr/share/nginx/html;
    index index.html;

    # Enable gzip compression for better performance
    gzip on;
    gzip_types text/plain text/css application/json application/javascript text/xml application/xml;

    location / {
        try_files $uri $uri/ /index.html;
        add_header X-Backend-Service "demo-backend";
        add_header X-Backend-Version "1.0.0";
    }

    location /health {
        add_header Content-Type text/plain;
        return 200 "OK";
    }

    location /dashboard {
        try_files /dashboard /dashboard.html =404;
        add_header X-Backend-Service "demo-backend";
    }

    location /users {
        add_header Content-Type application/json;
        return 200 '[{"id":1,"name":"Demo User","email":"demo@keystone-gateway.dev"}]';
    }

    location /reports {
        add_header Content-Type application/json;
        return 200 '{"reports":["daily","weekly","monthly"],"status":"available"}';
    }
}
EOF

# API Backend (Node.js)
echo "🔌 Setting up API Backend..."
cat > mock-backends/api/package.json << 'EOF'
{
  "name": "keystone-api-backend",
  "version": "1.0.0",
  "description": "Mock API backend for Keystone Gateway testing",
  "main": "server.js",
  "dependencies": {
    "express": "^4.18.2",
    "cors": "^2.8.5"
  },
  "scripts": {
    "start": "node server.js"
  }
}
EOF

cat > mock-backends/api/server.js << 'EOF'
const express = require('express');
const cors = require('cors');

const app = express();
const PORT = process.env.PORT || 3000;

// Middleware
app.use(cors());
app.use(express.json());

// Add response headers for identification
app.use((req, res, next) => {
    res.set({
        'X-Backend-Service': 'api-backend',
        'X-Backend-Version': '1.0.0',
        'X-Backend-Port': PORT
    });
    next();
});

// Health check
app.get('/health', (req, res) => {
    res.status(200).send('OK');
});

// API v1 routes
app.get('/v1/health', (req, res) => {
    res.json({ status: 'healthy', version: 'v1', timestamp: new Date().toISOString() });
});

app.get('/v1/users', (req, res) => {
    res.json({
        users: [
            { id: 1, name: 'John Doe', email: 'john@example.com' },
            { id: 2, name: 'Jane Smith', email: 'jane@example.com' }
        ],
        version: 'v1'
    });
});

// API v2 routes  
app.get('/v2/health', (req, res) => {
    res.json({ status: 'healthy', version: 'v2', timestamp: new Date().toISOString() });
});

app.get('/v2/users', (req, res) => {
    res.json({
        data: [
            { id: 1, name: 'John Doe', email: 'john@example.com', role: 'admin' },
            { id: 2, name: 'Jane Smith', email: 'jane@example.com', role: 'user' }
        ],
        meta: { version: 'v2', count: 2 }
    });
});

// Admin routes
app.get('/admin/health', (req, res) => {
    res.json({ status: 'healthy', service: 'admin', timestamp: new Date().toISOString() });
});

app.get('/admin/stats', (req, res) => {
    res.json({
        requests: Math.floor(Math.random() * 10000),
        uptime: process.uptime(),
        memory: process.memoryUsage(),
        timestamp: new Date().toISOString()
    });
});

// Default routes
app.get('/', (req, res) => {
    res.json({
        service: 'Keystone API Backend',
        version: '1.0.0',
        endpoints: ['/health', '/v1/users', '/v2/users', '/admin/stats'],
        timestamp: new Date().toISOString()
    });
});

app.get('/users', (req, res) => {
    res.json({
        users: [
            { id: 1, name: 'Demo User 1', active: true },
            { id: 2, name: 'Demo User 2', active: true },
            { id: 3, name: 'Demo User 3', active: false }
        ],
        total: 3,
        timestamp: new Date().toISOString()
    });
});

// Start server
app.listen(PORT, '0.0.0.0', () => {
    console.log(`🔌 API Backend listening on port ${PORT}`);
    console.log(`🌐 Health check: http://localhost:${PORT}/health`);
});
EOF

# Auth Backend (Node.js)
echo "🔐 Setting up Auth Backend..."
cat > mock-backends/auth/package.json << 'EOF'
{
  "name": "keystone-auth-backend",
  "version": "1.0.0",
  "description": "Mock auth backend for Keystone Gateway testing",
  "main": "server.js",
  "dependencies": {
    "express": "^4.18.2",
    "cors": "^2.8.5",
    "jsonwebtoken": "^9.0.0"
  },
  "scripts": {
    "start": "node server.js"
  }
}
EOF

cat > mock-backends/auth/server.js << 'EOF'
const express = require('express');
const cors = require('cors');
const jwt = require('jsonwebtoken');

const app = express();
const PORT = process.env.PORT || 3000;
const JWT_SECRET = 'keystone-test-secret';

// Middleware
app.use(cors());
app.use(express.json());
app.use(express.urlencoded({ extended: true }));

// Add response headers
app.use((req, res, next) => {
    res.set({
        'X-Backend-Service': 'auth-backend',
        'X-Backend-Version': '1.0.0',
        'X-Backend-Port': PORT
    });
    next();
});

// Health check
app.get('/health', (req, res) => {
    res.status(200).send('OK');
});

// Login page
app.get('/login', (req, res) => {
    res.send(`
        <!DOCTYPE html>
        <html>
        <head>
            <title>Keystone Gateway - Login</title>
            <style>
                body { font-family: Arial, sans-serif; margin: 40px; background: #f5f5f5; }
                .container { max-width: 400px; margin: 0 auto; background: white; padding: 30px; border-radius: 8px; }
                input, button { width: 100%; padding: 10px; margin: 10px 0; border: 1px solid #ddd; border-radius: 4px; }
                button { background: #007acc; color: white; border: none; cursor: pointer; }
                button:hover { background: #005a9e; }
            </style>
        </head>
        <body>
            <div class="container">
                <h2>🔐 Keystone Gateway Login</h2>
                <form action="/authenticate" method="post">
                    <input type="text" name="username" placeholder="Username" value="demo" required>
                    <input type="password" name="password" placeholder="Password" value="password" required>
                    <button type="submit">Login</button>
                </form>
                <p><small>Demo credentials: demo/password</small></p>
            </div>
        </body>
        </html>
    `);
});

// Authentication endpoint
app.post('/authenticate', (req, res) => {
    const { username, password } = req.body;
    
    // Demo authentication
    if (username === 'demo' && password === 'password') {
        const token = jwt.sign(
            { username, role: 'user', iat: Date.now() },
            JWT_SECRET,
            { expiresIn: '1h' }
        );
        
        res.json({
            success: true,
            token,
            user: { username, role: 'user' },
            message: 'Authentication successful'
        });
    } else {
        res.status(401).json({
            success: false,
            message: 'Invalid credentials'
        });
    }
});

// Token verification
app.post('/verify', (req, res) => {
    const token = req.headers.authorization?.replace('Bearer ', '');
    
    if (!token) {
        return res.status(401).json({ valid: false, message: 'No token provided' });
    }
    
    try {
        const decoded = jwt.verify(token, JWT_SECRET);
        res.json({ valid: true, user: decoded });
    } catch (error) {
        res.status(401).json({ valid: false, message: 'Invalid token' });
    }
});

// User profile
app.get('/profile', (req, res) => {
    res.json({
        service: 'Authentication Service',
        endpoints: ['/login', '/authenticate', '/verify', '/profile'],
        timestamp: new Date().toISOString()
    });
});

// Default route
app.get('/', (req, res) => {
    res.json({
        service: 'Keystone Auth Backend',
        version: '1.0.0',
        status: 'running',
        timestamp: new Date().toISOString()
    });
});

app.listen(PORT, '0.0.0.0', () => {
    console.log(`🔐 Auth Backend listening on port ${PORT}`);
    console.log(`🌐 Health check: http://localhost:${PORT}/health`);
});
EOF

# Status Backend (Nginx)
echo "📊 Setting up Status Backend..."
cat > mock-backends/status/index.html << 'EOF'
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Keystone Gateway Status</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; background: #f8f9fa; }
        .container { max-width: 1200px; margin: 0 auto; }
        .status-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; margin: 20px 0; }
        .status-card { background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .status-healthy { border-left: 4px solid #28a745; }
        .status-warning { border-left: 4px solid #ffc107; }
        .metric { display: flex; justify-content: space-between; margin: 10px 0; }
        .metric-value { font-weight: bold; color: #007acc; }
        h1 { color: #333; text-align: center; }
        .timestamp { text-align: center; color: #666; font-size: 0.9em; }
    </style>
</head>
<body>
    <div class="container">
        <h1>📊 Keystone Gateway Status Dashboard</h1>
        <p class="timestamp">Last updated: <span id="timestamp"></span></p>
        
        <div class="status-grid">
            <div class="status-card status-healthy">
                <h3>🌐 Demo Service</h3>
                <div class="metric"><span>Status:</span><span class="metric-value">Healthy</span></div>
                <div class="metric"><span>Uptime:</span><span class="metric-value">99.9%</span></div>
                <div class="metric"><span>Response Time:</span><span class="metric-value">12ms</span></div>
            </div>
            
            <div class="status-card status-healthy">
                <h3>🔌 API Service</h3>
                <div class="metric"><span>Status:</span><span class="metric-value">Healthy</span></div>
                <div class="metric"><span>Uptime:</span><span class="metric-value">99.8%</span></div>
                <div class="metric"><span>Response Time:</span><span class="metric-value">8ms</span></div>
            </div>
            
            <div class="status-card status-healthy">
                <h3>🔐 Auth Service</h3>
                <div class="metric"><span>Status:</span><span class="metric-value">Healthy</span></div>
                <div class="metric"><span>Uptime:</span><span class="metric-value">99.7%</span></div>
                <div class="metric"><span>Response Time:</span><span class="metric-value">15ms</span></div>
            </div>
            
            <div class="status-card status-healthy">
                <h3>📈 Grafana</h3>
                <div class="metric"><span>Status:</span><span class="metric-value">Healthy</span></div>
                <div class="metric"><span>Uptime:</span><span class="metric-value">99.6%</span></div>
                <div class="metric"><span>Response Time:</span><span class="metric-value">45ms</span></div>
            </div>
        </div>
        
        <div class="status-card">
            <h3>🚀 Gateway Performance</h3>
            <div class="metric"><span>Total Requests:</span><span class="metric-value" id="requests">0</span></div>
            <div class="metric"><span>Requests/sec:</span><span class="metric-value" id="rps">0</span></div>
            <div class="metric"><span>Average Latency:</span><span class="metric-value">8.5ms</span></div>
            <div class="metric"><span>Error Rate:</span><span class="metric-value">0.01%</span></div>
        </div>
    </div>

    <script>
        function updateTimestamp() {
            document.getElementById('timestamp').textContent = new Date().toISOString();
        }
        
        function updateMetrics() {
            const requests = Math.floor(Math.random() * 100000) + 50000;
            const rps = Math.floor(Math.random() * 500) + 100;
            document.getElementById('requests').textContent = requests.toLocaleString();
            document.getElementById('rps').textContent = rps;
        }
        
        updateTimestamp();
        updateMetrics();
        setInterval(updateTimestamp, 1000);
        setInterval(updateMetrics, 5000);
    </script>
</body>
</html>
EOF

cat > mock-backends/status/nginx.conf << 'EOF'
server {
    listen 80;
    server_name localhost;
    root /usr/share/nginx/html;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
        add_header X-Backend-Service "status-backend";
    }

    location /health {
        add_header Content-Type text/plain;
        return 200 "OK";
    }

    location /api/status {
        add_header Content-Type application/json;
        return 200 '{"status":"healthy","services":{"demo":"up","api":"up","auth":"up","grafana":"up"}}';
    }
}
EOF

# PostgreSQL init script
echo "🗄️ Setting up Database..."
cat > mock-backends/postgres/init.sql << 'EOF'
-- Initialize Keystone Gateway demo database
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
    id VARCHAR(255) PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL
);

-- Insert demo data
INSERT INTO users (username, email) VALUES 
    ('demo', 'demo@keystone-gateway.dev'),
    ('admin', 'admin@keystone-gateway.dev'),
    ('testuser', 'test@keystone-gateway.dev')
ON CONFLICT (username) DO NOTHING;
EOF

# Grafana provisioning (basic setup)
mkdir -p mock-backends/grafana/provisioning/{dashboards,datasources}

cat > mock-backends/grafana/provisioning/datasources/datasource.yml << 'EOF'
apiVersion: 1
datasources:
  - name: Prometheus
    type: prometheus
    url: http://localhost:9090
    access: proxy
    isDefault: true
EOF

# Install Node.js dependencies
echo "📦 Installing Node.js dependencies..."
cd mock-backends/api && npm install --silent
cd ../auth && npm install --silent
cd ../..

echo ""
echo "✅ Mock backend setup completed!"
echo ""
echo "📋 Created services:"
echo "   • Demo Backend (Nginx) - Port 3001"
echo "   • API Backend (Node.js) - Port 3002" 
echo "   • Auth Backend (Node.js) - Port 3003"
echo "   • Status Backend (Nginx) - Port 3004"
echo "   • Grafana Backend - Port 3005"
echo "   • Redis - Port 6379"
echo "   • PostgreSQL - Port 5432"
echo ""
echo "🚀 Next steps:"
echo "   1. Run: docker-compose up -d"
echo "   2. Run: ./build-and-test.sh"
echo "   3. Open: http://localhost:8080/admin/health"
