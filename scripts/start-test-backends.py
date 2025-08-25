#!/usr/bin/env python3
"""
Simple test backend servers for Keystone Gateway testing.
Starts 3 HTTP servers on ports 3001, 3002, and 3003.
"""

import json
import http.server
import socketserver
import threading
import time
from urllib.parse import urlparse

class TestBackendHandler(http.server.BaseHTTPRequestHandler):
    def __init__(self, server_name, *args, **kwargs):
        self.server_name = server_name
        super().__init__(*args, **kwargs)

    def do_GET(self):
        if self.path == '/health':
            self.send_health_response()
        elif self.path.startswith('/api/'):
            self.send_api_response()
        else:
            self.send_default_response()

    def do_POST(self):
        self.send_api_response()

    def do_PUT(self):
        self.send_api_response()

    def do_DELETE(self):
        self.send_response(204)
        self.end_headers()

    def send_health_response(self):
        response = {
            "status": "healthy",
            "server": self.server_name,
            "timestamp": time.time(),
            "port": self.server.server_address[1]
        }
        self.send_json_response(200, response)

    def send_api_response(self):
        response = {
            "message": f"Response from {self.server_name}",
            "method": self.command,
            "path": self.path,
            "server": self.server_name,
            "port": self.server.server_address[1],
            "timestamp": time.time()
        }
        self.send_json_response(200, response)

    def send_default_response(self):
        response = {
            "server": self.server_name,
            "message": "Test backend server",
            "path": self.path,
            "available_endpoints": ["/health", "/api/*"],
            "port": self.server.server_address[1]
        }
        self.send_json_response(200, response)

    def send_json_response(self, status_code, data):
        self.send_response(status_code)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Access-Control-Allow-Origin', '*')
        self.send_header('Access-Control-Allow-Methods', 'GET, POST, PUT, DELETE, OPTIONS')
        self.send_header('Access-Control-Allow-Headers', 'Content-Type, Authorization')
        self.end_headers()
        self.wfile.write(json.dumps(data, indent=2).encode())

    def log_message(self, format, *args):
        print(f"[{self.server_name}] {format % args}")

def create_server(port, server_name):
    """Create a server with a custom handler that includes the server name."""
    class CustomHandler(TestBackendHandler):
        def __init__(self, *args, **kwargs):
            super().__init__(server_name, *args, **kwargs)
    
    return socketserver.TCPServer(('', port), CustomHandler)

def start_server(port, server_name):
    """Start a server in a separate thread."""
    try:
        with create_server(port, server_name) as httpd:
            print(f"✅ {server_name} started on http://localhost:{port}")
            print(f"   Health check: http://localhost:{port}/health")
            httpd.serve_forever()
    except OSError as e:
        print(f"❌ Failed to start {server_name} on port {port}: {e}")

def main():
    print("🚀 Starting Keystone Gateway Test Backend Servers...")
    print("=" * 60)
    
    # Define servers according to config.lua-test.yaml
    servers = [
        (3001, "api-server-primary"),
        (3002, "api-server-secondary"), 
        (3003, "legacy-server")
    ]
    
    threads = []
    
    # Start each server in its own thread
    for port, name in servers:
        thread = threading.Thread(target=start_server, args=(port, name), daemon=True)
        thread.start()
        threads.append(thread)
        time.sleep(0.1)  # Small delay to avoid port conflicts
    
    print("=" * 60)
    print("🎯 All servers started! Test endpoints:")
    print()
    for port, name in servers:
        print(f"  {name}:")
        print(f"    Health: curl http://localhost:{port}/health")
        print(f"    API:    curl http://localhost:{port}/api/test")
        print()
    
    print("🔗 Gateway endpoints (when gateway is running):")
    print("  Default tenant:  curl http://localhost:8080/api/hello")
    print("  Advanced tenant: curl http://localhost:8080/api/v1/status")
    print("  API tenant:      curl http://localhost:8080/api/users")
    print()
    print("Press Ctrl+C to stop all servers...")
    
    try:
        # Keep main thread alive
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        print("\n👋 Shutting down all servers...")

if __name__ == "__main__":
    main()