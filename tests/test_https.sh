#!/bin/bash

# HTTPS CONNECT tunneling tests
# Requires enable_connect_tunneling=true in config

echo "=== HTTPS CONNECT Tunneling Tests ==="
echo ""

# Test 1: HTTPS request via CONNECT
echo "Test 1: HTTPS request (should use CONNECT tunneling)"
curl -x localhost:8888 -v https://httpbin.org/get 2>&1 | grep -E "(HTTP|CONNECT|200|SSL)" | head -10
echo ""

# Test 2: Check logs for CONNECT entries
echo "Test 2: Checking logs for CONNECT requests..."
if [ -f proxy.log ]; then
    grep "CONNECT" proxy.log | tail -3
else
    echo "Log file not found"
fi
echo ""

echo "=== HTTPS Tests Complete ==="

