#!/bin/bash

# start_single.sh - 단일 노드 시작 스크립트
# 사용법: ./scripts/start_single.sh [옵션]
#
# 예시:
#   ./scripts/start_single.sh          # 기본 단일 노드 시작
#   ./scripts/start_single.sh -f       # DB 강제 초기화 후 시작
#   ./scripts/start_single.sh -k       # 기존 DB 유지하고 시작
#   ./scripts/start_single.sh -d       # 데몬 모드로 시작

set -e

# 프로젝트 루트로 이동 (어디서 실행해도 동작)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT" || exit 1

# 색상 정의
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[1;36m'
BLUE='\033[0;34m'
NC='\033[0m'

# 기본값
FORCE_CLEAN=false
KEEP_DB=false
DAEMON_MODE=false

# 제네시스 타임스탬프
GENESIS_TIMESTAMP=$(date +%s)

# 옵션 파싱
while [[ $# -gt 0 ]]; do
    case $1 in
        -f|--force)
            FORCE_CLEAN=true
            shift
            ;;
        -k|--keep)
            KEEP_DB=true
            shift
            ;;
        -d|--daemon)
            DAEMON_MODE=true
            shift
            ;;
        -h|--help)
            echo "사용법: ./scripts/start_single.sh [옵션]"
            echo ""
            echo "옵션:"
            echo "  -f, --force    DB 강제 초기화"
            echo "  -k, --keep     기존 DB 유지"
            echo "  -d, --daemon   데몬 모드로 시작"
            echo "  -h, --help     도움말 표시"
            echo ""
            echo "예시:"
            echo "  ./scripts/start_single.sh          # 기본 시작"
            echo "  ./scripts/start_single.sh -f       # DB 초기화 후 시작"
            echo "  ./scripts/start_single.sh -d       # 백그라운드 시작"
            exit 0
            ;;
        *)
            shift
            ;;
    esac
done

echo ""
echo -e "${CYAN}╔══════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║      ABCFe 단일 노드 시작 스크립트            ║${NC}"
echo -e "${CYAN}╚══════════════════════════════════════════════╝${NC}"
echo ""

# =============================================================================
# 1. 기존 프로세스 정리
# =============================================================================
echo -e "${YELLOW}[1/6] 기존 프로세스 정리${NC}"
pkill -f abcfed 2>/dev/null || true
sleep 2
echo -e "${GREEN}  ✓ 완료${NC}"

# =============================================================================
# 2. 빌드 확인
# =============================================================================
echo -e "${YELLOW}[2/6] 빌드 확인${NC}"
if [ ! -f "./abcfed" ]; then
    echo "  빌드 중..."
    make build
else
    echo "  ✓ abcfed 바이너리 존재"
fi
echo -e "${GREEN}  ✓ 완료${NC}"

# =============================================================================
# 3. DB 초기화 (옵션에 따라)
# =============================================================================
echo -e "${YELLOW}[3/6] DB 처리${NC}"
if [ "$KEEP_DB" = true ]; then
    echo "  ✓ 기존 DB 유지 (-k 옵션)"
else
    rm -rf "./resource/db" 2>/dev/null || true
    echo "  ✓ ./resource/db 삭제"
fi
echo -e "${GREEN}  ✓ 완료${NC}"

# =============================================================================
# 4. 지갑 생성
# =============================================================================
echo -e "${YELLOW}[4/6] 지갑 생성${NC}"
WALLET_DIR="./resource/wallet"
mkdir -p "$WALLET_DIR"

if [ ! -f "$WALLET_DIR/wallet.json" ]; then
    ./abcfed wallet create --wallet-dir="$WALLET_DIR" > /dev/null 2>&1
    echo "  ✓ 지갑 생성: $WALLET_DIR"
else
    echo "  ✓ 지갑 존재: $WALLET_DIR"
fi
echo -e "${GREEN}  ✓ 완료${NC}"

# =============================================================================
# 5. 검증자 정보 수집 및 설정 파일 생성
# =============================================================================
echo -e "${YELLOW}[5/6] 설정 파일 생성${NC}"

WALLET_FILE="./resource/wallet/wallet.json"

