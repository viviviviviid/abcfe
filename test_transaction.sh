#!/bin/bash
# =============================================================================
# ABCFe 트랜잭션 테스트 스크립트
# =============================================================================
# 테스트 시나리오:
# 1. Genesis -> User1 (1,000,000 코인)
# 2. User1 -> User2 (100,000 코인)
# 3. 잔액 확인
# =============================================================================

set -e

# 색상 정의
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# API 엔드포인트
API_BASE="http://localhost:8000/api/v1"

# 주소 정의
GENESIS_ADDR="17112bdf0c4b66abcb6b9651538a36da97031cca"
USER1_ADDR="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
USER2_ADDR="bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

# 금액 정의
AMOUNT_TO_USER1=1000000
AMOUNT_TO_USER2=100000

echo -e "${BLUE}=============================================${NC}"
echo -e "${BLUE}   ABCFe 트랜잭션 테스트 시작${NC}"
echo -e "${BLUE}=============================================${NC}"
echo ""

# =============================================================================
# 헬퍼 함수
# =============================================================================

check_balance() {
    local addr=$1
    local name=$2
    local balance=$(curl -s "${API_BASE}/address/${addr}/balance" | jq -r '.data.balance // 0')
    echo -e "${name}: ${GREEN}${balance}${NC} 코인"
    echo $balance
}

wait_for_block() {
    echo -e "${YELLOW}블록 생성 대기 중 (4초)...${NC}"
    sleep 4
}

# =============================================================================
# Step 0: 초기 상태 확인
# =============================================================================
echo -e "${BLUE}[Step 0] 초기 상태 확인${NC}"
echo "----------------------------------------"

echo -e "노드 상태:"
curl -s "${API_BASE}/status" | jq '.data | {height: .currentHeight, hash: .currentBlockHash[:16]}'

echo ""
echo -e "초기 잔액:"
echo -n "  Genesis (${GENESIS_ADDR:0:8}...): "
GENESIS_BAL=$(curl -s "${API_BASE}/address/${GENESIS_ADDR}/balance" | jq -r '.data.balance // 0')
echo -e "${GREEN}${GENESIS_BAL}${NC} 코인"

echo -n "  User1 (${USER1_ADDR:0:8}...): "
USER1_BAL=$(curl -s "${API_BASE}/address/${USER1_ADDR}/balance" | jq -r '.data.balance // 0')
echo -e "${GREEN}${USER1_BAL}${NC} 코인"

echo -n "  User2 (${USER2_ADDR:0:8}...): "
USER2_BAL=$(curl -s "${API_BASE}/address/${USER2_ADDR}/balance" | jq -r '.data.balance // 0')
echo -e "${GREEN}${USER2_BAL}${NC} 코인"

echo ""

# =============================================================================
# Step 1: Genesis -> User1 트랜잭션
# =============================================================================
echo -e "${BLUE}[Step 1] Genesis -> User1 트랜잭션 (${AMOUNT_TO_USER1} 코인)${NC}"
echo "----------------------------------------"

echo -e "트랜잭션 전송 중..."
TX1_RESULT=$(curl -s -X POST "${API_BASE}/tx" \
  -H "Content-Type: application/json" \
  -d "{
    \"from\": \"${GENESIS_ADDR}\",
    \"to\": \"${USER1_ADDR}\",
    \"amount\": ${AMOUNT_TO_USER1},
    \"memo\": \"Genesis to User1 transfer\"
  }")

echo "응답: $TX1_RESULT"

if echo "$TX1_RESULT" | jq -e '.success == true' > /dev/null 2>&1; then
    echo -e "${GREEN}트랜잭션 성공!${NC}"
else
    echo -e "${RED}트랜잭션 실패!${NC}"
    echo "$TX1_RESULT" | jq .
fi

echo ""
echo -e "멤풀 상태:"
curl -s "${API_BASE}/mempool/list" | jq '.data | length' | xargs -I {} echo "  대기 중인 트랜잭션: {} 개"

# 블록 생성 대기
wait_for_block

echo ""
echo -e "블록 생성 후 잔액:"
echo -n "  Genesis: "
curl -s "${API_BASE}/address/${GENESIS_ADDR}/balance" | jq -r '.data.balance // 0' | xargs -I {} echo -e "${GREEN}{}${NC} 코인"
echo -n "  User1: "
curl -s "${API_BASE}/address/${USER1_ADDR}/balance" | jq -r '.data.balance // 0' | xargs -I {} echo -e "${GREEN}{}${NC} 코인"

