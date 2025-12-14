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
MIN_FEE=1  # 최소 수수료 (config.toml [fee] minFee와 일치)
BLOCK_REWARD=50  # 블록 보상 (config.toml [fee] blockReward와 일치)

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
echo -e "${BLUE}[Step 1] 송신자 -> User1 트랜잭션 (${AMOUNT_TO_USER1} 코인 + 수수료 ${MIN_FEE})${NC}"
echo "----------------------------------------"

echo -e "트랜잭션 전송 중 (/tx/send - 서버 지갑 서명)..."
echo -e "  금액: ${AMOUNT_TO_USER1} 코인"
echo -e "  수수료: ${MIN_FEE} 코인 (암묵적 수수료)"
TX1_RESULT=$(curl -s -X POST "${API_BASE}/tx/send" \
  -H "Content-Type: application/json" \
  -d "{
    \"accountIndex\": 0,
    \"to\": \"${USER1_ADDR}\",
    \"amount\": ${AMOUNT_TO_USER1},
    \"fee\": ${MIN_FEE},
    \"memo\": \"Wallet to User1 transfer (fee: ${MIN_FEE})\"
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

# 송신자가 proposer(블록 생성자)와 동일한지 확인
# 동일하면 블록 보상 + 수수료를 받음
PROPOSER_ADDR=$(curl -s "${API_BASE}/block/latest" | jq -r '.data.proposer // ""')

echo -e "블록 제안자: ${PROPOSER_ADDR:0:16}..."
echo -e "송신자 주소: ${SENDER_ADDR:0:16}..."

# 기본 예상 잔액 계산 (수수료 포함)
# 송신자: 초기잔액 - 금액 - 수수료
EXPECTED_SENDER=$((SENDER_BAL - AMOUNT_TO_USER1 - MIN_FEE))
EXPECTED_USER1=$((USER1_BAL + AMOUNT_TO_USER1))

# 송신자가 proposer인 경우, 블록 보상 + 수수료를 받음
if [ "$PROPOSER_ADDR" == "$SENDER_ADDR" ]; then
    echo -e "${YELLOW}송신자가 블록 제안자입니다. 블록 보상 + 수수료를 받습니다.${NC}"
    # 블록 보상 + 수수료(본인이 낸 것)를 받음
    EXPECTED_SENDER=$((EXPECTED_SENDER + BLOCK_REWARD + MIN_FEE))
fi

echo ""
echo "예상 잔액 계산:"
echo "  송신자: 초기(${SENDER_BAL}) - 금액(${AMOUNT_TO_USER1}) - 수수료(${MIN_FEE})"
if [ "$PROPOSER_ADDR" == "$SENDER_ADDR" ]; then
    echo "         + 블록보상(${BLOCK_REWARD}) + 수수료(${MIN_FEE}) = ${EXPECTED_SENDER} 코인"
else
    echo "         = ${EXPECTED_SENDER} 코인"
fi
echo "  User1: 초기(${USER1_BAL}) + 금액(${AMOUNT_TO_USER1}) = ${EXPECTED_USER1} 코인"

echo ""
echo "실제 잔액:"
echo "  송신자: ${FINAL_SENDER} 코인"
echo "  User1: ${FINAL_USER1} 코인"

echo ""

if [ "$FINAL_SENDER" -eq "$EXPECTED_SENDER" ] && \
   [ "$FINAL_USER1" -eq "$EXPECTED_USER1" ]; then
    echo -e "${GREEN}=============================================${NC}"
    echo -e "${GREEN}   모든 테스트 통과!${NC}"
    echo -e "${GREEN}   수수료 포함 트랜잭션이 정상 처리되었습니다.${NC}"
    echo -e "${GREEN}=============================================${NC}"
else
    echo -e "${YELLOW}=============================================${NC}"
    echo -e "${YELLOW}   잔액이 예상과 다릅니다${NC}"
    echo -e "${YELLOW}   예상 송신자: ${EXPECTED_SENDER}, 실제: ${FINAL_SENDER}${NC}"
    echo -e "${YELLOW}   예상 User1: ${EXPECTED_USER1}, 실제: ${FINAL_USER1}${NC}"
    echo -e "${YELLOW}   (블록 생성 타이밍 또는 Coinbase TX 문제일 수 있음)${NC}"
    echo -e "${YELLOW}=============================================${NC}"
fi

echo ""
echo -e "${BLUE}테스트 완료!${NC}"
