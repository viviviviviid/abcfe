# Gemini PoA Analysis Report

## 1. Current Status

The node network is successfully peering and syncing blocks. However, the consensus mechanism is not functioning correctly. The first node to start produces blocks, but these blocks are not being validated by a set of authorities, and there is no proper proposer rotation. The system is currently operating in a "solo mode" where one node acts as a block producer without any real consensus.

## 2. Core Problems

Our analysis has identified two critical architectural flaws that prevent the Proof-of-Authority (PoA) mechanism from working as intended.

### Flaw 1: Broken Validator Registration (Root Cause)

The system is designed to create a set of validators from nodes that have staked a sufficient amount. However, the process of linking a staker to a verifiable identity (their public key) is broken.

-   **Missing Public Key in Staker:** The `Staker` struct in `consensus/staking.go` does not have a field to store the public key of the account that is staking.
-   **Nil Public Key in Validator:** Because the public key is missing from the staker information, when the `ValidatorSet` is created in `consensus/validator.go`, the `PublicKey` for each validator is explicitly set to `nil`.
-   **Fallback to Solo Mode:** The consensus engine in `consensus/engine.go`, upon finding no active validators with valid public keys, reverts to a `produceBlockSolo` function. This function bypasses all consensus checks and simply produces blocks, which explains the behavior you are observing.

### Flaw 2: Missing Proposer and Signature Validation

There is a second, equally critical flaw in the block validation logic itself. Even if the validator set were populated correctly, the chain would not be secure.

-   **No Proposer Check:** The `ValidateBlock` function in `core/validate.go` performs several important checks (e.g., block hash, transaction integrity). However, it **completely fails to verify that the block was created by the correct proposer** as determined by the round-robin selection algorithm in `consensus/selection.go`.
-   **No Signature Verification:** The validation logic also does not verify the block's signature against the alleged proposer's public key. This check is essential to ensure a block was actually created by the authority who claims to have created it.

## 3. Path to a Functional PoA System

To fix these issues and implement a working PoA consensus mechanism, the following changes are required.

### Step 1: Fix Validator Registration

1.  **Update `Staker` Struct:** Modify the `Staker` struct in `consensus/staking.go` to include a `PublicKey []byte` field.
2.  **Update `Stake` Function:** Modify the `Stake` method in `consensus/consensus.go`. It must now accept the staker's public key as an argument and store it in the new field when creating/updating a `Staker` object.
3.  **Correctly Populate Validator:** In `consensus/validator.go`, fix the `UpdateFromStakerSet` function to copy the `PublicKey` from the `Staker` object to the `Validator` object, removing the `TODO` and the `nil` assignment.

### Step 2: Implement Proposer and Signature Validation

1.  **Enhance `ValidateBlock`:** The `ValidateBlock` function in `core/validate.go` needs to be significantly refactored.
2.  **Access Consensus State:** The validation logic needs access to the current consensus state (specifically, the `ValidatorSet` and the `SelectProposer` function). This may require passing the `Consensus` object or a dedicated interface to the `BlockChain`'s `ValidateBlock` method.
3.  **Add Proposer Verification:** Before validating anything else, the function must:
    a.  Call `SelectProposer` to determine who the **expected** proposer for the block's height and round is.
    b.  Compare the public key of the expected proposer with the public key of the block's actual proposer (`block.Header.Proposer`). If they do not match, the block is invalid.
4.  **Add Signature Verification:** Immediately after confirming the proposer is correct, use the proposer's public key to verify the block's signature (`block.Header.Signature`). If the signature is invalid, the block must be rejected.

