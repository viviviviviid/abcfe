# 멀티 노드 관리 스크립트 가이드

ABCFe 블록체인 멀티 노드 환경을 쉽게 관리할 수 있는 스크립트 모음입니다.

## 📋 스크립트 목록

### 🚀 자동 셋업 (추천)

#### `setup_multi_nodes.sh` - 전체 자동 셋업
```bash
./setup_multi_nodes.sh [노드개수]

# 예시
./setup_multi_nodes.sh 3  # 3개 노드 자동 셋업
```

**실행 순서:**
1. 기존 노드 중지
2. DB 초기화 (선택적)
3. 지갑 생성
4. 제네시스 블록 셋업
5. 노드 시작
6. 상태 확인

---

### 📦 개별 스크립트

#### `create_wallets.sh` - 지갑 생성
```bash
./create_wallets.sh [노드개수]

# 예시
./create_wallets.sh 3  # 3개 지갑 생성
```
- Node 1: `./resource/wallet/wallet.json`
- Node 2: `./resource/wallet2/wallet.json`
- Node 3: `./resource/wallet3/wallet.json`

#### `setup_genesis.sh` - 제네시스 블록 셋업
```bash
./setup_genesis.sh [노드개수]

# 예시
./setup_genesis.sh 3
```
- Boot 노드(Node 1)에서 제네시스 블록 생성
- 다른 노드들에게 제네시스 블록 복사
- 모든 노드가 동일한 체인에서 시작하도록 보장

#### `start_multi_nodes.sh` - 노드 시작
```bash
./start_multi_nodes.sh [노드개수] [제네시스복사]

# 예시
./start_multi_nodes.sh 3           # 3개 노드 시작
./start_multi_nodes.sh 3 true      # 제네시스 블록 복사 후 시작
```
- Node 1: Boot/Producer (Port 30303, REST 8000)
- Node 2+: Validator/Sync-only (Port 30304+, REST 8001+)

#### `check_nodes.sh` - 상태 확인
```bash
./check_nodes.sh
```
- 실행 중인 노드 프로세스 확인
- **각 프로세스의 실행 파일 경로** 표시 (어떤 바이너리를 사용하는지 확인)
- 각 노드의 블록 높이, 해시, 멤풀 상태
- 동기화 상태 확인

**출력 예시:**
```
실행 중인 노드 프로세스:
  PID: 12345
    실행 파일: /home/user/workspace/abcfe/abcfe-node/abcfed
    작업 디렉토리: /home/user/workspace/abcfe/abcfe-node
    명령어: ./abcfed --config=config/config.toml
```

#### `check_port.sh` - 포트 사용 현황 확인
```bash
./check_port.sh [포트번호] [포트번호2] ...

# 예시
./check_port.sh 8080          # 단일 포트 확인
./check_port.sh 8080 8081 30303  # 여러 포트 확인
```
- 특정 포트를 사용 중인 프로세스의 PID 확인
- **실행 파일 경로** 표시 (어떤 바이너리를 사용하는지 확인)
- 프로세스 이름, 명령어, 상세 정보 표시
- ABCFe 노드 포트 자동 감지 (8080-8084, 30303-30307)

**출력 예시:**
```
포트 8080:
  ✓ 사용 중
  PID: 12345
  프로세스: abcfed
  실행 파일: /home/user/workspace/abcfe/abcfe-node/abcfed
  작업 디렉토리: /home/user/workspace/abcfe/abcfe-node
  명령어: ./abcfed --config=config/config.toml
```

#### `check_process_binary.sh` - 프로세스 실행 파일 확인
```bash
./check_process_binary.sh <프로세스명 또는 PID>

# 예시
./check_process_binary.sh abcfed        # 프로세스명으로 검색
./check_process_binary.sh 12345         # PID로 확인
./check_process_binary.sh -a abcfed     # 모든 abcfed 프로세스 확인
```
- 프로세스의 **실제 실행 파일 경로** 확인
- 작업 디렉토리, 명령어, 상세 정보 표시
- 같은 이름의 프로세스가 다른 경로의 바이너리를 사용하는지 확인 가능

**출력 예시:**
```
=== 'abcfed' 프로세스 정보 ===

[PID: 12345]
  실행 파일: /home/user/workspace/abcfe/abcfe-node/abcfed
  작업 디렉토리: /home/user/workspace/abcfe/abcfe-node
  명령어: ./abcfed --config=config/config.toml...
```

#### `stop_all_nodes.sh` - 노드 중지
```bash
./stop_all_nodes.sh
```
- 실행 중인 모든 `abcfed` 프로세스 종료

#### `clean_all.sh` - 데이터 정리
```bash
./clean_all.sh
```
- 모든 DB 삭제
- 로그 파일 정리
- **지갑은 유지됨**

---

## 🎯 주요 사용 시나리오

### 1️⃣ 처음 시작 (자동)
```bash
# 한 번에 모든 셋업
./setup_multi_nodes.sh 3
```

### 2️⃣ 처음 시작 (수동)
```bash
# 단계별 실행
./stop_all_nodes.sh
./create_wallets.sh 3
./setup_genesis.sh 3
./start_multi_nodes.sh 3
./check_nodes.sh
```

### 3️⃣ 노드 재시작
```bash
# 데이터 유지하면서 재시작
./stop_all_nodes.sh
./start_multi_nodes.sh 3
./check_nodes.sh
```

### 4️⃣ 완전 초기화 후 재시작
```bash
# 모든 데이터 삭제 후 재시작
./clean_all.sh
./setup_multi_nodes.sh 3
```

