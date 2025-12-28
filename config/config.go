package config

import (
	"os"
	"path"

	"github.com/abcfe/abcfe-node/common/utils"
	"github.com/naoina/toml"
)

type Common struct {
	Level         string // local, dev, prod
	ServiceName   string
	Port          int
	Mode          string // boot, validator, sentry
	NetworkID     string // Network identifier
	BlockProducer bool   // Whether block producer
}

type LogInfo struct {
	Path       string
	MaxAgeHour int
	RotateHour int
	ProdTelKey string
	ProdChatId int
	DevTelKey  string
	DevChatId  int
}

type DB struct {
	Path string
}

type Wallet struct {
	Path string
}

type Version struct {
	Transaction string
	Protocol    string
}

type Genesis struct {
	SystemAddresses []string `toml:"SystemAddresses"`
	SystemBalances  []uint64 `toml:"SystemBalances"`
	Timestamp       int64    `toml:"Timestamp"` // Genesis block timestamp (fixed value, use current time if 0)
}

// ValidatorConfig genesis validator config
type ValidatorConfig struct {
	Address     string `toml:"address"`
	PublicKey   string `toml:"publicKey"`   // hex encoded
	VotingPower uint64 `toml:"votingPower"`
}

// Validators validator list config (for PoA)
type Validators struct {
	List []ValidatorConfig `toml:"list"`
}

type Server struct {
	RestPort         int `toml:"RestPort"`         // 공개 API 포트 (0.0.0.0 바인딩)
	InternalRestPort int `toml:"InternalRestPort"` // 내부 API 포트 (127.0.0.1 바인딩, 0이면 비활성화)
}

type P2P struct {
	Address   string   `toml:"Address"`
	Port      int      `toml:"Port"`
	BootNodes []string `toml:"BootNodes"`
}

// Fee config
type Fee struct {
	MinFee      uint64 `toml:"minFee"`      // Minimum fee (fixed value)
	BlockReward uint64 `toml:"blockReward"` // Block reward
}

// Transaction limit config
type Transaction struct {
	MaxMemoSize uint64 `toml:"maxMemoSize"` // Max memo size (bytes)
	MaxDataSize uint64 `toml:"maxDataSize"` // Max data size (bytes)
}

// Consensus config
type Consensus struct {
	// Proposer selection mode: "roundrobin", "vrf", "hybrid"
	// - roundrobin: Simple round-robin (predictable, default)
	// - vrf: VRF-like hash-based selection (unpredictable)
	// - hybrid: VRF for round 0, round-robin for timeouts
	ProposerSelection string `toml:"proposerSelection"`
}

type Config struct {
	Common      Common
	LogInfo     LogInfo
	DB          DB
	Wallet      Wallet
	Version     Version
	Genesis     Genesis
	Validators  Validators  // Genesis validator list (PoA)
	Server      Server
	P2P         P2P
	Fee         Fee         // Fee config
	Transaction Transaction // Transaction limit config
	Consensus   Consensus   // Consensus config
}

func NewConfig(filepath string) (*Config, error) {
	if filepath == "" {
		workDir, _ := os.Getwd()
		rootDir := utils.FindProjectRoot(workDir)
		filepath = path.Join(rootDir, "config", "config.toml")
	}

	if file, err := os.Open(filepath); err != nil {
		return nil, err
	} else {
		defer file.Close()

		c := new(Config)
		if err := toml.NewDecoder(file).Decode(c); err != nil {
			return nil, err
		} else {
			c.sanitize()
			return c, nil
		}
	}
}

func (p *Config) sanitize() {
	if p.LogInfo.Path[0] == byte('~') {
		p.LogInfo.Path = path.Join(utils.HomeDir(), p.LogInfo.Path[1:])
	}
}

func (p *Config) GetConfig() *Config {
	return p
}

func (p *Config) GetLogInfoConfig() *LogInfo {
	return &p.LogInfo
}
