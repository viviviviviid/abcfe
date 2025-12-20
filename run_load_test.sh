#!/bin/bash

# run_load_test.sh - ABCFe 부하 테스트 스크립트
# 사용법: ./run_load_test.sh [지갑 수] [옵션]
#
# 예시:
#   ./run_load_test.sh 10           # 10개 지갑으로 테스트
#   ./run_load_test.sh 50 -v        # 50개 지갑, verbose 모드
#   ./run_load_test.sh 100 -c 10    # 100개 지갑, 동시성 10

set -e

# 색상 정의
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[1;36m'
NC='\033[0m'

# 기본값
WALLET_COUNT=${1:-10}
FUND_AMOUNT=1000
SEND_AMOUNT=100
FEE=1
CONCURRENCY=5
VERBOSE=""
NODE_COUNT=3
SKIP_SETUP=false
BASE_URL="http://localhost:8000/api/v1"
WALLET_PATH="./resource/wallet"

# 옵션 파싱
shift || true
while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--verbose)
            VERBOSE="-verbose"
            shift
            ;;
        -c|--concurrency)
            CONCURRENCY="$2"
            shift 2
            ;;
        -f|--fund)
            FUND_AMOUNT="$2"
            shift 2
            ;;
        -s|--send)
            SEND_AMOUNT="$2"
            shift 2
            ;;
        -n|--nodes)
            NODE_COUNT="$2"
            shift 2
            ;;
        --skip-setup)
            SKIP_SETUP=true
            shift
            ;;
        -u|--url)
            BASE_URL="$2"
            shift 2
            ;;
        -w|--wallet)
            WALLET_PATH="$2"
            shift 2
            ;;
        -h|--help)
            echo "사용법: ./run_load_test.sh [지갑 수] [옵션]"
            echo ""
            echo "옵션:"
            echo "  -v, --verbose       상세 출력"
            echo "  -c, --concurrency   동시 전송 수 (기본: 5)"
            echo "  -f, --fund          지갑당 펀딩 금액 (기본: 1000)"
            echo "  -s, --send          지갑당 전송 금액 (기본: 100)"
            echo "  -n, --nodes         PoA 노드 수 (기본: 3)"
            echo "  --skip-setup        노드 재시작 건너뛰기"
            echo "  -u, --url           API 베이스 URL"
            echo "  -w, --wallet        서버 지갑 경로 (기본: ./resource/wallet)"
            echo "  -h, --help          도움말 표시"
            echo ""
            echo "예시:"
            echo "  ./run_load_test.sh 10           # 10개 지갑"
            echo "  ./run_load_test.sh 50 -v -c 10  # 50개 지갑, verbose, 동시성 10"
            echo "  ./run_load_test.sh 100 --skip-setup  # 노드 재시작 없이 테스트"
            exit 0
            ;;
        *)
            shift
            ;;
    esac
done

echo ""
echo -e "${CYAN}╔══════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║         ABCFe 부하 테스트 스크립트            ║${NC}"
echo -e "${CYAN}╚══════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${YELLOW}설정:${NC}"
echo "  지갑 수:        $WALLET_COUNT"
echo "  펀딩 금액:      $FUND_AMOUNT"
echo "  전송 금액:      $SEND_AMOUNT"
echo "  수수료:         $FEE"
echo "  동시성:         $CONCURRENCY"
echo "  PoA 노드 수:    $NODE_COUNT"
echo "  API URL:        $BASE_URL"
echo "  서버 지갑:      $WALLET_PATH"
echo ""

# =============================================================================
# 1. 환경 준비
# =============================================================================
if [ "$SKIP_SETUP" = false ]; then
    echo -e "${YELLOW}[1/4] 환경 준비${NC}"

    # 바이너리 확인/빌드
    if [ ! -f "./abcfed" ]; then
        echo "  빌드 중..."
        make build
    fi
    echo -e "${GREEN}  ✓ 바이너리 준비 완료${NC}"

    # 기존 노드 정리 및 재시작
    echo "  노드 정리 중..."
    pkill -f abcfed 2>/dev/null || true
    sleep 2

    echo "  PoA 노드 시작 중..."
    ./start_poa.sh $NODE_COUNT

    echo "  네트워크 안정화 대기 (15초)..."
    sleep 15
    echo -e "${GREEN}  ✓ 환경 준비 완료${NC}"
else
    echo -e "${YELLOW}[1/4] 환경 준비 (건너뜀)${NC}"
fi

# =============================================================================
# 2. 노드 상태 확인
# =============================================================================
echo ""
echo -e "${YELLOW}[2/4] 노드 상태 확인${NC}"

# API 응답 확인
STATUS=$(curl -s ${BASE_URL}/status 2>/dev/null)
if [ -z "$STATUS" ]; then
    echo -e "${RED}  ✗ API 서버 응답 없음${NC}"
    echo "  노드가 실행 중인지 확인하세요."
    exit 1
fi

HEIGHT=$(echo "$STATUS" | python3 -c "import sys, json; print(json.load(sys.stdin).get('data', {}).get('currentHeight', 'N/A'))" 2>/dev/null || echo "N/A")
echo -e "${GREEN}  ✓ API 서버 응답 정상 (현재 높이: $HEIGHT)${NC}"

# =============================================================================
# 3. 부하 테스트 실행
# =============================================================================
echo ""
echo -e "${YELLOW}[3/4] 부하 테스트 실행${NC}"
echo ""

go run cmd/load_test/main.go \
    -wallets=$WALLET_COUNT \
    -fund=$FUND_AMOUNT \
    -send=$SEND_AMOUNT \
    -fee=$FEE \
    -concurrency=$CONCURRENCY \
    -url=$BASE_URL \
    -wallet=$WALLET_PATH \
    $VERBOSE

# =============================================================================
# 4. 결과 요약
# =============================================================================
echo ""
echo -e "${YELLOW}[4/4] 최종 상태 확인${NC}"

# 블록 높이 확인
FINAL_STATUS=$(curl -s ${BASE_URL}/status 2>/dev/null)
FINAL_HEIGHT=$(echo "$FINAL_STATUS" | python3 -c "import sys, json; print(json.load(sys.stdin).get('data', {}).get('currentHeight', 'N/A'))" 2>/dev/null || echo "N/A")

echo "  시작 높이: $HEIGHT"
echo "  종료 높이: $FINAL_HEIGHT"
echo ""

# 멤풀 상태
MEMPOOL=$(curl -s ${BASE_URL}/mempool 2>/dev/null)
if [ -n "$MEMPOOL" ]; then
    MEMPOOL_COUNT=$(echo "$MEMPOOL" | python3 -c "import sys, json; print(json.load(sys.stdin).get('data', {}).get('count', 0))" 2>/dev/null || echo "0")
    echo "  멤풀 트랜잭션: $MEMPOOL_COUNT"
fi

echo ""
echo -e "${GREEN}✓ 부하 테스트 완료!${NC}"
echo ""
echo -e "${CYAN}=== 유용한 명령어 ===${NC}"
echo "  노드 중지: pkill -f abcfed"
echo "  로그 확인: tail -f /tmp/poa_node1.log"
echo ""
