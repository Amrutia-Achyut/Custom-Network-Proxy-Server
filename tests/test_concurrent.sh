#!/bin/bash

# Concurrent connection tests

echo "=== Concurrent Connection Tests ==="
echo ""

# Test: Multiple simultaneous requests
echo "Test: Sending 10 concurrent requests..."
for i in {1..10}; do
    curl -x localhost:8888 -s -o /dev/null -w "Request $i: %{http_code}\n" http://httpbin.org/get &
done

wait
echo ""

# Test: Check logs for concurrent entries
echo "Checking logs for concurrent requests..."
if [ -f proxy.log ]; then
    echo "Last 10 log entries:"
    tail -10 proxy.log
else
    echo "Log file not found"
fi
echo ""

echo "=== Concurrent Tests Complete ==="

