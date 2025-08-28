#!/usr/bin/env node

const http = require('http');

const server = http.createServer((req, res) => {
  console.log(`${new Date().toISOString()} - ${req.method} ${req.url}`);
  console.log('Headers:', JSON.stringify(req.headers, null, 2));

  res.setHeader('Content-Type', 'application/json');

  if (req.url === '/health') {
    res.statusCode = 200;
    res.end(JSON.stringify({ status: 'ok', timestamp: new Date().toISOString() }));
    return;
  }

  if (req.url.startsWith('/api/')) {
    // Check if Authorization header is present
    const authHeader = req.headers['authorization'];
    if (!authHeader || !authHeader.startsWith('Bearer ')) {
      res.statusCode = 401;
      res.end(JSON.stringify({ error: 'Missing or invalid Authorization header' }));
      return;
    }

    const token = authHeader.substring('Bearer '.length);
    res.statusCode = 200;
    res.end(JSON.stringify({
      message: 'OAuth middleware test successful!',
      token_received: token.substring(0, 20) + '...', // Show partial token for verification
      path: req.url,
      method: req.method
    }));
    return;
  }

  res.statusCode = 404;
  res.end(JSON.stringify({ error: 'Not Found' }));
});

const PORT = 3001;
server.listen(PORT, '127.0.0.1', () => {
  console.log(`Test backend server running on http://127.0.0.1:${PORT}`);
});