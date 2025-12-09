#!/bin/bash

# 여러 노드용 지갑 생성 스크립트
# 사용법: ./create_wallets.sh [노드 개수]

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

NODE_COUNT=${1:-2}

if [ "$NODE_COUNT" -lt 1 ] || [ "$NODE_COUNT" -gt 10 ]; then
    echo -e "${RED}Error: 노드 개수는 1-10 사이여야 합니다.${NC}"
    exit 1
fi

echo -e "${GREEN}=== ABCFe 노드 지갑 생성 ===${NC}"
echo "노드 개수: $NODE_COUNT"
echo ""

# 빌드 확인
if [ ! -f "./abcfed" ]; then
    echo -e "${YELLOW}빌드 파일이 없습니다. 빌드 중...${NC}"
    make build
fi

# 각 노드별 지갑 생성
for i in $(seq 1 $NODE_COUNT); do
    if [ "$i" -eq 1 ]; then
        wallet_dir="./resource/wallet"
    else
        wallet_dir="./resource/wallet${i}"
    fi
    
    echo -e "${YELLOW}=== Node $i 지갑 ===${NC}"
    
    # 지갑 디렉토리 생성
    mkdir -p "$wallet_dir"
    
    # 이미 지갑이 있는지 확인
    if [ -f "${wallet_dir}/wallet.json" ]; then
        echo -e "${GREEN}✓ 지갑 이미 존재: ${wallet_dir}/wallet.json${NC}"
        
        # 주소 표시
        echo "계정 정보:"
        ./abcfed wallet list --wallet-dir="$wallet_dir" 2>/dev/null | grep -A 2 "Address:" || true
    else
        echo "지갑 생성 중: ${wallet_dir}/"
        
        # 지갑 생성
        output=$(./abcfed wallet create --wallet-dir="$wallet_dir" 2>&1)
        
        if [ -f "${wallet_dir}/wallet.json" ]; then
            echo -e "${GREEN}✓ 지갑 생성 완료${NC}"
            echo ""
            echo "니모닉 (안전하게 보관하세요!):"
            echo "$output" | grep "Mnemonic:" | cut -d: -f2- | xargs
            echo ""
            echo "주소:"
            echo "$output" | grep "Address:" | cut -d: -f2- | xargs
        else
            echo -e "${RED}✗ 지갑 생성 실패${NC}"
            echo "$output"
        fi
    fi
    echo ""
done

echo -e "${GREEN}=== 완료 ===${NC}"
echo ""
echo "지갑 목록 확인:"
for i in $(seq 1 $NODE_COUNT); do
    if [ "$i" -eq 1 ]; then
        wallet_dir="./resource/wallet"
    else
        wallet_dir="./resource/wallet${i}"
    fi
    echo "  ./abcfed wallet list --wallet-dir=\"$wallet_dir\""
done

