#!/bin/bash

# Test blocking functionality
# Make sure example.com is in blocked_domains.txt

echo "=== Blocking Tests ==="
echo ""

# Test 1: Request to blocked domain
echo "Test 1: Request to blocked domain (should return 403)"
curl -x localhost:8888 -v http://example.com/ 2>&1 | grep -E "(HTTP|403|Forbidden)"
echo ""

# Test 2: Request to allowed domain
echo "Test 2: Request to allowed domain (should succeed)"
curl -x localhost:8888 -v http://httpbin.org/get 2>&1 | grep -E "(HTTP|200)" | head -3
echo ""

# Test 3: Check logs for blocked entries
echo "Test 3: Checking logs for BLOCKED entries..."
if [ -f proxy.log ]; then
    grep "BLOCKED" proxy.log | tail -3
else
    echo "Log file not found"
fi
echo ""

echo "=== Blocking Tests Complete ==="

