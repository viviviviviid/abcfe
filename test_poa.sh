#!/bin/bash

# test_poa.sh - PoA 컨센서스 테스트 스크립트
# 모든 노드가 검증자로 동작하여 합의 과정을 테스트

set -e

# 색상 정의
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[1;36m'
NC='\033[0m'

NODE_COUNT=${1:-3}

echo ""
echo -e "${CYAN}╔════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║     ABCFe PoA 컨센서스 테스트 스크립트    ║${NC}"
echo -e "${CYAN}╚════════════════════════════════════════╝${NC}"
echo ""

# 1. 기존 노드 중지
echo -e "${YELLOW}[1/6] 기존 프로세스 정리${NC}"
pkill -f abcfed 2>/dev/null || true
sleep 2
echo -e "${GREEN}  ✓ 완료${NC}"

# 2. 빌드
echo -e "${YELLOW}[2/6] 빌드${NC}"
if [ ! -f "./abcfed" ] || [ "Makefile" -nt "./abcfed" ]; then
    make build
fi
echo -e "${GREEN}  ✓ 완료${NC}"

# 3. DB 초기화
echo -e "${YELLOW}[3/6] DB 초기화${NC}"
for i in $(seq 1 $NODE_COUNT); do
    if [ $i -eq 1 ]; then
        DB_PATH="./resource/db"
    else
        DB_PATH="./resource/db$i"
    fi
    rm -rf "$DB_PATH" 2>/dev/null || true
    echo "  ✓ $DB_PATH 삭제"
done

# 4. 지갑 생성
echo -e "${YELLOW}[4/6] 지갑 생성 ($NODE_COUNT개)${NC}"
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

# 5. 검증자 정보 수집
echo -e "${YELLOW}[5/7] 검증자 정보 수집${NC}"

# 각 노드의 지갑에서 주소와 공개키 추출
declare -a VALIDATOR_ADDRS
declare -a VALIDATOR_PUBKEYS

for i in $(seq 1 $NODE_COUNT); do
    if [ $i -eq 1 ]; then
        WALLET_FILE="./resource/wallet/wallet.json"
    else
        WALLET_FILE="./resource/wallet${i}/wallet.json"
    fi

    # 주소 추출 (accounts[0].address - 바이트 배열을 hex로 변환)
    ADDR=$(python3 -c "
import json
w = json.load(open('$WALLET_FILE'))
addr_bytes = w['accounts'][0]['address']
print(''.join(format(b, '02x') for b in addr_bytes))
" 2>/dev/null)
    # 공개키 추출 (accounts[0].public_key - base64를 hex로 변환)
    PUBKEY=$(python3 -c "
import json, base64
w = json.load(open('$WALLET_FILE'))
pubkey_b64 = w['accounts'][0]['public_key']
pubkey_bytes = base64.b64decode(pubkey_b64)
print(pubkey_bytes.hex())
" 2>/dev/null)

    VALIDATOR_ADDRS+=("$ADDR")
    VALIDATOR_PUBKEYS+=("$PUBKEY")
    echo "  ✓ Node $i: addr=${ADDR:0:16}... pubkey=${PUBKEY:0:16}..."
done

# 검증자 목록 TOML 생성 (모든 노드가 공유)
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
    done
    echo "]"
}

VALIDATORS_TOML=$(generate_validators_toml)
echo ""
echo -e "${GREEN}  ✓ 검증자 목록 생성 완료 (${NODE_COUNT}명)${NC}"

# 6. PoA 설정 파일 생성 (모든 노드가 검증자)
echo -e "${YELLOW}[6/7] PoA 설정 파일 생성${NC}"

# Node 1 설정 (Boot + BlockProducer)
cat > "config/config_poa_node1.toml" <<EOF
[common]
level = "local"
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
SystemAddresses = ["d79d745230218fc50118c59b0ee2c73934768a5f"]
SystemBalances = [500000000]

[server]
RestPort = 8000

[p2p]
Address = "0.0.0.0"
Port = 30303
BootNodes = []
${VALIDATORS_TOML}
EOF
echo "  ✓ config/config_poa_node1.toml (Boot + BlockProducer)"

# Node 2+ 설정 (Validator + BlockProducer)
for i in $(seq 2 $NODE_COUNT); do
    REST_PORT=$((8000 + i - 1))
    P2P_PORT=$((30303 + i - 1))

    cat > "config/config_poa_node${i}.toml" <<EOF
[common]
level = "local"
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
SystemAddresses = ["d79d745230218fc50118c59b0ee2c73934768a5f"]
SystemBalances = [500000000]

[server]
RestPort = ${REST_PORT}

[p2p]
Address = "0.0.0.0"
Port = ${P2P_PORT}
BootNodes = ["127.0.0.1:30303"]
${VALIDATORS_TOML}
EOF
    echo "  ✓ config/config_poa_node${i}.toml (Validator + BlockProducer)"
done

# 7. 노드 시작
echo -e "${YELLOW}[7/7] 노드 시작${NC}"

# 먼저 Node 1을 짧게 시작해서 제네시스 블록만 생성 (BlockProducer=false로)
echo "  Node 1 시작 (제네시스 블록 생성용)..."

# 임시로 BlockProducer=false 설정 파일 생성
cat > "config/config_poa_genesis.toml" <<EOFGEN
[common]
level = "local"
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
SystemAddresses = ["d79d745230218fc50118c59b0ee2c73934768a5f"]
SystemBalances = [500000000]

[server]
RestPort = 8000

[p2p]
Address = "0.0.0.0"
Port = 30303
BootNodes = []
${VALIDATORS_TOML}
EOFGEN

./abcfed --config="config/config_poa_genesis.toml" > /tmp/poa_genesis.log 2>&1 &
GENESIS_PID=$!
echo "  PID: $GENESIS_PID"
sleep 2  # 제네시스 블록 생성만 대기

# 제네시스 노드 중지
kill $GENESIS_PID 2>/dev/null || true
sleep 1
echo "  ✓ 제네시스 블록 생성 완료"

# Node 1의 제네시스 블록을 다른 노드에 복사
echo ""
echo -e "${YELLOW}제네시스 블록 동기화 중...${NC}"
for i in $(seq 2 $NODE_COUNT); do
    DB_PATH="./resource/db${i}"
    rm -rf "$DB_PATH" 2>/dev/null || true
    mkdir -p "$DB_PATH"
    # LevelDB 전체 복사
    cp -r ./resource/db/* "$DB_PATH/" 2>/dev/null || true
    echo "  ✓ Node $i DB 복사"
done

# 노드 순차 시작 (Node 1 먼저, 다른 노드들은 연결 후 시작)
echo ""
echo -e "${YELLOW}노드 순차 시작${NC}"

# Node 1 (Boot node) 먼저 시작
echo "  Node 1 시작 (Boot node, REST: 8000)..."
./abcfed --config="config/config_poa_node1.toml" > /tmp/poa_node1.log 2>&1 &
NODE1_PID=$!
echo "  PID: $NODE1_PID"
sleep 2  # P2P 리스너가 준비될 때까지 대기

# 나머지 노드들 시작
for i in $(seq 2 $NODE_COUNT); do
    REST_PORT=$((8000 + i - 1))
    echo "  Node $i 시작 (REST: $REST_PORT)..."
    ./abcfed --config="config/config_poa_node${i}.toml" > /tmp/poa_node${i}.log 2>&1 &
    eval "NODE${i}_PID=\$!"
    echo "  PID: $(eval echo \${NODE${i}_PID})"
    sleep 1  # 연결 시간 확보
done

# 노드들이 연결되도록 잠시 대기
sleep 2

echo ""
echo -e "${GREEN}모든 노드 시작 완료!${NC}"
echo ""

# 상태 확인 대기
echo -e "${YELLOW}동기화 대기 중... (10초)${NC}"
sleep 10

# 노드 상태 확인
echo ""
echo -e "${CYAN}╔════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║            노드 상태 확인               ║${NC}"
echo -e "${CYAN}╚════════════════════════════════════════╝${NC}"
echo ""

for i in $(seq 1 $NODE_COUNT); do
    REST_PORT=$((8000 + i - 1))
    echo -n "Node $i (REST: $REST_PORT): "

    STATUS=$(curl -s http://localhost:${REST_PORT}/api/v1/status 2>/dev/null)
    if [ -n "$STATUS" ]; then
        HEIGHT=$(echo "$STATUS" | python3 -c "import sys, json; print(json.load(sys.stdin).get('data', {}).get('currentHeight', 'N/A'))" 2>/dev/null || echo "N/A")
        PEERS=$(echo "$STATUS" | python3 -c "import sys, json; print(json.load(sys.stdin).get('data', {}).get('peerCount', 'N/A'))" 2>/dev/null || echo "N/A")
        echo -e "${GREEN}✓ Height: $HEIGHT, Peers: $PEERS${NC}"
    else
        echo -e "${RED}✗ 응답 없음${NC}"
    fi
done

echo ""
echo -e "${CYAN}=== 컨센서스 상태 확인 ===${NC}"
for i in $(seq 1 $NODE_COUNT); do
    REST_PORT=$((8000 + i - 1))
    echo -n "Node $i 컨센서스: "

    CONS=$(curl -s http://localhost:${REST_PORT}/api/v1/consensus/status 2>/dev/null)
    if [ -n "$CONS" ]; then
        STATE=$(echo "$CONS" | python3 -c "import sys, json; print(json.load(sys.stdin).get('data', {}).get('state', 'N/A'))" 2>/dev/null || echo "N/A")
        VALIDATORS=$(echo "$CONS" | python3 -c "import sys, json; print(json.load(sys.stdin).get('data', {}).get('validators', 'N/A'))" 2>/dev/null || echo "N/A")
        echo -e "State: ${GREEN}$STATE${NC}, Validators: $VALIDATORS"
    else
        echo -e "${RED}✗ 응답 없음${NC}"
    fi
done

echo ""
echo -e "${CYAN}=== 유용한 명령어 ===${NC}"
echo "로그 확인 (실시간):"
for i in $(seq 1 $NODE_COUNT); do
    echo "  tail -f /tmp/poa_node${i}.log"
done
echo ""
echo "블록 높이 비교:"
echo "  for i in \$(seq 1 $NODE_COUNT); do echo -n \"Node \$i: \"; curl -s http://localhost:\$((8000+i-1))/api/v1/status | python3 -c \"import sys, json; print(json.load(sys.stdin).get('data',{}).get('currentHeight','N/A'))\"; done"
echo ""
echo "노드 중지:"
echo "  pkill -f abcfed"
echo ""
echo "컨센서스 로그 필터:"
echo "  grep -i consensus /tmp/poa_node1.log | tail -20"
echo ""
