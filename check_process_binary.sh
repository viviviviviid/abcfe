#!/bin/bash

# check_process_binary.sh - 프로세스의 실제 실행 파일 경로 확인

if [ $# -eq 0 ]; then
    echo "사용법: $0 <프로세스명 또는 PID>"
    echo ""
    echo "예시:"
    echo "  $0 abcfed          # 프로세스명으로 검색"
    echo "  $0 12345           # PID로 확인"
    echo "  $0 -a abcfed       # 모든 abcfed 프로세스 확인"
    exit 1
fi

# 색상 정의
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

SHOW_ALL=false
SEARCH_TERM=""

# 옵션 파싱
if [ "$1" = "-a" ] || [ "$1" = "--all" ]; then
    SHOW_ALL=true
    SEARCH_TERM="$2"
else
    SEARCH_TERM="$1"
fi

# 숫자면 PID, 아니면 프로세스명
if [[ "$SEARCH_TERM" =~ ^[0-9]+$ ]]; then
    # PID로 직접 확인
    pid=$SEARCH_TERM
    if ! ps -p $pid > /dev/null 2>&1; then
        echo -e "${RED}✗ PID $pid 프로세스가 존재하지 않습니다${NC}"
        exit 1
    fi
    
    echo -e "${CYAN}=== PID $pid 프로세스 정보 ===${NC}"
    echo ""
    
    # 실행 파일 경로
    if [ -r "/proc/$pid/exe" ]; then
        exe_path=$(readlink -f /proc/$pid/exe 2>/dev/null || echo "N/A")
    else
        exe_path="N/A (권한 없음)"
    fi
    
    # 작업 디렉토리
    if [ -r "/proc/$pid/cwd" ]; then
        cwd=$(readlink -f /proc/$pid/cwd 2>/dev/null || echo "N/A")
    else
        cwd="N/A"
    fi
    
    # 명령어
    cmd=$(ps -p $pid -o args= 2>/dev/null)
    
    # 프로세스 정보
    ps_info=$(ps -p $pid -o pid,ppid,user,%cpu,%mem,etime,cmd 2>/dev/null | tail -1)
    
    echo -e "${GREEN}실행 파일:${NC} ${BLUE}$exe_path${NC}"
    echo -e "${GREEN}작업 디렉토리:${NC} $cwd"
    echo -e "${GREEN}명령어:${NC} $cmd"
    echo ""
    echo -e "${YELLOW}상세 정보:${NC}"
    echo "$ps_info" | awk '{printf "  PID: %s | PPID: %s | User: %s | CPU: %s | Mem: %s | Time: %s\n  CMD: %s\n", $1, $2, $3, $4, $5, $6, substr($0, index($0,$7))}'
    
else
    # 프로세스명으로 검색
    if [ "$SHOW_ALL" = true ]; then
        pids=$(pgrep -f "$SEARCH_TERM" 2>/dev/null)
    else
        pids=$(pgrep "$SEARCH_TERM" 2>/dev/null)
    fi
    
    if [ -z "$pids" ]; then
        echo -e "${RED}✗ '$SEARCH_TERM' 프로세스를 찾을 수 없습니다${NC}"
        exit 1
    fi
    
    echo -e "${CYAN}=== '$SEARCH_TERM' 프로세스 정보 ===${NC}"
    echo ""
    
    for pid in $pids; do
        echo -e "${YELLOW}[PID: $pid]${NC}"
        
        # 실행 파일 경로
        if [ -r "/proc/$pid/exe" ]; then
            exe_path=$(readlink -f /proc/$pid/exe 2>/dev/null || echo "N/A")
        else
            exe_path="N/A (권한 없음)"
        fi
        
        # 작업 디렉토리
        if [ -r "/proc/$pid/cwd" ]; then
            cwd=$(readlink -f /proc/$pid/cwd 2>/dev/null || echo "N/A")
        else
            cwd="N/A"
        fi
        
        # 명령어
        cmd=$(ps -p $pid -o args= 2>/dev/null | head -c 100)
        
        echo -e "  ${GREEN}실행 파일:${NC} ${BLUE}$exe_path${NC}"
        echo -e "  ${GREEN}작업 디렉토리:${NC} $cwd"
        echo -e "  ${GREEN}명령어:${NC} ${cmd}..."
        echo ""
    done
fi


