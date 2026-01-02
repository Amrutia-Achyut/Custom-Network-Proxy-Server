#!/bin/bash

# Basic proxy server tests
# Make sure the proxy server is running on localhost:8888

echo "=== Basic Proxy Server Tests ==="
echo ""

# Test 1: Simple GET request
echo "Test 1: Simple GET request"
curl -x localhost:8888 -v http://httpbin.org/get 2>&1 | grep -E "(HTTP|Host|GET)" | head -5
echo ""

# Test 2: HEAD request
echo "Test 2: HEAD request"
curl -x localhost:8888 -I http://httpbin.org/get 2>&1 | grep -E "(HTTP|Content-Type|Content-Length)" | head -5
echo ""

# Test 3: POST request
echo "Test 3: POST request"
curl -x localhost:8888 -X POST -d "test=data" http://httpbin.org/post 2>&1 | grep -E "(HTTP|test)" | head -5
echo ""

# Test 4: Check logs
echo "Test 4: Checking proxy.log for entries..."
if [ -f proxy.log ]; then
    tail -5 proxy.log
else
    echo "Log file not found"
fi
echo ""

echo "=== Tests Complete ==="

