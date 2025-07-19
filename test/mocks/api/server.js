const express = require('express');
const app = express();
const PORT = process.env.PORT || 3000;

// Middleware
app.use(express.json());

// Health check endpoint
app.get('/health', (req, res) => {
  res.status(200).send('healthy');
});

// API endpoints
app.get('/api/users', (req, res) => {
  res.json([
    { id: 1, name: 'John Doe', email: 'john@example.com' },
    { id: 2, name: 'Jane Smith', email: 'jane@example.com' }
  ]);
});

app.get('/api/status', (req, res) => {
  res.json({
    status: 'ok',
    service: 'api-backend',
    timestamp: new Date().toISOString(),
    uptime: process.uptime()
  });
});

app.get('/api/version', (req, res) => {
  res.json({
    version: '1.0.0',
    service: 'api-backend',
    node: process.version
  });
});

// Catch all for API routes
app.get('/api/*', (req, res) => {
  res.status(404).json({
    error: 'API endpoint not found',
    path: req.path,
    method: req.method
  });
});

// Root endpoint
app.get('/', (req, res) => {
  res.json({
    message: 'API Backend Service',
    status: 'running',
    endpoints: ['/health', '/api/users', '/api/status', '/api/version']
  });
});

app.listen(PORT, '0.0.0.0', () => {
  console.log(`API Backend running on port ${PORT}`);
});