#!/bin/bash

# check_port.sh - 특정 포트를 사용 중인 프로세스 확인

if [ $# -eq 0 ]; then
    echo "사용법: $0 <포트번호> [포트번호2] ..."
    echo ""
    echo "예시:"
    echo "  $0 8080"
    echo "  $0 8080 8081 30303"
    exit 1
fi

# 색상 정의
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== 포트 사용 현황 ===${NC}"
echo ""

for port in "$@"; do
    echo -e "${YELLOW}포트 $port:${NC}"
    
    # lsof 사용 (가장 정확)
    if command -v lsof >/dev/null 2>&1; then
        pid=$(lsof -ti :$port 2>/dev/null)
        if [ -n "$pid" ]; then
            process=$(ps -p $pid -o comm= 2>/dev/null)
            cmdline=$(ps -p $pid -o args= 2>/dev/null | head -c 80)
            
            # 실제 실행 파일 경로 확인
            if [ -r "/proc/$pid/exe" ]; then
                exe_path=$(readlink -f /proc/$pid/exe 2>/dev/null || echo "N/A")
            else
                exe_path="N/A (권한 없음)"
            fi
            
            # 작업 디렉토리 확인
            if [ -r "/proc/$pid/cwd" ]; then
                cwd=$(readlink -f /proc/$pid/cwd 2>/dev/null || echo "N/A")
            else
                cwd="N/A"
            fi
            
            echo -e "  ${GREEN}✓ 사용 중${NC}"
            echo -e "  PID: $pid"
            echo -e "  프로세스: $process"
            echo -e "  ${BLUE}실행 파일: $exe_path${NC}"
            echo -e "  작업 디렉토리: $cwd"
            echo -e "  명령어: ${cmdline}..."
            
            # 프로세스 상세 정보
            if [ -n "$pid" ]; then
                echo -e "  상세:"
                ps -p $pid -o pid,ppid,user,%cpu,%mem,etime,cmd 2>/dev/null | tail -1 | sed 's/^/    /'
            fi
        else
            echo -e "  ${RED}✗ 사용 중이 아님${NC}"
        fi
    # netstat 사용 (fallback)
    elif command -v netstat >/dev/null 2>&1; then
        result=$(netstat -tulpn 2>/dev/null | grep ":$port ")
        if [ -n "$result" ]; then
            echo -e "  ${GREEN}✓ 사용 중${NC}"
            echo "$result" | sed 's/^/  /'
        else
            echo -e "  ${RED}✗ 사용 중이 아님${NC}"
        fi
    # ss 사용 (fallback)
    elif command -v ss >/dev/null 2>&1; then
        result=$(ss -tulpn 2>/dev/null | grep ":$port ")
        if [ -n "$result" ]; then
            echo -e "  ${GREEN}✓ 사용 중${NC}"
            echo "$result" | sed 's/^/  /'
        else
            echo -e "  ${RED}✗ 사용 중이 아님${NC}"
        fi
    else
        echo -e "  ${RED}✗ lsof, netstat, ss 명령어를 찾을 수 없습니다${NC}"
    fi
    echo ""
done

# 모든 ABCFe 노드 포트 확인 (자동 감지)
echo -e "${BLUE}=== ABCFe 노드 포트 자동 확인 ===${NC}"
echo ""

# 기본 포트 범위
rest_api_ports=(8080 8081 8082 8083 8084)
p2p_ports=(30303 30304 30305 30306 30307)
ws_ports=(8080 8081 8082 8083 8084)  # WebSocket은 REST API와 같은 포트

echo -e "${YELLOW}REST API 포트:${NC}"
for port in "${rest_api_ports[@]}"; do
    pid=$(lsof -ti :$port 2>/dev/null)
    if [ -n "$pid" ]; then
        node_num=$((port - 8079))
        
        # 실제 실행 파일 경로 확인
        if [ -r "/proc/$pid/exe" ]; then
            exe_path=$(readlink -f /proc/$pid/exe 2>/dev/null || echo "N/A")
        else
            exe_path="N/A (권한 없음)"
        fi
        
        echo -e "  ${GREEN}✓ 포트 $port (Node $node_num)${NC} - PID: $pid"
        echo -e "    실행 파일: ${BLUE}$exe_path${NC}"
    fi
done

echo ""
echo -e "${YELLOW}P2P 포트:${NC}"
for port in "${p2p_ports[@]}"; do
    pid=$(lsof -ti :$port 2>/dev/null)
    if [ -n "$pid" ]; then
        node_num=$((port - 30302))
        
        # 실제 실행 파일 경로 확인
        if [ -r "/proc/$pid/exe" ]; then
            exe_path=$(readlink -f /proc/$pid/exe 2>/dev/null || echo "N/A")
        else
            exe_path="N/A (권한 없음)"
        fi
        
        echo -e "  ${GREEN}✓ 포트 $port (Node $node_num)${NC} - PID: $pid"
        echo -e "    실행 파일: ${BLUE}$exe_path${NC}"
    fi
done

