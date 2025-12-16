#!/usr/bin/env python3
"""
Simple mock backend servers for testing the gateway.
Runs three HTTP servers on ports 9001, 9002, 9003.
"""
import http.server
import socketserver
import threading
import json
from datetime import datetime

def create_handler(service_name, port):
    """Create a request handler for a specific service."""
    class Handler(http.server.BaseHTTPRequestHandler):
        def do_GET(self):
            response = {
                "service": service_name,
                "port": port,
                "path": self.path,
                "method": "GET",
                "timestamp": datetime.utcnow().isoformat()
            }
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(json.dumps(response, indent=2).encode())

        def do_POST(self):
            content_length = int(self.headers.get('Content-Length', 0))
            body = self.rfile.read(content_length).decode() if content_length > 0 else ""

            response = {
                "service": service_name,
                "port": port,
                "path": self.path,
                "method": "POST",
                "body": body,
                "timestamp": datetime.utcnow().isoformat()
            }
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(json.dumps(response, indent=2).encode())

        def log_message(self, format, *args):
            # Custom log format
            print(f"[{service_name}:{port}] {format % args}")

    return Handler

def start_server(name, port):
    """Start a single HTTP server."""
    handler = create_handler(name, port)
    with socketserver.TCPServer(("", port), handler) as httpd:
        print(f"✓ {name} listening on http://localhost:{port}")
        httpd.serve_forever()

if __name__ == "__main__":
    print("Starting mock backend servers...")

    servers = [
        ("API Backend", 9001),
        ("Admin Backend", 9002),
        ("Default Backend", 9003),
    ]

    threads = []
    for name, port in servers:
        thread = threading.Thread(target=start_server, args=(name, port), daemon=True)
        thread.start()
        threads.append(thread)

    print("\n" + "="*60)
    print("All backends running! Press Ctrl+C to stop.")
    print("="*60 + "\n")

    # Keep main thread alive
    try:
        for thread in threads:
            thread.join()
    except KeyboardInterrupt:
        print("\n\nShutting down backends...")
