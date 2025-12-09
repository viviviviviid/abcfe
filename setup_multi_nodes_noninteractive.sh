#!/bin/bash

# setup_multi_nodes_noninteractive.sh - 비대화형 멀티 노드 셋업
# 원격 서버 또는 자동화에 사용

set -e

NODE_COUNT=${1:-3}

echo ""
echo "=== ABCFe 멀티 노드 자동 셋업 (비대화형) ==="
echo ""

# 1. 기존 노드 중지
echo "[1/5] 기존 노드 중지"
./stop_all_nodes.sh 2>&1 | head -5
echo ""

# 2. DB 자동 초기화
echo "[2/5] DB 자동 초기화"
for i in $(seq 1 $NODE_COUNT); do
    if [ $i -eq 1 ]; then
        DB_PATH="./resource/db"
    else
        DB_PATH="./resource/db$i"
    fi
    if [ -d "$DB_PATH" ]; then
        rm -rf "$DB_PATH"
        echo "  ✓ $DB_PATH 삭제"
    fi
done
echo ""

# 3. 지갑 생성
echo "[3/5] 지갑 생성"
if ! ./create_wallets.sh $NODE_COUNT 2>&1 | grep -E "Node|Address|완료|오류|실패" || [ ${PIPESTATUS[0]} -ne 0 ]; then
    echo "✗ 지갑 생성 실패"
    echo ""
    echo "트러블슈팅:"
    echo "  1. abcfed 바이너리 확인: ls -lh abcfed"
    echo "  2. 빌드: make build"
    echo "  3. 권한: chmod +x abcfed"
    exit 1
fi
echo ""

# 4. 제네시스 블록 셋업
echo "[4/5] 제네시스 블록 셋업"
if ! ./setup_genesis.sh $NODE_COUNT 2>&1 | grep -E "✓|완료|생성|오류|실패" || [ ${PIPESTATUS[0]} -ne 0 ]; then
    echo "✗ 제네시스 블록 셋업 실패"
    exit 1
fi
echo ""

# 5. 노드 시작
echo "[5/5] 노드 시작"
if ! ./start_multi_nodes.sh $NODE_COUNT 2>&1 | tail -20; then
    echo "✗ 노드 시작 실패"
    exit 1
fi
echo ""

echo "=== 셋업 완료 ==="
echo ""
echo "상태 확인: ./check_nodes.sh"
echo "노드 중지: ./stop_all_nodes.sh"
echo ""