POA(Proof of Authority) 요약과 현재 상태 분석 정리 중:
POA(Proof of Authority)란?
POA는 합의 알고리즘으로, 미리 승인된 검증자(Validator)들이 블록을 생성하고 검증합니다.
특징
검증자 신원이 공개되어 책임이 명확함
검증자 수가 제한되어 빠른 합의 가능
검증자 권한이 명확해 보안이 강함
검증자 선정은 권한 기반(스테이킹 기반도 가능)
작동 방식
제안자 선택: 라운드 로빈 등으로 제안자 결정
블록 제안: 제안자가 블록 생성 후 네트워크에 제안
검증: 다른 검증자들이 블록 검증 및 투표
합의: 2/3 이상 동의 시 블록 커밋
현재 구현 상태 분석
구현된 부분
Validator 관리 구조
ValidatorSet: 검증자 목록 관리
StakerSet: 스테이킹 기반 검증자 선정
ProposerSelector: 라운드 로빈 제안자 선택
투표 시스템
VoteSet: Prevote/Precommit 투표 집계
2/3 다수결 검증 로직
블록 제안 로직
proposeBlock(): 제안자가 블록 생성
BlockProposal: 제안 메시지 구조
P2P 메시지 타입
MsgTypeProposal, MsgTypeVote 정의
BroadcastProposal(), BroadcastVote() 함수 존재
문제점
P2P 메시지 처리 누락
p2p.goLines 97-113
   // handleMessage 수신 메시지 처리
   func (s *P2PService) handleMessage(msg *Message, peer *Peer) {
   	switch msg.Type {
   	case MsgTypePing:
   		s.handlePing(peer)
   	case MsgTypePong:
   		// Pong은 LastSeen 업데이트만 (handlePeer에서 처리됨)
   	case MsgTypeNewBlock:
   		s.handleNewBlock(msg, peer)
   	case MsgTypeNewTx:
   		s.handleNewTx(msg, peer)
   	case MsgTypeGetBlocks:
   		s.handleGetBlocks(msg, peer)
   	case MsgTypeBlocks:
   		s.handleBlocks(msg, peer)
   	}
   }
   // handleMessage 수신 메시지 처리   func (s *P2PService) handleMessage(msg *Message, peer *Peer) {    switch msg.Type {    case MsgTypePing:      s.handlePing(peer)    case MsgTypePong:      // Pong은 LastSeen 업데이트만 (handlePeer에서 처리됨)    case MsgTypeNewBlock:      s.handleNewBlock(msg, peer)    case MsgTypeNewTx:      s.handleNewTx(msg, peer)    case MsgTypeGetBlocks:      s.handleGetBlocks(msg, peer)    case MsgTypeBlocks:      s.handleBlocks(msg, peer)    }   }
MsgTypeProposal, MsgTypeVote 케이스가 없음
제안/투표 메시지를 받아도 처리하지 않음
Proposal 브로드캐스트 누락
engine.goLines 175-212
   // proposeBlock 블록 제안
   func (e *ConsensusEngine) proposeBlock() {
   	// ... 블록 생성 ...
   	e.proposedBlock = newBlock
   	// 투표셋 초기화
   	e.prevotes = NewVoteSet(newBlock.Header.Height, e.consensus.CurrentRound, VoteTypePrevote)
   	e.precommits = NewVoteSet(newBlock.Header.Height, e.consensus.CurrentRound, VoteTypePrecommit)
   	logger.Info("[Consensus] Proposed block ", newBlock.Header.Height, " (hash: ", utils.HashToString(newBlock.Header.Hash)[:16], ")")
   	// ❌ P2P로 브로드캐스트하는 코드가 없음!
   }
   // proposeBlock 블록 제안   func (e *ConsensusEngine) proposeBlock() {    // ... 블록 생성 ...    e.proposedBlock = newBlock    // 투표셋 초기화    e.prevotes = NewVoteSet(newBlock.Header.Height, e.consensus.CurrentRound, VoteTypePrevote)    e.precommits = NewVoteSet(newBlock.Header.Height, e.consensus.CurrentRound, VoteTypePrecommit)    logger.Info("[Consensus] Proposed block ", newBlock.Header.Height, " (hash: ", utils.HashToString(newBlock.Header.Hash)[:16], ")")    // ❌ P2P로 브로드캐스트하는 코드가 없음!   }
