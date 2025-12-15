package protocol

const (
	// Blockchain configuration info
	PrefixNetworkConfig = "net:config"

	// Metadata related prefixes
	PrefixMeta          = "meta:"       // Metadata key
	PrefixMetaHeight    = "meta:height" // Latest block height
	PrefixMetaBlockHash = "meta:hash"   // Latest block hash

	// Block related prefixes
	PrefixBlock         = "blk:"     // blk:Hash = Block data
	PrefixBlockByHeight = "blk:h:"   // blk:h:Height = Block hash
	PrefixBlockTxs      = "blk:txs:" // blk:txs:BlockHash:Index = Transaction hash

	// Transaction related prefixes
	PrefixTxs      = "tx:"        // tx:TxHash = Transaction data
	PrefixTxStatus = "tx:status:" // tx:status:Hash = Status
	PrefixTxBlock  = "tx:blk:"    // tx:block:TxHash = Block hash

	// [Usage pattern 1] tx:in:TxHash:Index = Specific input data
	// [Usage pattern 2] tx:in:TxHash = All input data list
	PrefixTxIn = "tx:in:"

	// [Usage pattern 1] tx:out:TxHash:Index = Specific output data
	// [Usage pattern 2] tx:out:TxHash = All output data list
	PrefixTxOut = "tx:out:"

	// UTXO related prefixes
	PrefixUtxo        = "utxo:"      // utxo:TxHash:Index = UTXO data
	PrefixUtxoList    = "utxo:addr:" // utxo:addr:Address = UTXO key array
	PrefixUtxoBalance = "utxo:bal:"  // utxo:bal:Address = Balance

	// Account related prefixes
	PrefixAddress         = "addr:"      // addr:AccountAddress = Account data
	PrefixAddressTxs      = "addr:txs:"  // addr:txs:AccountAddress = Transaction hash json-array
	PrefixAddressReceived = "addr:recv:" // addr:recv:AccountAddress:Index = []{TxHash: index} (Received)
	PrefixAddressSent     = "addr:sent:" // addr:sent:AccountAddress:Index = []TxHash (Sent)

	// Consensus related prefixes
	PrefixStakerInfo = "staker:" // Wallet address - Staking info
)
