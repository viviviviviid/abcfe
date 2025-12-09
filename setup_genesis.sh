#!/bin/bash

# setup_genesis.sh - 제네시스 블록 생성 및 복사 스크립트

set -e

NODE_COUNT=${1:-2}

echo -e "\033[1;33m=== 제네시스 블록 셋업 ===[0m"

# Node 1 (boot 노드) DB 경로
BOOT_DB="./resource/db/leveldb_10000.db"

# Node 1이 이미 제네시스 블록을 가지고 있는지 확인
if [ -d "$BOOT_DB" ] && [ -f "$BOOT_DB/CURRENT" ]; then
    echo "✓ Boot 노드 제네시스 블록 존재"
    HAS_GENESIS=true
else
    echo "⚠ Boot 노드 제네시스 블록 없음 - 생성 필요"
    HAS_GENESIS=false
fi

# 제네시스 블록이 없으면 Boot 노드를 잠시 시작해서 생성
if [ "$HAS_GENESIS" = false ]; then
    echo "Boot 노드 시작하여 제네시스 블록 생성 중..."
    
    # 백그라운드로 Node 1 시작
    ./abcfed > /tmp/boot_genesis.log 2>&1 &
    BOOT_PID=$!
    
    # 제네시스 블록 생성 대기 (최대 10초)
    for i in {1..10}; do
        if [ -f "$BOOT_DB/CURRENT" ]; then
            echo "✓ 제네시스 블록 생성 완료"
            break
        fi
        sleep 1
    done
    
    # Boot 노드 종료
    kill $BOOT_PID 2>/dev/null || true
    wait $BOOT_PID 2>/dev/null || true
    sleep 1
fi

# 다른 노드들에게 제네시스 블록 복사
echo ""
echo "제네시스 블록을 다른 노드들에게 복사 중..."

for i in $(seq 2 $NODE_COUNT); do
    PORT=$((10000 + i - 1))
    DB_PATH="./resource/db$i/leveldb_${PORT}.db"
    
    echo "  Node $i: $DB_PATH"
    
    # DB 디렉토리 생성
    mkdir -p "$DB_PATH"
    
    # 제네시스 블록만 복사 (MANIFEST, CURRENT, LOG, 그리고 첫 번째 SST 파일들)
    if [ -d "$BOOT_DB" ]; then
        # 전체 DB를 복사 (제네시스 블록 포함)
        cp -r "$BOOT_DB"/* "$DB_PATH/" 2>/dev/null || true
        echo "    ✓ 제네시스 블록 복사 완료"
    else
        echo "    ✗ Boot DB가 없습니다"
        exit 1
    fi
done

echo ""
echo -e "\033[0;32m✓ 제네시스 블록 셋업 완료![0m"
echo ""

