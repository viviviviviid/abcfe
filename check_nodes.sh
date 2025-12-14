#!/bin/bash

# 모든 노드 상태 확인 스크립트

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}=== ABCFe 노드 상태 확인 ===${NC}"
echo ""

# 실행 중인 노드 프로세스 확인
NODE_PIDS=$(pgrep -f abcfed)

if [ -z "$NODE_PIDS" ]; then
    echo -e "${RED}실행 중인 노드가 없습니다.${NC}"
    exit 0
fi

echo -e "${YELLOW}실행 중인 노드 프로세스:${NC}"
ps aux | grep abcfed | grep -v grep | while read line; do
    pid=$(echo "$line" | awk '{print $2}')
    cmd=$(echo "$line" | awk '{for(i=11;i<=NF;i++) printf "%s ", $i; print ""}')
    
    # 실제 실행 파일 경로 확인
    if [ -r "/proc/$pid/exe" ]; then
        exe_path=$(readlink -f /proc/$pid/exe 2>/dev/null || echo "N/A")
    else
        exe_path="N/A (권한 없음)"
    fi
    
    # 작업 디렉토리 확인
    if [ -r "/proc/$pid/cwd" ]; then
        cwd=$(readlink -f /proc/$pid/cwd 2>/dev/null || echo "N/A")
    else
        cwd="N/A"
    fi
    
    echo -e "  ${GREEN}PID: $pid${NC}"
    echo -e "    실행 파일: ${BLUE}$exe_path${NC}"
    echo -e "    작업 디렉토리: $cwd"
    echo -e "    명령어: $cmd"
    echo ""
done
echo ""

# 각 포트별로 노드 상태 확인 및 peers 정보 추가
for port in 8000 8001 8002 8003 8004 8005 8006 8007 8008 8009; do
    if curl -s http://localhost:${port}/api/v1/status > /dev/null 2>&1; then
        node_num=$((port - 7999))
        status=$(curl -s http://localhost:${port}/api/v1/status)
        height=$(echo "$status" | python3 -c "import sys, json; print(json.load(sys.stdin)['data']['currentHeight'])" 2>/dev/null || echo "N/A")
        hash=$(echo "$status" | python3 -c "import sys, json; print(json.load(sys.stdin)['data']['currentBlockHash'][:16])" 2>/dev/null || echo "N/A")
        mempool=$(echo "$status" | python3 -c "import sys, json; print(json.load(sys.stdin)['data']['mempoolSize'])" 2>/dev/null || echo "N/A")

        # peers 정보 (연결된 peer 수 및 peer 리스트)
        peer_status=$(curl -s http://localhost:${port}/api/v1/p2p/status 2>/dev/null)
        peer_count=$(echo "$peer_status" | python3 -c "import sys, json; print(json.load(sys.stdin).get('data', {}).get('peerCount', 'N/A'))" 2>/dev/null || echo "N/A")

        echo -e "${GREEN}Node ${node_num} (REST: ${port}):${NC}"
        echo "  Height: $height"
        echo "  Hash: ${hash}..."
        echo "  Mempool: $mempool transactions"
        echo "  Peers: $peer_count"

        # 상세 Peers 목록 출력
        # 아래 API 중 하나만 동작할 수 있음. 우선적으로 /api/v1/p2p/peers 시도, 없으면 /api/v1/p2p/connection
        # 일부 ABCFe 버전에서 /peers가 작동 안할 수도 있음
        if [ "$peer_count" != "N/A" ] && [ "$peer_count" != "0" ]; then
            peer_list=$(curl -s http://localhost:${port}/api/v1/p2p/peers 2>/dev/null)
            if [ -n "$peer_list" ]; then
                peer_ips=$(echo "$peer_list" | python3 -c "
import sys, json
try:
    peers = json.load(sys.stdin).get('data', [])
    if isinstance(peers, list) and peers:
        for p in peers:
            ip = p.get('IP') or p.get('ip') or p.get('addr') or 'unknown'
            port = p.get('Port') or p.get('port') or 'unknown'
            peerid = p.get('ID') or p.get('id') or ''
            display = f\"    - {ip}:{port}\"
            if peerid: display += f\" (ID: {peerid})\"
            print(display)
    elif isinstance(peers, dict): # 혹시 dict 타입이면 일괄 출력
        for k, v in peers.items():
            print(f\"    - {k}: {v}\")
except Exception as e:
    pass
" 2>/dev/null)
                if [ -n "$peer_ips" ]; then
                    echo "  Peer List:"
                    echo "$peer_ips"
                fi
            else
                # 대체 API 시도(/api/v1/p2p/connection)
                peer_conn=$(curl -s http://localhost:${port}/api/v1/p2p/connection 2>/dev/null)
                if [ -n "$peer_conn" ]; then
                    peer_ips=$(echo "$peer_conn" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin).get('data', [])
    if isinstance(data, list) and data:
        for p in data:
            ip = p.get('IP') or p.get('ip') or p.get('addr') or 'unknown'
            port = p.get('Port') or p.get('port') or 'unknown'
            peerid = p.get('ID') or p.get('id') or ''
            display = f\"    - {ip}:{port}\"
            if peerid: display += f\" (ID: {peerid})\"
            print(display)
    elif isinstance(data, dict):
        for k, v in data.items():
            print(f\"    - {k}: {v}\")
except Exception as e:
    pass
" 2>/dev/null)
                    if [ -n "$peer_ips" ]; then
                        echo "  Peer List (from connection):"
                        echo "$peer_ips"
                    else
                        echo "  Peer 목록 정보를 가져올 수 없음 (peers/connection)"
                    fi
                else
                    echo "  Peer 목록 정보를 가져올 수 없음 (peers/connection)"
                fi
            fi
        fi
        echo ""
    fi
done

# 동기화 상태 확인
echo -e "${YELLOW}=== 동기화 상태 ===${NC}"
heights=()
for port in 8000 8001 8002 8003 8004 8005 8006 8007 8008 8009; do
    if curl -s http://localhost:${port}/api/v1/status > /dev/null 2>&1; then
        height=$(curl -s http://localhost:${port}/api/v1/status | python3 -c "import sys, json; print(json.load(sys.stdin)['data']['currentHeight'])" 2>/dev/null || echo "0")
        heights+=($height)
    fi
done

if [ ${#heights[@]} -gt 1 ]; then
    max_height=${heights[0]}
    min_height=${heights[0]}
    for h in "${heights[@]}"; do
        if [ "$h" -gt "$max_height" ]; then
            max_height=$h
        fi
        if [ "$h" -lt "$min_height" ]; then
            min_height=$h
        fi
    done
    
    if [ "$max_height" -eq "$min_height" ]; then
        echo -e "${GREEN}✓ 모든 노드가 동기화되었습니다 (Height: $max_height)${NC}"
    else
        echo -e "${YELLOW}⚠ 노드 간 높이 차이: 최대 $max_height, 최소 $min_height${NC}"
    fi
fi

