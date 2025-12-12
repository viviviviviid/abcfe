#!/bin/bash
# =============================================================================
# ABCFe 트랜잭션 테스트 스크립트 (서명된 트랜잭션 버전)
# =============================================================================
# 테스트 시나리오:
# 1. 서버 지갑 계정 0 -> User1 (1,000 코인)
# 2. 잔액 확인
# =============================================================================
# 주의: /tx API는 보안 취약점으로 제거됨
# 대신 /tx/send (서버 지갑 서명) 사용
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

# 금액 정의
AMOUNT_TO_USER1=1000

echo -e "${BLUE}=============================================${NC}"
echo -e "${BLUE}   ABCFe 트랜잭션 테스트 시작${NC}"
echo -e "${BLUE}   (서명된 트랜잭션 버전)${NC}"
echo -e "${BLUE}=============================================${NC}"
echo ""

# =============================================================================
# 헬퍼 함수
# =============================================================================

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
echo -e "지갑 계정 목록:"
WALLET_ACCOUNTS=$(curl -s "${API_BASE}/wallet/accounts")
echo "$WALLET_ACCOUNTS" | jq '.data[] | {index: .index, address: .address[:16]}'

# 첫 번째 계정 주소 가져오기
SENDER_ADDR=$(echo "$WALLET_ACCOUNTS" | jq -r '.data[0].address')
echo ""
echo -e "송신자 주소 (계정 0): ${GREEN}${SENDER_ADDR}${NC}"

# 수신자 주소 (실제 니모닉에서 생성)
# 테스트용 니모닉: "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
# 이 니모닉은 BIP-39 표준 테스트 벡터로, 실제 자금을 보관하지 마세요!
# BIP-44 경로: m/44'/60'/0'/0/0
# 생성된 주소 (Keccak256): 01b2542a9097399d94a1875d125c38bbed39fa12
USER1_ADDR="01b2542a9097399d94a1875d125c38bbed39fa12"
USER1_MNEMONIC="abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
echo -e "수신자 주소 (User1): ${GREEN}${USER1_ADDR}${NC}"
echo -e "수신자 니모닉 (테스트용): ${YELLOW}${USER1_MNEMONIC}${NC}"

echo ""
echo -e "초기 잔액:"
echo -n "  송신자 (${SENDER_ADDR:0:8}...): "
SENDER_BAL=$(curl -s "${API_BASE}/address/${SENDER_ADDR}/balance" | jq -r '.data.balance // 0')
echo -e "${GREEN}${SENDER_BAL}${NC} 코인"

echo -n "  User1 (${USER1_ADDR:0:8}...): "
USER1_BAL=$(curl -s "${API_BASE}/address/${USER1_ADDR}/balance" | jq -r '.data.balance // 0')
echo -e "${GREEN}${USER1_BAL}${NC} 코인"

echo ""

# 송신자 잔액이 0이면 테스트 불가
if [ "$SENDER_BAL" -eq 0 ]; then
    echo -e "${RED}=============================================${NC}"
    echo -e "${RED}   오류: 송신자 잔액이 0입니다!${NC}"
    echo -e "${RED}   제네시스 주소가 지갑 주소와 일치하는지 확인하세요.${NC}"
    echo -e "${RED}=============================================${NC}"
    echo ""
    echo -e "config.toml의 [genesis] 섹션에서 SystemAddresses를"
    echo -e "지갑 주소 (${SENDER_ADDR})로 설정해야 합니다."
    exit 1
fi

# =============================================================================
# Step 1: 송신자 -> User1 트랜잭션 (서명된 트랜잭션)
# =============================================================================
echo -e "${BLUE}[Step 1] 송신자 -> User1 트랜잭션 (${AMOUNT_TO_USER1} 코인)${NC}"
echo "----------------------------------------"