### 5️⃣ 노드 추가
```bash
# 기존: 2개 노드 실행 중
# 추가: 1개 노드 더 실행

# 새 노드 지갑 생성
./abcfed wallet create --wallet-dir=./resource/wallet3

# 새 노드용 설정 파일 생성 (또는 start_multi_nodes.sh가 자동 생성)
# config/config_node3.toml

# 제네시스 블록 복사
./setup_genesis.sh 3

# 모든 노드 재시작
./stop_all_nodes.sh
./start_multi_nodes.sh 3
```

---

## 📊 노드 설정

### Node 1 (Boot/Producer)
- **역할**: 블록 생성, 네트워크 부트스트랩
- **P2P 포트**: 30303
- **REST API**: 8000
- **모드**: `boot`, `blockProducer: true`
- **DB**: `./resource/db/leveldb_3000.db`
- **지갑**: `./resource/wallet/wallet.json`

### Node 2-N (Validator/Sync)
- **역할**: 블록 동기화, 검증
- **P2P 포트**: 30304, 30305, ...
- **REST API**: 8001, 8002, ...
- **모드**: `validator`, `blockProducer: false`
- **DB**: `./resource/db2/`, `./resource/db3/`, ...
- **지갑**: `./resource/wallet2/`, `./resource/wallet3/`, ...
- **Boot 노드**: `127.0.0.1:30303`

---

## 🔍 트러블슈팅

### 노드가 시작되지 않음
```bash
# 로그 확인
tail -f /tmp/abcfed_node1.log
tail -f /tmp/abcfed_node2.log

# 또는
tail -f ./log/syslogs/_2025-12-09.log
tail -f ./log/syslogs2/_2025-12-09.log
```

### 동기화가 안됨
```bash
# 상태 확인
./check_nodes.sh

# 제네시스 블록 재복사
./setup_genesis.sh 3

# 노드 재시작
./stop_all_nodes.sh
./start_multi_nodes.sh 3
```

### 포트 충돌
```bash
# 사용 중인 포트 확인
lsof -i :30303
lsof -i :8000

# 프로세스 강제 종료
./stop_all_nodes.sh
```

### DB 손상
```bash
# 완전 초기화
./clean_all.sh
./setup_multi_nodes.sh 3
```

---

## 💡 팁

1. **로그 레벨 조정**: `config.toml`에서 `level = "debug"` 설정
2. **동기화 확인**: `./check_nodes.sh`를 주기적으로 실행
3. **자동 재시작**: systemd나 supervisor로 프로세스 관리 가능
4. **백업**: 정기적으로 `./resource/wallet*/` 백업 권장

---

## 🔧 트러블슈팅

### 지갑 생성 실패

**증상**: "지갑 생성 중..." 에서 멈춤 또는 시간 초과

**해결 방법**:

```bash
# 1. abcfed 바이너리 확인
ls -lh abcfed

# 2. 빌드 (없거나 오래된 경우)
make build

# 3. 실행 권한 부여
chmod +x abcfed

# 4. 수동으로 지갑 생성 테스트
./abcfed wallet create --wallet-dir="./resource/wallet"
```

### 노드가 시작되지 않음

**증상**: PID 파일 관련 오류 또는 포트 충돌

**해결 방법**:

```bash
# 1. 기존 프로세스 확인
ps aux | grep abcfed

# 2. 강제 종료
pkill -9 abcfed

# 3. PID 파일 삭제
rm -f ~/.abcfe-node/*.pid

# 4. 포트 사용 확인
./check_port.sh 8080 8081 30303  # 또는
lsof -i :8080  # REST API 포트
lsof -i :30303 # P2P 포트
lsof -ti :8080  # PID만 추출

# 5. 재시작
./start_multi_nodes.sh 3
```

### 블록 동기화 안됨

**증상**: Node 2-5의 블록 높이가 0으로 유지됨

**해결 방법**:

```bash
# 1. 로그 확인
tail -f log/syslogs/_*.log
tail -f log/syslogs2/_*.log

# 2. P2P 연결 확인
curl http://localhost:8080/api/v1/status
curl http://localhost:8081/api/v1/status

# 3. 제네시스 블록 재설정
./stop_all_nodes.sh
rm -rf resource/db*
./setup_genesis.sh 5
./start_multi_nodes.sh 5
```

### DB 손상

**증상**: "database is locked" 또는 "invalid db format"

**해결 방법**:

```bash
# 전체 초기화 (경고: 데이터 손실!)
./stop_all_nodes.sh
./clean_all.sh
echo "y" | ./setup_multi_nodes.sh 5
```

### 원격 서버에서 스크립트 멈춤

**증상**: 대화형 입력 대기로 인한 멈춤

**해결 방법**:

```bash
# 비대화형 스크립트 사용
./setup_multi_nodes_noninteractive.sh 5

# 또는 자동 응답
echo "y" | ./setup_multi_nodes.sh 5
```

---

## 📝 관련 파일

- `config/config.toml` - Node 1 설정
- `config/config_node2.toml` - Node 2 설정
- `config/config_node3.toml` - Node 3 설정
- `USER_GUIDE.md` - 전체 사용자 가이드
- `QUICK_START.md` - 빠른 시작 가이드

---

## 🆘 도움말

더 자세한 정보는 다음 문서를 참고하세요:
- `USER_GUIDE.md` - 전체 기능 가이드
- `QUICK_START.md` - 빠른 시작 가이드
- `CLAUDE.md` - 개발자 가이드

