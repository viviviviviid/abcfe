#!/bin/bash

# 모든 ABCFe 노드 중지 스크립트

# 프로젝트 루트로 이동 (어디서 실행해도 동작)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT" || exit 1

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}=== 모든 ABCFe 노드 중지 ===${NC}"

# 실행 중인 노드 확인
NODE_COUNT=$(pgrep -f abcfed | wc -l | tr -d ' ')

if [ "$NODE_COUNT" -eq "0" ]; then
    echo "실행 중인 노드가 없습니다."
    exit 0
fi

echo "실행 중인 노드: $NODE_COUNT 개"

# 모든 노드 프로세스 종료
pkill -f abcfed

# 프로세스가 완전히 종료될 때까지 대기
sleep 2

# 남아있는 프로세스 확인
REMAINING=$(pgrep -f abcfed | wc -l | tr -d ' ')

if [ "$REMAINING" -eq "0" ]; then
    echo -e "${GREEN}✓ 모든 노드가 중지되었습니다.${NC}"
else
    echo -e "${YELLOW}경고: 일부 프로세스가 아직 실행 중입니다. 강제 종료합니다...${NC}"
    pkill -9 -f abcfed
    sleep 1
    echo -e "${GREEN}✓ 모든 노드가 중지되었습니다.${NC}"
fi

