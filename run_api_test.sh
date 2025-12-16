#!/bin/bash
set -e

echo "=== 0. Forcing Rebuild ==="
rm -f abcfed
echo "Removed old binary to ensure rebuild."

echo "=== 1. Cleaning Environment ==="
# Automatically answer 'y' to clean_all.sh prompt
echo "y" | ./clean_all.sh

echo "=== 2. Starting 10 PoA Nodes ==="
# This script starts nodes in background and exits after checking status
# It will rebuild abcfed because we deleted it
./start_poa.sh 10

echo "=== 3. Waiting for network stability (20s) ==="
# Increased wait time to allow for block production (15s interval)
sleep 20

echo "=== 4. Running API Verification Tests ==="
# This Go program creates a wallet, funds it via /tx/send, then spends via /tx/signed
go run cmd/api_test/main.go

echo ""
echo "=== Test Complete ==="
echo "To stop nodes later: ./stop_all_nodes.sh"
