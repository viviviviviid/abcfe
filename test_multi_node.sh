#!/bin/bash

# 기존 프로세스 정리
pkill -f abcfed 2>/dev/null || true
sleep 1

# DB 정리 (새로 시작하기 위해)
rm -rf ./resource/db/*
rm -rf ./resource/db2/*

echo "=== Starting Node 1 (Port 30303, REST 8000) ==="
./abcfed &
NODE1_PID=$!
echo "Node 1 PID: $NODE1_PID"

sleep 3

echo ""
echo "=== Starting Node 2 (Port 30304, REST 8001, connects to Node 1) ==="
./abcfed --config=./config/config_node2.toml &
NODE2_PID=$!
echo "Node 2 PID: $NODE2_PID"

echo ""
echo "=== Waiting for blocks to be created... ==="
sleep 15

echo ""
echo "=== Checking Node 1 status ==="
curl -s http://localhost:8000/api/v1/status 2>/dev/null || echo "Node 1 API not responding"

echo ""
echo "=== Checking Node 2 status ==="
curl -s http://localhost:8001/api/v1/status 2>/dev/null || echo "Node 2 API not responding"

echo ""
echo "=== Stopping nodes ==="
kill $NODE1_PID 2>/dev/null
kill $NODE2_PID 2>/dev/null

echo "Done!"
