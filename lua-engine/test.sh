#!/bin/bash

# Test script for Keystone Lua Engine

set -e

# Colors for output
CYAN='\033[36m'
GREEN='\033[32m'
YELLOW='\033[33m'
RED='\033[31m'
RESET='\033[0m'

LUA_ENGINE_URL="http://localhost:8081"

echo -e "${CYAN}🧪 Keystone Lua Engine Test Suite${RESET}"
echo -e "${CYAN}===================================${RESET}"

# Check if lua engine is running
echo -e "\n${CYAN}1. Health Check${RESET}"
if curl -s "${LUA_ENGINE_URL}/health" > /dev/null; then
    echo -e "${GREEN}✅ Lua engine is running${RESET}"
    curl -s "${LUA_ENGINE_URL}/health" | jq .
else
    echo -e "${RED}❌ Lua engine is not running. Start it with: make run${RESET}"
    exit 1
fi

# Test canary deployment
echo -e "\n${CYAN}2. Testing Canary Deployment${RESET}"
curl -s -X POST "${LUA_ENGINE_URL}/route/canary" \
  -H "Content-Type: application/json" \
  -d '{
    "method": "GET",
    "path": "/api/users",
    "host": "api.example.com",
    "headers": {
      "X-Canary": "false",
      "X-Canary-Percent": "30"
    },
    "backends": [
      {
        "name": "api-stable",
        "url": "http://backend1:8080",
        "health": true
      },
      {
        "name": "api-canary",
        "url": "http://backend2:8080",
        "health": true
      }
    ]
  }' | jq .

# Test forced canary
echo -e "\n${CYAN}3. Testing Forced Canary${RESET}"
curl -s -X POST "${LUA_ENGINE_URL}/route/canary" \
  -H "Content-Type: application/json" \
  -d '{
    "method": "GET",
    "path": "/api/users",
    "host": "api.example.com",
    "headers": {
      "X-Canary": "true"
    },
    "backends": [
      {
        "name": "api-stable",
        "url": "http://backend1:8080",
        "health": true
      },
      {
        "name": "api-canary",
        "url": "http://backend2:8080",
        "health": true
      }
    ]
  }' | jq .

# Test blue-green deployment
echo -e "\n${CYAN}4. Testing Blue/Green Deployment${RESET}"
curl -s -X POST "${LUA_ENGINE_URL}/route/blue-green" \
  -H "Content-Type: application/json" \
  -d '{
    "method": "GET",
    "path": "/api/data",
    "host": "app.example.com",
    "headers": {
      "X-Deployment-State": "green"
    },
    "backends": [
      {
        "name": "app-blue",
        "url": "http://backend-blue:8080",
        "health": true
      },
      {
        "name": "app-green",
        "url": "http://backend-green:8080",
        "health": true
      }
    ]
  }' | jq .

# Test A/B testing
echo -e "\n${CYAN}5. Testing A/B Testing${RESET}"
curl -s -X POST "${LUA_ENGINE_URL}/route/ab-testing" \
  -H "Content-Type: application/json" \
  -d '{
    "method": "GET",
    "path": "/feature",
    "host": "test.example.com",
    "headers": {
      "X-User-ID": "user123"
    },
    "backends": [
      {
        "name": "service-version-a",
        "url": "http://backend-a:8080",
        "health": true
      },
      {
        "name": "service-version-b",
        "url": "http://backend-b:8080",
        "health": true
      }
    ]
  }' | jq .

# Test script reload
echo -e "\n${CYAN}6. Testing Script Reload${RESET}"
curl -s -X POST "${LUA_ENGINE_URL}/reload" | jq .

# Test non-existent tenant
echo -e "\n${CYAN}7. Testing Non-existent Tenant${RESET}"
curl -s -X POST "${LUA_ENGINE_URL}/route/nonexistent" \
  -H "Content-Type: application/json" \
  -d '{
    "method": "GET",
    "path": "/test",
    "host": "test.example.com",
    "headers": {},
    "backends": [
      {
        "name": "backend1",
        "url": "http://backend:8080",
        "health": true
      }
    ]
  }' | jq .

echo -e "\n${GREEN}✨ Test suite completed!${RESET}"