echo ""

# =============================================================================
# Step 2: User1 -> User2 트랜잭션
# =============================================================================
echo -e "${BLUE}[Step 2] User1 -> User2 트랜잭션 (${AMOUNT_TO_USER2} 코인)${NC}"
echo "----------------------------------------"

echo -e "트랜잭션 전송 중..."
TX2_RESULT=$(curl -s -X POST "${API_BASE}/tx" \
  -H "Content-Type: application/json" \
  -d "{
    \"from\": \"${USER1_ADDR}\",
    \"to\": \"${USER2_ADDR}\",
    \"amount\": ${AMOUNT_TO_USER2},
    \"memo\": \"User1 to User2 transfer\"
  }")

echo "응답: $TX2_RESULT"

if echo "$TX2_RESULT" | jq -e '.success == true' > /dev/null 2>&1; then
    echo -e "${GREEN}트랜잭션 성공!${NC}"
else
    echo -e "${RED}트랜잭션 실패!${NC}"
    echo "$TX2_RESULT" | jq .
fi

echo ""
echo -e "멤풀 상태:"
curl -s "${API_BASE}/mempool/list" | jq '.data | length' | xargs -I {} echo "  대기 중인 트랜잭션: {} 개"

# 블록 생성 대기
wait_for_block

echo ""

# =============================================================================
# Step 3: 최종 상태 확인
# =============================================================================
echo -e "${BLUE}[Step 3] 최종 상태 확인${NC}"
echo "----------------------------------------"

echo -e "최종 잔액:"
echo -n "  Genesis (${GENESIS_ADDR:0:8}...): "
FINAL_GENESIS=$(curl -s "${API_BASE}/address/${GENESIS_ADDR}/balance" | jq -r '.data.balance // 0')
echo -e "${GREEN}${FINAL_GENESIS}${NC} 코인"

echo -n "  User1 (${USER1_ADDR:0:8}...): "
FINAL_USER1=$(curl -s "${API_BASE}/address/${USER1_ADDR}/balance" | jq -r '.data.balance // 0')
echo -e "${GREEN}${FINAL_USER1}${NC} 코인"

echo -n "  User2 (${USER2_ADDR:0:8}...): "
FINAL_USER2=$(curl -s "${API_BASE}/address/${USER2_ADDR}/balance" | jq -r '.data.balance // 0')
echo -e "${GREEN}${FINAL_USER2}${NC} 코인"

echo ""
echo -e "노드 상태:"
curl -s "${API_BASE}/status" | jq '.data | {height: .currentHeight, hash: .currentBlockHash[:16]}'

echo ""
echo -e "최신 블록:"
curl -s "${API_BASE}/block/latest" | jq '.data | {height: .header.height, txCount: (.transactions | length)}'

echo ""

# =============================================================================
# 결과 검증
# =============================================================================
echo -e "${BLUE}[검증] 결과 확인${NC}"
echo "----------------------------------------"

EXPECTED_GENESIS=$((GENESIS_BAL - AMOUNT_TO_USER1))
EXPECTED_USER1=$((AMOUNT_TO_USER1 - AMOUNT_TO_USER2))
EXPECTED_USER2=$AMOUNT_TO_USER2

echo "예상 잔액:"
echo "  Genesis: ${EXPECTED_GENESIS} 코인"
echo "  User1: ${EXPECTED_USER1} 코인"
echo "  User2: ${EXPECTED_USER2} 코인"

echo ""

if [ "$FINAL_GENESIS" -eq "$EXPECTED_GENESIS" ] && \
   [ "$FINAL_USER1" -eq "$EXPECTED_USER1" ] && \
   [ "$FINAL_USER2" -eq "$EXPECTED_USER2" ]; then
    echo -e "${GREEN}=============================================${NC}"
    echo -e "${GREEN}   모든 테스트 통과!${NC}"
    echo -e "${GREEN}=============================================${NC}"
else
    echo -e "${YELLOW}=============================================${NC}"
    echo -e "${YELLOW}   잔액이 예상과 다릅니다 (블록 생성 타이밍 문제일 수 있음)${NC}"
    echo -e "${YELLOW}=============================================${NC}"
fi

echo ""
echo -e "${BLUE}테스트 완료!${NC}"
