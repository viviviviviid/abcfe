#!/bin/bash

# start_poa.sh - N개 노드 PoA 컨센서스 시작 스크립트
# 사용법: ./start_poa.sh [노드 수] [옵션]
#
# 예시:
#   ./start_poa.sh 3      # 3개 노드로 PoA 시작
#   ./start_poa.sh 9      # 9개 노드로 PoA 시작
#   ./start_poa.sh 5 -f   # 5개 노드, DB 강제 초기화
#   ./start_poa.sh 3 -k   # 3개 노드, 기존 DB 유지

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
FORCE_CLEAN=false
KEEP_DB=false

# 제네시스 타임스탬프 (스크립트 실행 시점 - 모든 노드가 동일한 제네시스 블록을 갖도록)
GENESIS_TIMESTAMP=$(date +%s)

# 옵션 파싱
shift || true
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
        -h|--help)
            echo "사용법: ./start_poa.sh [노드 수] [옵션]"
            echo ""
            echo "옵션:"
            echo "  -f, --force    DB 강제 초기화"
            echo "  -k, --keep     기존 DB 유지"
            echo "  -h, --help     도움말 표시"
            echo ""
            echo "예시:"
            echo "  ./start_poa.sh 3      # 3개 노드"
            echo "  ./start_poa.sh 9 -f   # 9개 노드, DB 초기화"
            exit 0
            ;;
        *)
            shift
            ;;
    esac
done

# 노드 수 검증
if [ "$NODE_COUNT" -lt 1 ] || [ "$NODE_COUNT" -gt 20 ]; then
    echo -e "${RED}오류: 노드 수는 1~20 사이여야 합니다.${NC}"
    exit 1
fi

echo ""
echo -e "${CYAN}╔══════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║      ABCFe PoA 컨센서스 시작 스크립트         ║${NC}"
echo -e "${CYAN}║      노드 수: ${NODE_COUNT}개                              ║${NC}"
echo -e "${CYAN}╚══════════════════════════════════════════════╝${NC}"
echo ""

# =============================================================================
# 1. 기존 프로세스 정리
# =============================================================================
echo -e "${YELLOW}[1/7] 기존 프로세스 정리${NC}"
pkill -f abcfed 2>/dev/null || true
sleep 2
echo -e "${GREEN}  ✓ 완료${NC}"

# =============================================================================
# 2. 빌드 확인
# =============================================================================
echo -e "${YELLOW}[2/7] 빌드 확인${NC}"
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
echo -e "${YELLOW}[3/7] DB 처리${NC}"
if [ "$KEEP_DB" = true ]; then
    echo "  ✓ 기존 DB 유지 (-k 옵션)"
else
    for i in $(seq 1 $NODE_COUNT); do
        if [ $i -eq 1 ]; then
            DB_PATH="./resource/db"
        else
            DB_PATH="./resource/db$i"
        fi
        rm -rf "$DB_PATH" 2>/dev/null || true
        echo "  ✓ $DB_PATH 삭제"
    done
fi
echo -e "${GREEN}  ✓ 완료${NC}"

# =============================================================================
# 4. 지갑 생성
# =============================================================================
echo -e "${YELLOW}[4/7] 지갑 생성 (${NODE_COUNT}개)${NC}"
for i in $(seq 1 $NODE_COUNT); do
    if [ $i -eq 1 ]; then
        WALLET_DIR="./resource/wallet"
    else
        WALLET_DIR="./resource/wallet$i"
    fi

    mkdir -p "$WALLET_DIR"

    if [ ! -f "$WALLET_DIR/wallet.json" ]; then
        ./abcfed wallet create --wallet-dir="$WALLET_DIR" > /dev/null 2>&1
        echo "  ✓ Node $i 지갑 생성: $WALLET_DIR"
    else
        echo "  ✓ Node $i 지갑 존재: $WALLET_DIR"
    fi
done
echo -e "${GREEN}  ✓ 완료${NC}"

# =============================================================================
# 5. 검증자 정보 수집
# =============================================================================
echo -e "${YELLOW}[5/7] 검증자 정보 수집${NC}"

declare -a VALIDATOR_ADDRS
declare -a VALIDATOR_PUBKEYS

for i in $(seq 1 $NODE_COUNT); do
    if [ $i -eq 1 ]; then
        WALLET_FILE="./resource/wallet/wallet.json"
    else
        WALLET_FILE="./resource/wallet${i}/wallet.json"
    fi

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

    VALIDATOR_ADDRS+=("$ADDR")
    VALIDATOR_PUBKEYS+=("$PUBKEY")
    echo "  ✓ Node $i: ${ADDR:0:16}..."
    sleep 1
done

# 검증자 목록 TOML 생성
generate_validators_toml() {
    echo ""
    echo "[validators]"
    echo "list = ["
    for i in $(seq 0 $((NODE_COUNT - 1))); do
        ADDR="${VALIDATOR_ADDRS[$i]}"
        PUBKEY="${VALIDATOR_PUBKEYS[$i]}"
        if [ $i -eq $((NODE_COUNT - 1)) ]; then
            echo "  { address = \"$ADDR\", publicKey = \"$PUBKEY\", votingPower = 1000 }"
        else
            echo "  { address = \"$ADDR\", publicKey = \"$PUBKEY\", votingPower = 1000 },"
        fi
        sleep 1
    done
    echo "]"
}