# 주소 추출
ADDR=$(python3 -c "
import json
w = json.load(open('$WALLET_FILE'))
addr_bytes = w['accounts'][0]['address']
print(''.join(format(b, '02x') for b in addr_bytes))
" 2>/dev/null)

# 공개키 추출
PUBKEY=$(python3 -c "
import json, base64
w = json.load(open('$WALLET_FILE'))
pubkey_b64 = w['accounts'][0]['public_key']
pubkey_bytes = base64.b64decode(pubkey_b64)
print(pubkey_bytes.hex())
" 2>/dev/null)

echo "  ✓ 주소: ${ADDR:0:16}..."

# 설정 파일 생성
cat > "config/config.toml" <<EOF
[common]
level = "local"
serviceName = "abcfe-node"
port = 10000
mode = "boot"
networkID = "abcfe-mainnet"
blockProducer = true

[logInfo]
path = "./log/syslogs/"
maxAgeHour = 1440
rotateHour = 24
ProdTelKey = ""
ProdChatId = 0
DevTelKey = ""
DevChatId = 0

[db]
path = "./resource/db/"

[wallet]
path = "./resource/wallet"

[version]
protocol = "1.0.0"
transaction = "1.0.0"

[genesis]
SystemAddresses = ["${ADDR}"]
SystemBalances = [500000000]
Timestamp = ${GENESIS_TIMESTAMP}

[server]
RestPort = 8000
InternalRestPort = 8800

[p2p]
Address = "0.0.0.0"
Port = 30303
BootNodes = []

[fee]
minFee = 1
blockReward = 50

[transaction]
maxMemoSize = 256
maxDataSize = 1024

[consensus]
proposerSelection = "roundrobin"

[validators]
list = [
  { address = "${ADDR}", publicKey = "${PUBKEY}", votingPower = 1000 }
]
EOF

echo "  ✓ config/config.toml 생성"
echo -e "${GREEN}  ✓ 완료${NC}"

# =============================================================================
# 6. 노드 시작
# =============================================================================
echo -e "${YELLOW}[6/6] 노드 시작${NC}"

if [ "$DAEMON_MODE" = true ]; then
    echo "  데몬 모드로 시작 중..."
    ./abcfed > /tmp/abcfed_single.log 2>&1 &
    NODE_PID=$!
    echo "  PID: $NODE_PID"
    echo $NODE_PID > /tmp/abcfed_single.pid
else
    echo "  포그라운드 모드로 시작 중..."
    echo "  (Ctrl+C로 종료)"
    echo ""
fi

echo ""
echo -e "${GREEN}✓ 노드 시작 완료!${NC}"
echo ""

# =============================================================================
# 상태 확인 (데몬 모드일 때만)
# =============================================================================
if [ "$DAEMON_MODE" = true ]; then
    echo -e "${YELLOW}상태 확인 대기 중... (5초)${NC}"
    sleep 5

    echo ""
    echo -e "${CYAN}╔══════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║              노드 상태 요약                   ║${NC}"
    echo -e "${CYAN}╚══════════════════════════════════════════════╝${NC}"
    echo ""

    STATUS=$(curl -s http://localhost:8000/api/v1/status 2>/dev/null)

    if [ -n "$STATUS" ]; then
        HEIGHT=$(echo "$STATUS" | python3 -c "import sys, json; print(json.load(sys.stdin).get('data', {}).get('currentHeight', 'N/A'))" 2>/dev/null || echo "N/A")
        echo -e "  REST Port: ${GREEN}8000${NC}"
        echo -e "  P2P Port:  ${GREEN}30303${NC}"
        echo -e "  Height:    ${GREEN}${HEIGHT}${NC}"
        echo -e "  Address:   ${CYAN}${ADDR}${NC}"
    else
        echo -e "  상태: ${RED}연결 실패${NC}"
        echo "  로그 확인: tail -f /tmp/abcfed_single.log"
    fi

    echo ""
    echo -e "${CYAN}=== 유용한 명령어 ===${NC}"
    echo ""
    echo "📋 상태 확인:"
    echo "   curl -s http://localhost:8000/api/v1/status | python3 -m json.tool"
    echo ""
    echo "📝 로그 확인:"
    echo "   tail -f /tmp/abcfed_single.log"
    echo ""
    echo "🔍 컨센서스 상태:"
    echo "   curl -s http://localhost:8000/api/v1/consensus/status | python3 -m json.tool"
    echo ""
    echo "💰 잔액 확인:"
    echo "   curl -s http://localhost:8000/api/v1/address/${ADDR}/balance"
    echo ""
    echo "🛑 노드 중지:"
    echo "   pkill -f abcfed  또는  kill \$(cat /tmp/abcfed_single.pid)"
    echo ""
else
    # 포그라운드 모드로 실행
    ./abcfed
fi