블록 생성 후 다른 노드로 전송하지 않음
Vote 브로드캐스트 누락
engine.goLines 279-324
   // castVote 투표 생성 및 전송
   func (e *ConsensusEngine) castVote(voteType VoteType, blockHash prt.Hash) {
   	// ... 투표 생성 ...
   	// 로컬 투표 처리
   	// ❌ P2P로 브로드캐스트하는 코드가 없음!
   }
   // castVote 투표 생성 및 전송   func (e *ConsensusEngine) castVote(voteType VoteType, blockHash prt.Hash) {    // ... 투표 생성 ...    // 로컬 투표 처리    // ❌ P2P로 브로드캐스트하는 코드가 없음!   }
투표 생성 후 다른 노드로 전송하지 않음
Validator 등록 미연결
consensus.goLines 87-103
   // RegisterValidator 로컬 노드를 검증자로 등록
   func (c *Consensus) RegisterValidator(address prt.Address, publicKey []byte, privateKey []byte) error {
   	// ...
   	c.LocalValidator = validator
   	c.LocalProposer = NewProposer(validator, privateKey)
   	return nil
   }
   // RegisterValidator 로컬 노드를 검증자로 등록   func (c *Consensus) RegisterValidator(address prt.Address, publicKey []byte, privateKey []byte) error {    // ...    c.LocalValidator = validator    c.LocalProposer = NewProposer(validator, privateKey)    return nil   }
app.go에서 호출되지 않음
LocalValidator가 nil이면 투표 불가
Proposal 검증 미완성
engine.goLines 214-237
   // HandleProposal 제안 메시지 처리 (P2P로부터)
   func (e *ConsensusEngine) HandleProposal(height uint64, round uint32, blockHash prt.Hash, block *core.Block) {
   	// 블록 검증
   	if err := e.blockchain.ValidateBlock(*block); err != nil {
   		logger.Error("[Consensus] Invalid proposed block: ", err)
   		return
   	}
   	// ❌ 제안자가 실제로 권한이 있는지 확인하지 않음!
   }
   // HandleProposal 제안 메시지 처리 (P2P로부터)   func (e *ConsensusEngine) HandleProposal(height uint64, round uint32, blockHash prt.Hash, block *core.Block) {    // 블록 검증    if err := e.blockchain.ValidateBlock(*block); err != nil {      logger.Error("[Consensus] Invalid proposed block: ", err)      return    }    // ❌ 제안자가 실제로 권한이 있는지 확인하지 않음!   }
제안자 권한 검증 없음
POA 작동을 위해 필요한 내용
1. P2P 메시지 처리 연결
p2p/p2p.go의 handleMessage에 MsgTypeProposal, MsgTypeVote 처리 추가
수신한 Proposal/Vote를 ConsensusEngine으로 전달
2. Proposal 브로드캐스트 연결
proposeBlock()에서 블록 생성 후 P2PService.BroadcastProposal() 호출
제안 메시지에 블록 데이터 포함
3. Vote 브로드캐스트 연결
castVote()에서 투표 생성 후 P2PService.BroadcastVote() 호출
다른 검증자들이 투표를 받을 수 있도록
4. Validator 초기화
app.go에서 노드 시작 시 RegisterValidator() 호출
지갑의 주소/키로 검증자 등록
5. Proposal 검증 강화
HandleProposal에서 제안자가 현재 라운드의 제안자인지 확인
ProposerSelector.SelectProposer()로 검증
6. Vote 검증 강화
HandleVote에서 투표자가 검증자인지 확인
서명 검증 추가
7. ValidatorSet 동기화
모든 노드가 동일한 ValidatorSet을 유지
제네시스 블록 또는 설정 파일로 초기화
현재 문제의 원인
"처음 뜨는 노드가 프로포절만 하고 검증이 안 되는" 이유:
Proposal을 브로드캐스트하지 않아 다른 노드가 받지 못함
다른 노드가 Proposal을 받아도 P2P에서 처리하지 않음
Vote를 브로드캐스트하지 않아 합의가 진행되지 않음
Validator가 등록되지 않아 투표를 생성하지 못함
결과적으로 제안 노드는 블록을 생성하지만, 다른 노드와의 합의가 이루어지지 않습니다.