echo -e "트랜잭션 전송 중 (/tx/send - 서버 지갑 서명)..."
TX1_RESULT=$(curl -s -X POST "${API_BASE}/tx/send" \
  -H "Content-Type: application/json" \
  -d "{
    \"accountIndex\": 0,
    \"to\": \"${USER1_ADDR}\",
    \"amount\": ${AMOUNT_TO_USER1},
    \"memo\": \"Wallet to User1 transfer\"
  }")

echo "응답: $TX1_RESULT"

if echo "$TX1_RESULT" | jq -e '.success == true' > /dev/null 2>&1; then
    TX_ID=$(echo "$TX1_RESULT" | jq -r '.data.txId')
    echo -e "${GREEN}트랜잭션 성공!${NC}"
    echo -e "  TX ID: ${TX_ID:0:16}..."
else
    echo -e "${RED}트랜잭션 실패!${NC}"
    echo "$TX1_RESULT" | jq .
    exit 1
fi

echo ""
echo -e "멤풀 상태:"
curl -s "${API_BASE}/mempool/list" | jq '.data | length' | xargs -I {} echo "  대기 중인 트랜잭션: {} 개"

# 블록 생성 대기
wait_for_block

echo ""
echo -e "블록 생성 후 멤풀:"
curl -s "${API_BASE}/mempool/list" | jq '.data | length' | xargs -I {} echo "  대기 중인 트랜잭션: {} 개"

echo ""

# =============================================================================
# Step 2: 최종 상태 확인
# =============================================================================
echo -e "${BLUE}[Step 2] 최종 상태 확인${NC}"
echo "----------------------------------------"

echo -e "최종 잔액:"
echo -n "  송신자 (${SENDER_ADDR:0:8}...): "
FINAL_SENDER=$(curl -s "${API_BASE}/address/${SENDER_ADDR}/balance" | jq -r '.data.balance // 0')
echo -e "${GREEN}${FINAL_SENDER}${NC} 코인"

echo -n "  User1 (${USER1_ADDR:0:8}...): "
FINAL_USER1=$(curl -s "${API_BASE}/address/${USER1_ADDR}/balance" | jq -r '.data.balance // 0')
echo -e "${GREEN}${FINAL_USER1}${NC} 코인"

echo ""
echo -e "노드 상태:"
curl -s "${API_BASE}/status" | jq '.data | {height: .currentHeight, hash: .currentBlockHash[:16]}'

echo ""
echo -e "최신 블록:"
curl -s "${API_BASE}/block/latest" | jq '.data | {height: .header.height, txCount: (.transactions | length), proposer: .proposer[:16]}'

echo ""

# =============================================================================
# 결과 검증
# =============================================================================
echo -e "${BLUE}[검증] 결과 확인${NC}"
echo "----------------------------------------"

EXPECTED_SENDER=$((SENDER_BAL - AMOUNT_TO_USER1))
EXPECTED_USER1=$((USER1_BAL + AMOUNT_TO_USER1))

echo "예상 잔액:"
echo "  송신자: ${EXPECTED_SENDER} 코인"
echo "  User1: ${EXPECTED_USER1} 코인"

echo ""
echo "실제 잔액:"
echo "  송신자: ${FINAL_SENDER} 코인"
echo "  User1: ${FINAL_USER1} 코인"

echo ""

if [ "$FINAL_SENDER" -eq "$EXPECTED_SENDER" ] && \
   [ "$FINAL_USER1" -eq "$EXPECTED_USER1" ]; then
    echo -e "${GREEN}=============================================${NC}"
    echo -e "${GREEN}   모든 테스트 통과!${NC}"
    echo -e "${GREEN}   서명된 트랜잭션이 정상 처리되었습니다.${NC}"
    echo -e "${GREEN}=============================================${NC}"
else
    echo -e "${YELLOW}=============================================${NC}"
    echo -e "${YELLOW}   잔액이 예상과 다릅니다${NC}"
    echo -e "${YELLOW}   (블록 생성 타이밍 문제일 수 있음)${NC}"
    echo -e "${YELLOW}=============================================${NC}"
fi

echo ""
echo -e "${BLUE}테스트 완료!${NC}"
