#!/bin/bash

# ABCFe 멀티 노드 실행 스크립트
# 사용법: ./start_multi_nodes.sh [노드 개수]

set -e

# 색상 정의
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 노드 개수 (기본값: 2)
NODE_COUNT=${1:-2}

if [ "$NODE_COUNT" -lt 1 ] || [ "$NODE_COUNT" -gt 10 ]; then
    echo -e "${RED}Error: 노드 개수는 1-10 사이여야 합니다.${NC}"
    exit 1
fi

echo -e "${GREEN}=== ABCFe 멀티 노드 시작 ===${NC}"
echo "노드 개수: $NODE_COUNT"
echo ""

# 기존 프로세스 정리
echo -e "${YELLOW}기존 노드 프로세스 정리 중...${NC}"
pkill -f abcfed 2>/dev/null || true
sleep 2

# 빌드 확인
if [ ! -f "./abcfed" ]; then
    echo -e "${YELLOW}빌드 파일이 없습니다. 빌드 중...${NC}"
    make build
fi

# 노드별 설정 파일 생성
create_node_config() {
    local node_num=$1
    local config_file="config/config_node${node_num}.toml"
    
    if [ "$node_num" -eq 1 ]; then
        # Node 1은 기본 설정 사용
        return
    fi
    
    local port=$((10000 + node_num - 1))
    local rest_port=$((8000 + node_num - 1))
    local p2p_port=$((30303 + node_num - 1))
    local boot_node_port=30303
    
    cat > "$config_file" <<EOF
[common]
level = "local"
serviceName = "abcfe-node-${node_num}"
port = ${port}
mode = "validator"
networkID = "abcfe-mainnet"
blockProducer = false

[logInfo]
path = "./log/syslogs${node_num}/"
maxAgeHour = 1440
rotateHour = 24
ProdTelKey = ""
ProdChatId = 0
DevTelKey = ""
DevChatId = 0

[db]
path = "./resource/db${node_num}/"

[wallet]
path = "./resource/wallet${node_num}"

[version]
protocol = "1.0.0"
transaction = "1.0.0"

[genesis]
SystemAddresses = ["17112bdf0c4b66abcb6b9651538a36da97031cca"]
SystemBalances = [500000000]

[server]
RestPort = ${rest_port}

[p2p]
Address = "0.0.0.0"
Port = ${p2p_port}
BootNodes = ["127.0.0.1:${boot_node_port}"]
EOF
    
    echo "설정 파일 생성: $config_file"
}

# 지갑 생성 또는 확인
create_or_check_wallet() {
    local node_num=$1
    local wallet_dir=""
    
    if [ "$node_num" -eq 1 ]; then
        wallet_dir="./resource/wallet"
    else
        wallet_dir="./resource/wallet${node_num}"
    fi
    
    # 지갑 디렉토리 생성
    mkdir -p "$wallet_dir"
    
    # 지갑 파일이 존재하는지 확인
    if [ -f "${wallet_dir}/wallet.json" ]; then
        echo "  지갑 존재: ${wallet_dir}/wallet.json"
        return 0
    fi
    
    # 지갑 생성
    echo "  지갑 생성 중: ${wallet_dir}/"
    ./abcfed wallet create --wallet-dir="$wallet_dir" > /tmp/wallet_node${node_num}.log 2>&1
    
    if [ -f "${wallet_dir}/wallet.json" ]; then
        echo -e "  ${GREEN}✓ 지갑 생성 완료${NC}"
        return 0
    else
        echo -e "  ${RED}✗ 지갑 생성 실패${NC}"
        echo "    로그: /tmp/wallet_node${node_num}.log"
        return 1
    fi
}

# 노드 시작
start_node() {
    local node_num=$1
    local config_file=""
    
    # 지갑 생성/확인
    echo -e "${YELLOW}Node ${node_num} 지갑 확인...${NC}"
    if ! create_or_check_wallet $node_num; then
        echo -e "${RED}Error: Node ${node_num} 지갑 생성 실패${NC}"
        return 1
    fi
    
    if [ "$node_num" -eq 1 ]; then
        echo -e "${GREEN}=== Node 1 시작 (Port 10000, REST 8000) ===${NC}"
        ./abcfed > /tmp/abcfed_node1.log 2>&1 &
        NODE1_PID=$!
        echo "Node 1 PID: $NODE1_PID"
    else
        create_node_config $node_num
        config_file="config/config_node${node_num}.toml"
        local rest_port=$((8000 + node_num - 1))
        
        echo -e "${GREEN}=== Node ${node_num} 시작 (REST ${rest_port}) ===${NC}"
        ./abcfed --config="$config_file" > /tmp/abcfed_node${node_num}.log 2>&1 &
        eval "NODE${node_num}_PID=\$!"
        echo "Node ${node_num} PID: $(eval echo \${NODE${node_num}_PID})"
    fi
}

# 모든 노드 시작
for i in $(seq 1 $NODE_COUNT); do
    start_node $i
    sleep 3  # 각 노드 시작 간격
done

echo ""
echo -e "${GREEN}=== 모든 노드 시작 완료 ===${NC}"
echo ""
echo "노드 상태 확인 중..."
sleep 5

# 노드 상태 확인
echo ""
echo -e "${YELLOW}=== 노드 상태 확인 ===${NC}"
for i in $(seq 1 $NODE_COUNT); do
    rest_port=$((8000 + i - 1))
    echo -n "Node $i (REST: $rest_port): "
    if curl -s http://localhost:${rest_port}/api/v1/status > /dev/null 2>&1; then
        height=$(curl -s http://localhost:${rest_port}/api/v1/status | python3 -c "import sys, json; print(json.load(sys.stdin)['data']['currentHeight'])" 2>/dev/null || echo "N/A")
        echo -e "${GREEN}✓ 실행 중 (Height: $height)${NC}"
    else
        echo -e "${RED}✗ 응답 없음${NC}"
    fi
done

echo ""
echo -e "${GREEN}=== 사용 가능한 명령어 ===${NC}"
echo "노드 상태 확인:"
for i in $(seq 1 $NODE_COUNT); do
    rest_port=$((8000 + i - 1))
    echo "  curl http://localhost:${rest_port}/api/v1/status"
done
echo ""
echo "모든 노드 중지:"
echo "  ./stop_all_nodes.sh"
echo ""
echo "로그 확인:"
for i in $(seq 1 $NODE_COUNT); do
    echo "  tail -f /tmp/abcfed_node${i}.log"
done
echo ""
echo "동기화 확인:"
echo "  ./check_nodes.sh"

