#!/bin/bash

# setup_multi_nodes.sh - 멀티 노드 전체 셋업 자동화 스크립트

set -e

NODE_COUNT=${1:-3}
SKIP_CONFIRM=${2:-false}  # 2번째 인자로 확인 건너뛰기

echo ""
echo -e "\033[1;36m╔════════════════════════════════════════╗[0m"
echo -e "\033[1;36m║   ABCFe 멀티 노드 자동 셋업 스크립트   ║[0m"
echo -e "\033[1;36m╚════════════════════════════════════════╝[0m"
echo ""

# 1. 기존 노드 중지
echo -e "\033[1;33m[단계 1/5] 기존 노드 중지[0m"
./stop_all_nodes.sh
echo ""

# 2. DB 초기화 (선택적)
if [ "$SKIP_CONFIRM" = "true" ] || [ "$SKIP_CONFIRM" = "yes" ]; then
    REPLY="y"
    echo "기존 블록체인 데이터를 자동으로 삭제합니다."
else
    read -p "기존 블록체인 데이터를 삭제하시겠습니까? (y/N): " -n 1 -r
    echo
fi

if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo -e "\033[1;33m[단계 2/5] DB 초기화[0m"
    for i in $(seq 1 $NODE_COUNT); do
        if [ $i -eq 1 ]; then
            DB_PATH="./resource/db"
        else
            DB_PATH="./resource/db$i"
        fi
        if [ -d "$DB_PATH" ]; then
            rm -rf "$DB_PATH"
            echo "  ✓ $DB_PATH 삭제 완료"
        fi
    done
    echo ""
else
    echo -e "\033[1;33m[단계 2/5] DB 초기화 건너뜀[0m"
    echo ""
fi

# 3. 지갑 생성
echo -e "\033[1;33m[단계 3/5] 지갑 생성 ($NODE_COUNT 개)[0m"
if ! ./create_wallets.sh $NODE_COUNT; then
    echo -e "\033[1;31m✗ 지갑 생성 실패[0m"
    echo ""
    echo "트러블슈팅:"
    echo "  1. abcfed 바이너리가 있는지 확인: ls -lh abcfed"
    echo "  2. 빌드가 필요한 경우: make build"
    echo "  3. 실행 권한 확인: chmod +x abcfed"
    exit 1
fi
echo ""

# 4. 제네시스 블록 셋업
echo -e "\033[1;33m[단계 4/5] 제네시스 블록 셋업[0m"
if ! ./setup_genesis.sh $NODE_COUNT; then
    echo -e "\033[1;31m✗ 제네시스 블록 셋업 실패[0m"
    exit 1
fi
echo ""

# 5. 노드 시작
echo -e "\033[1;33m[단계 5/5] 노드 시작 ($NODE_COUNT 개)[0m"
if ! ./start_multi_nodes.sh $NODE_COUNT; then
    echo -e "\033[1;31m✗ 노드 시작 실패[0m"
    exit 1
fi
echo ""

# 최종 상태 확인
echo -e "\033[1;36m[최종 상태 확인][0m"
sleep 3
./check_nodes.sh

echo ""
echo -e "\033[0;32m╔════════════════════════════════════════╗[0m"
echo -e "\033[0;32m║       멀티 노드 셋업 완료! 🎉         ║[0m"
echo -e "\033[0;32m╚════════════════════════════════════════╝[0m"
echo ""

echo -e "\033[1;36m=== 유용한 명령어 ===[0m"
echo "상태 확인:        ./check_nodes.sh"
echo "노드 중지:        ./stop_all_nodes.sh"
echo "노드 재시작:      ./start_multi_nodes.sh $NODE_COUNT"
echo "로그 확인:        tail -f /tmp/abcfed_node1.log"
echo ""

