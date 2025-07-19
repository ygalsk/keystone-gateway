const express = require('express');
const app = express();
const PORT = process.env.PORT || 3000;

// Middleware
app.use(express.json());

// Health check endpoint
app.get('/health', (req, res) => {
  res.status(200).send('healthy');
});

// Auth endpoints
app.post('/auth/login', (req, res) => {
  const { username, password } = req.body;
  
  if (username === 'demo' && password === 'password') {
    res.json({
      success: true,
      token: 'mock-jwt-token-' + Date.now(),
      user: { id: 1, username: 'demo', email: 'demo@example.com' }
    });
  } else {
    res.status(401).json({
      success: false,
      error: 'Invalid credentials'
    });
  }
});

app.post('/auth/logout', (req, res) => {
  res.json({
    success: true,
    message: 'Logged out successfully'
  });
});

app.get('/auth/verify', (req, res) => {
  const authHeader = req.headers.authorization;
  
  if (authHeader && authHeader.startsWith('Bearer mock-jwt-token')) {
    res.json({
      valid: true,
      user: { id: 1, username: 'demo', email: 'demo@example.com' }
    });
  } else {
    res.status(401).json({
      valid: false,
      error: 'Invalid or missing token'
    });
  }
});

app.get('/auth/status', (req, res) => {
  res.json({
    status: 'ok',
    service: 'auth-backend',
    timestamp: new Date().toISOString(),
    uptime: process.uptime()
  });
});

// Root endpoint
app.get('/', (req, res) => {
  res.json({
    message: 'Auth Backend Service',
    status: 'running',
    endpoints: ['/health', '/auth/login', '/auth/logout', '/auth/verify', '/auth/status']
  });
});

app.listen(PORT, '0.0.0.0', () => {
  console.log(`Auth Backend running on port ${PORT}`);
});