VALIDATORS_TOML=$(generate_validators_toml)
echo -e "${GREEN}  ✓ 검증자 목록 생성 완료 (${NODE_COUNT}명)${NC}"

# =============================================================================
# 6. 설정 파일 생성
# =============================================================================
echo -e "${YELLOW}[6/7] 설정 파일 생성${NC}"

# 제네시스 주소 (Node 1의 주소 사용)
GENESIS_ADDR="${VALIDATOR_ADDRS[0]}"

# Node 1 설정 (Boot + BlockProducer)
cat > "config/config_poa_node1.toml" <<EOF
[common]
level = "alpha"
serviceName = "abcfe-poa-node-1"
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
SystemAddresses = ["${GENESIS_ADDR}"]
SystemBalances = [500000000]
Timestamp = ${GENESIS_TIMESTAMP}

[server]
RestPort = 8000

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
${VALIDATORS_TOML}
EOF
echo "  ✓ config/config_poa_node1.toml (Boot)"

# Node 2+ 설정
for i in $(seq 2 $NODE_COUNT); do
    REST_PORT=$((8000 + i - 1))
    P2P_PORT=$((30303 + i - 1))

    cat > "config/config_poa_node${i}.toml" <<EOF
[common]
level = "alpha"
serviceName = "abcfe-poa-node-${i}"
port = $((10000 + i - 1))
mode = "validator"
networkID = "abcfe-mainnet"
blockProducer = true

[logInfo]
path = "./log/syslogs${i}/"
maxAgeHour = 1440
rotateHour = 24
ProdTelKey = ""
ProdChatId = 0
DevTelKey = ""
DevChatId = 0

[db]
path = "./resource/db${i}/"

[wallet]
path = "./resource/wallet${i}"

[version]
protocol = "1.0.0"
transaction = "1.0.0"

[genesis]
SystemAddresses = ["${GENESIS_ADDR}"]
SystemBalances = [500000000]
Timestamp = ${GENESIS_TIMESTAMP}

[server]
RestPort = ${REST_PORT}

[p2p]
Address = "0.0.0.0"
Port = ${P2P_PORT}
BootNodes = ["127.0.0.1:30303"]

[fee]
minFee = 1
blockReward = 50

[transaction]
maxMemoSize = 256
maxDataSize = 1024
${VALIDATORS_TOML}
EOF
    echo "  ✓ config/config_poa_node${i}.toml (Validator)"
done
echo -e "${GREEN}  ✓ 완료${NC}"

# =============================================================================
# 7. 노드 시작
# =============================================================================
echo -e "${YELLOW}[7/7] 노드 시작${NC}"

# 제네시스 블록 생성용 임시 시작
if [ "$KEEP_DB" = false ]; then
    echo "  제네시스 블록 생성 중..."

    # 임시 설정 (blockProducer=false)
    cat > "config/config_poa_genesis.toml" <<EOFGEN
[common]
level = "alpha"
serviceName = "abcfe-poa-genesis"
port = 10000
mode = "boot"
networkID = "abcfe-mainnet"
blockProducer = false

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
SystemAddresses = ["${GENESIS_ADDR}"]
SystemBalances = [500000000]
Timestamp = ${GENESIS_TIMESTAMP}

[server]
RestPort = 8000

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
${VALIDATORS_TOML}
EOFGEN

    ./abcfed --config="config/config_poa_genesis.toml" > /tmp/poa_genesis.log 2>&1 &
    GENESIS_PID=$!
    sleep 5
    kill $GENESIS_PID 2>/dev/null || true
    sleep 5
    echo "  ✓ 제네시스 블록 생성 완료"

    # 다른 노드에 DB 복사
    for i in $(seq 2 $NODE_COUNT); do
        DB_PATH="./resource/db${i}"
        rm -rf "$DB_PATH" 2>/dev/null || true
        mkdir -p "$DB_PATH"
        cp -r ./resource/db/* "$DB_PATH/" 2>/dev/null || true
        echo "  ✓ Node $i DB 복사"
    done
fi

# 노드 순차 시작
echo ""
echo -e "${BLUE}노드 순차 시작 중...${NC}"

# Node 1 (Boot)
echo "  Node 1 시작 (Boot, REST: 8000, P2P: 30303)..."
./abcfed --config="config/config_poa_node1.toml" > /tmp/poa_node1.log 2>&1 &
echo "  PID: $!"
sleep 3

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
echo -e "${YELLOW}동기화 대기 중... (10초)${NC}"
sleep 10

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
echo "🔍 컨센서스 상태:"
echo "   curl -s http://localhost:8000/api/v1/consensus/status | python3 -m json.tool"
echo ""
echo "🛑 노드 중지:"
echo "   pkill -f abcfed"
echo ""
