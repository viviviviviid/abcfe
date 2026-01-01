#!/bin/bash

# clean_all.sh - 모든 노드 데이터 정리 스크립트

# 프로젝트 루트로 이동 (어디서 실행해도 동작)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT" || exit 1

echo -e "\033[1;31m=== ABCFe 노드 데이터 정리 ===[0m"
echo ""

# 확인
read -p "⚠️  모든 노드 데이터(DB, 로그)를 삭제하시겠습니까? 지갑은 유지됩니다. (y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "취소되었습니다."
    exit 0
fi

echo ""

# 노드 중지
echo "1. 노드 중지 중..."
./scripts/stop_all_nodes.sh
echo ""

# DB 삭제
echo "2. DB 삭제 중..."
for db in ./resource/db*; do
    if [ -d "$db" ]; then
        rm -rf "$db"
        echo "  ✓ $db 삭제"
    fi
done
echo ""

# 로그 삭제
echo "3. 로그 삭제 중..."
for log in ./log/syslogs*; do
    if [ -d "$log" ]; then
        rm -rf "$log"/*
        echo "  ✓ $log 정리"
    fi
done

rm -f /tmp/abcfed_node*.log 2>/dev/null || true
echo "  ✓ 임시 로그 삭제"
echo ""

# PID 파일 삭제
rm -f /tmp/abcfed_node*.pid 2>/dev/null || true

echo -e "\033[0;32m✓ 정리 완료![0m"
echo ""
echo "지갑은 유지되었습니다:"
ls -d ./resource/wallet* 2>/dev/null || echo "  (지갑 없음)"
echo ""

