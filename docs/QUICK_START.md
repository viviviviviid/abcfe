# ABCFe 빠른 시작 가이드

> **마지막 업데이트: 2026-01-01**

## 단일 노드 (1분 시작)

```bash
# 1. 빌드
make build

# 2. 단일 노드 시작 (지갑 + 설정 자동 생성)
./start_single.sh

# 또는 백그라운드 실행
./start_single.sh -d
```

## 멀티 노드 (PoA 컨센서스)

```bash
# 4개 노드로 PoA 네트워크 시작 (3f+1, f=1)
./start_poa.sh 4

# 상태 확인
./check_nodes.sh
```

## 상태 확인

```bash
# 노드 상태
curl http://localhost:8000/api/v1/status

# 블록 목록
curl http://localhost:8000/api/v1/blocks

# 컨센서스 상태
curl http://localhost:8000/api/v1/consensus/status
```

## WebSocket 연결

```bash
# websocat 설치 (macOS)
brew install websocat

# 실시간 이벤트 수신
websocat ws://localhost:8000/ws
```

## 접속 정보

### 단일 노드
- REST API: http://localhost:8000
- Internal API: http://localhost:8800 (localhost만)
- P2P: 30303
- WebSocket: ws://localhost:8000/ws

### 멀티 노드 (4개)
| Node | REST | Internal | P2P |
|------|------|----------|-----|
| 1 (boot) | 8000 | 8800 | 30303 |
| 2 | 8001 | 8801 | 30304 |
| 3 | 8002 | 8802 | 30305 |
| 4 | 8003 | 8803 | 30306 |

## 노드 관리

```bash
# 노드 중지
./stop_all_nodes.sh

# DB/로그 초기화 (지갑 유지)
./clean_all.sh

# PoA 재시작 (DB 유지)
./resume_poa.sh
```

## 관련 문서

- [USER_GUIDE.md](USER_GUIDE.md) - 전체 사용자 가이드
- [TX_GUIDE.md](TX_GUIDE.md) - 트랜잭션 서명 가이드
- [CLAUDE.md](../CLAUDE.md) - 개발자 가이드
