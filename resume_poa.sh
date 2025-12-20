#!/bin/bash

# resume_poa.sh - 기존 DB/지갑 유지하면서 PoA 노드 재시작
# 사용법: ./resume_poa.sh [노드 수]
#
# 예시:
#   ./resume_poa.sh 3      # 3개 노드로 재시작
#   ./resume_poa.sh 9      # 9개 노드로 재시작

set -e

# 색상 정의
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[1;36m'
BLUE='\033[0;34m'
NC='\033[0m'

# 기본값
NODE_COUNT=${1:-3}

# 노드 수 검증
if [ "$NODE_COUNT" -lt 1 ] || [ "$NODE_COUNT" -gt 20 ]; then
    echo -e "${RED}오류: 노드 수는 1~20 사이여야 합니다.${NC}"
    exit 1
fi

echo ""
echo -e "${CYAN}╔══════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║      ABCFe PoA 노드 재시작 스크립트           ║${NC}"
echo -e "${CYAN}║      노드 수: ${NODE_COUNT}개 (DB/지갑 유지)             ║${NC}"
echo -e "${CYAN}╚══════════════════════════════════════════════╝${NC}"
echo ""

# =============================================================================
# 1. 사전 검증
# =============================================================================
echo -e "${YELLOW}[1/4] 사전 검증${NC}"

# 바이너리 확인
if [ ! -f "./abcfed" ]; then
    echo -e "${RED}  ✗ abcfed 바이너리가 없습니다. make build를 먼저 실행하세요.${NC}"
    exit 1
fi
echo "  ✓ abcfed 바이너리 확인"

# 설정 파일 확인
for i in $(seq 1 $NODE_COUNT); do
    CONFIG_FILE="config/config_poa_node${i}.toml"
    if [ ! -f "$CONFIG_FILE" ]; then
        echo -e "${RED}  ✗ 설정 파일 없음: $CONFIG_FILE${NC}"
        echo -e "${RED}    먼저 ./start_poa.sh $NODE_COUNT 를 실행하여 초기 설정을 생성하세요.${NC}"
        exit 1
    fi
done
echo "  ✓ 설정 파일 확인 (${NODE_COUNT}개)"

# DB 확인
DB_EXISTS=false
if [ -d "./resource/db" ] && [ "$(ls -A ./resource/db 2>/dev/null)" ]; then
    DB_EXISTS=true
fi

if [ "$DB_EXISTS" = false ]; then
    echo -e "${RED}  ✗ DB가 존재하지 않습니다.${NC}"
    echo -e "${RED}    먼저 ./start_poa.sh $NODE_COUNT 를 실행하여 제네시스 블록을 생성하세요.${NC}"
    exit 1
fi
echo "  ✓ DB 존재 확인"

# 지갑 확인
for i in $(seq 1 $NODE_COUNT); do
    if [ $i -eq 1 ]; then
        WALLET_FILE="./resource/wallet/wallet.json"
    else
        WALLET_FILE="./resource/wallet${i}/wallet.json"
    fi

    if [ ! -f "$WALLET_FILE" ]; then
        echo -e "${RED}  ✗ 지갑 파일 없음: $WALLET_FILE${NC}"
        exit 1
    fi
done
echo "  ✓ 지갑 파일 확인 (${NODE_COUNT}개)"

echo -e "${GREEN}  ✓ 사전 검증 완료${NC}"

# =============================================================================
# 2. 기존 프로세스 정리
# =============================================================================
echo -e "${YELLOW}[2/4] 기존 프로세스 정리${NC}"

RUNNING_COUNT=$(pgrep -f abcfed | wc -l | tr -d ' ')
if [ "$RUNNING_COUNT" -gt 0 ]; then
    echo "  기존 abcfed 프로세스 ${RUNNING_COUNT}개 종료 중..."
    pkill -f abcfed 2>/dev/null || true
    sleep 3

    # 강제 종료 확인
    STILL_RUNNING=$(pgrep -f abcfed | wc -l | tr -d ' ')
    if [ "$STILL_RUNNING" -gt 0 ]; then
        echo "  강제 종료 중..."
        pkill -9 -f abcfed 2>/dev/null || true
        sleep 2
    fi
fi
echo -e "${GREEN}  ✓ 완료${NC}"

# =============================================================================
# 3. 현재 상태 확인
# =============================================================================
echo -e "${YELLOW}[3/4] 기존 데이터 상태 확인${NC}"

# Node 1 DB에서 현재 높이 확인 (가능한 경우)
echo "  DB 디렉토리:"
for i in $(seq 1 $NODE_COUNT); do
    if [ $i -eq 1 ]; then
        DB_PATH="./resource/db"
    else
        DB_PATH="./resource/db${i}"
    fi

    if [ -d "$DB_PATH" ]; then
        SIZE=$(du -sh "$DB_PATH" 2>/dev/null | cut -f1)
        echo "    Node $i: $DB_PATH ($SIZE)"
    else
        echo -e "    ${RED}Node $i: $DB_PATH (없음)${NC}"
    fi
done

echo -e "${GREEN}  ✓ 완료${NC}"

# =============================================================================
# 4. 노드 시작
# =============================================================================
echo -e "${YELLOW}[4/4] 노드 시작${NC}"
echo ""

# Node 1 (Boot)
echo "  Node 1 시작 (Boot, REST: 8000, P2P: 30303)..."
./abcfed --config="config/config_poa_node1.toml" > /tmp/poa_node1.log 2>&1 &
NODE1_PID=$!
echo "  PID: $NODE1_PID"
sleep 3

# Node 1 상태 확인
if ! kill -0 $NODE1_PID 2>/dev/null; then
    echo -e "${RED}  ✗ Node 1 시작 실패. 로그 확인: tail -f /tmp/poa_node1.log${NC}"
    exit 1
fi

# 나머지 노드들
for i in $(seq 2 $NODE_COUNT); do
    REST_PORT=$((8000 + i - 1))
    P2P_PORT=$((30303 + i - 1))
    echo "  Node $i 시작 (REST: $REST_PORT, P2P: $P2P_PORT)..."
    ./abcfed --config="config/config_poa_node${i}.toml" > /tmp/poa_node${i}.log 2>&1 &
    echo "  PID: $!"
    sleep 1
done

echo ""
echo -e "${GREEN}✓ 모든 노드 시작 완료!${NC}"
echo ""

# =============================================================================
# 상태 확인
# =============================================================================
echo -e "${YELLOW}동기화 대기 중... (5초)${NC}"
sleep 5

echo ""
echo -e "${CYAN}╔══════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║              노드 상태 요약                   ║${NC}"
echo -e "${CYAN}╚══════════════════════════════════════════════╝${NC}"
echo ""

# 테이블 헤더
printf "%-8s %-10s %-10s %-10s %-10s\n" "Node" "REST" "P2P" "Height" "Peers"
printf "%-8s %-10s %-10s %-10s %-10s\n" "--------" "----------" "----------" "----------" "----------"

for i in $(seq 1 $NODE_COUNT); do
    REST_PORT=$((8000 + i - 1))
    P2P_PORT=$((30303 + i - 1))

    STATUS=$(curl -s http://localhost:${REST_PORT}/api/v1/status 2>/dev/null)
    PEER_STATUS=$(curl -s http://localhost:${REST_PORT}/api/v1/p2p/status 2>/dev/null)

    if [ -n "$STATUS" ]; then
        HEIGHT=$(echo "$STATUS" | python3 -c "import sys, json; print(json.load(sys.stdin).get('data', {}).get('currentHeight', 'N/A'))" 2>/dev/null || echo "N/A")
    else
        HEIGHT="ERROR"
    fi

    if [ -n "$PEER_STATUS" ]; then
        PEERS=$(echo "$PEER_STATUS" | python3 -c "import sys, json; print(json.load(sys.stdin).get('data', {}).get('peerCount', 'N/A'))" 2>/dev/null || echo "N/A")
    else
        PEERS="-"
    fi

    if [ "$HEIGHT" != "ERROR" ]; then
        printf "%-8s %-10s %-10s ${GREEN}%-10s${NC} %-10s\n" "Node $i" "$REST_PORT" "$P2P_PORT" "$HEIGHT" "$PEERS"
    else
        printf "%-8s %-10s %-10s ${RED}%-10s${NC} %-10s\n" "Node $i" "$REST_PORT" "$P2P_PORT" "$HEIGHT" "$PEERS"
    fi
done

echo ""
echo -e "${CYAN}=== 유용한 명령어 ===${NC}"
echo ""
echo "📋 블록 높이 비교:"
echo "   for i in \$(seq 1 $NODE_COUNT); do echo -n \"Node \$i: \"; curl -s http://localhost:\$((8000+i-1))/api/v1/status | python3 -c \"import sys, json; print(json.load(sys.stdin).get('data',{}).get('currentHeight','N/A'))\"; done"
echo ""
echo "📝 로그 확인:"
echo "   tail -f /tmp/poa_node1.log                    # Node 1 로그"
echo "   tail -f /tmp/poa_node*.log                    # 모든 노드 로그"
echo ""
echo "🔍 JSON-RPC 테스트:"
echo "   curl -X POST http://localhost:8000/rpc -H 'Content-Type: application/json' -d '{\"jsonrpc\":\"2.0\",\"method\":\"abcfe_getStatus\",\"id\":1}'"
echo ""
echo "🛑 노드 중지:"
echo "   pkill -f abcfed"
echo ""
