package core

import (
	"fmt"
	"time"

	"github.com/abcfe/abcfe-node/common/utils"
	prt "github.com/abcfe/abcfe-node/protocol"
)

func (p *BlockChain) SetGenesisBlock() (*Block, error) {
	var defaultPrevHash prt.Hash
	var emptyProposer prt.Address
	var emptySignature prt.Signature

	// Since genesis block has no previous block hash, it is set to 0
	for i := range defaultPrevHash {
		defaultPrevHash[i] = 0x00
	}

	// Get timestamp from config (use current time if 0)
	genesisTimestamp := p.cfg.Genesis.Timestamp
	if genesisTimestamp == 0 {
		genesisTimestamp = time.Now().Unix()
	}

	txs, err := p.setGenesisTxs(genesisTimestamp)
	if err != nil {
		return nil, err
	}

	// Calculate merkle root
	merkleRoot := calculateMerkleRoot(txs)

	blkHeader := &BlockHeader{
		PrevHash:   defaultPrevHash,
		Version:    p.cfg.Version.Protocol,
		Height:     0,
		Timestamp:  genesisTimestamp,
		MerkleRoot: merkleRoot,
	}

	block := &Block{
		Header:       *blkHeader,
		Transactions: txs,
		Proposer:     emptyProposer,  // Genesis block has no proposer
		Signature:    emptySignature, // Genesis block has no signature
	}

	// Calculate block hash using only Header (same way as normal block)
	blkHash := utils.Hash(block.Header)
	block.Header.Hash = blkHash

	return block, nil
}

func (p *BlockChain) setGenesisTxs(genesisTimestamp int64) ([]*Transaction, error) {
	txIns := []*TxInput{}
	txOuts := []*TxOutput{}

	systemAddrs := p.cfg.Genesis.SystemAddresses
	systemBals := p.cfg.Genesis.SystemBalances

	if len(systemAddrs) != len(systemBals) {
		return nil, fmt.Errorf("system address and balance count mismatch")
	}

	for i, systemAddr := range systemAddrs {
		addr, err := utils.StringToAddress(systemAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to convert between address and string")
		}

		output := &TxOutput{
			Address: addr,
			Amount:  systemBals[i],
			TxType:  TxTypeGeneral,
		}
		txOuts = append(txOuts, output)
	}

	txs := []*Transaction{
		{
			Version:   p.cfg.Version.Transaction,
			Timestamp: genesisTimestamp,
			Inputs:    txIns,
			Outputs:   txOuts,
			Memo:      "ABCFE Chain Genesis Block",
		},
	}

	// TODO Put signature value and then hash

	for i, tx := range txs {
		txHash := utils.Hash(tx)
		txs[i].ID = txHash
	}

	return txs, nil
}
