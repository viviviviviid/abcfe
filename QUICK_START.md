# 🚀 ABCFe 멀티 노드 빠른 시작 가이드

## ⚡ 1분 시작 (자동)

```bash
# 1. 빌드
make build

# 2. 전체 자동 셋업 (3개 노드)
./setup_multi_nodes.sh 3

# 끝! 🎉
```

---

## 📋 수동 시작 (단계별)

### 1단계: 기존 노드 중지
```bash
./stop_all_nodes.sh
```

### 2단계: 지갑 생성 (3개 노드용)
```bash
./create_wallets.sh 3
```

### 3단계: 제네시스 블록 셋업
```bash
./setup_genesis.sh 3
```

### 4단계: 노드 실행
```bash
./start_multi_nodes.sh 3
```

### 5단계: 상태 확인
```bash
./check_nodes.sh
```

---

## 🔄 일상적인 작업

### 노드 재시작
```bash
./stop_all_nodes.sh
./start_multi_nodes.sh 3
```

### 상태 확인
```bash
./check_nodes.sh
```

### 로그 확인
```bash
tail -f /tmp/abcfed_node1.log
tail -f /tmp/abcfed_node2.log
```

### 완전 초기화
```bash
./clean_all.sh
./setup_multi_nodes.sh 3
```

---

## 📊 접속 정보

### REST API
- **Node 1**: http://localhost:8000/api/v1/status
- **Node 2**: http://localhost:8001/api/v1/status
- **Node 3**: http://localhost:8002/api/v1/status

### WebSocket
- **Node 1**: ws://localhost:8000/ws
- **Node 2**: ws://localhost:8001/ws
- **Node 3**: ws://localhost:8002/ws

### P2P 포트
- **Node 1** (Boot): 30303
- **Node 2**: 30304
- **Node 3**: 30305

---

## 🎯 주요 스크립트

| 스크립트 | 설명 |
|---------|------|
| `setup_multi_nodes.sh` | 🚀 **전체 자동 셋업** (추천!) |
| `create_wallets.sh` | 💰 지갑 생성 |
| `setup_genesis.sh` | 🌱 제네시스 블록 셋업 |
| `start_multi_nodes.sh` | ▶️ 노드 시작 |
| `stop_all_nodes.sh` | ⏹️ 노드 중지 |
| `check_nodes.sh` | 📊 상태 확인 |
| `clean_all.sh` | 🧹 데이터 정리 |

---

## 💡 팁

1. **처음 시작**: `setup_multi_nodes.sh` 사용
2. **재시작**: `stop_all_nodes.sh` → `start_multi_nodes.sh`
3. **문제 발생**: `clean_all.sh` → `setup_multi_nodes.sh`
4. **로그 확인**: `/tmp/abcfed_node*.log` 또는 `./log/syslogs*/`

---

## 📚 상세 문서

- **`README_SCRIPTS.md`** - 스크립트 상세 가이드
- **`README_MULTINODE.md`** - 멀티 노드 상세 설명
- **`USER_GUIDE.md`** - 전체 사용자 가이드

---

## ⚠️ 주의사항

1. **지갑 백업**: `./resource/wallet*/` 디렉토리를 주기적으로 백업하세요
2. **제네시스 블록**: 모든 노드가 동일한 제네시스 블록을 가져야 합니다
3. **포트 충돌**: 8000-800X, 30303-3030X 포트가 사용 가능해야 합니다

---

## 🆘 문제 해결

### "포트가 이미 사용 중"
```bash
./stop_all_nodes.sh
# 또는
pkill -9 abcfed
```

### "동기화가 안됨"
```bash
./setup_genesis.sh 3
./stop_all_nodes.sh
./start_multi_nodes.sh 3
```

### "노드가 시작 안됨"
```bash
# 로그 확인
tail -f /tmp/abcfed_node1.log

# DB 초기화
./clean_all.sh
./setup_multi_nodes.sh 3
```

---

**더 자세한 내용은 `README_SCRIPTS.md`를 참고하세요!** 📖

