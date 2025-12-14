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
	NetworkID     string // 네트워크 식별자
	BlockProducer bool   // 블록 생성자 여부
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
	Timestamp       int64    `toml:"Timestamp"` // 제네시스 블록 타임스탬프 (고정값, 0이면 현재 시간 사용)
}

// ValidatorConfig 제네시스 검증자 설정
type ValidatorConfig struct {
	Address     string `toml:"address"`
	PublicKey   string `toml:"publicKey"`   // hex encoded
	VotingPower uint64 `toml:"votingPower"`
}

// Validators 검증자 목록 설정 (PoA용)
type Validators struct {
	List []ValidatorConfig `toml:"list"`
}

type Server struct {
	RestPort int `toml:"RestPort"`
}

type P2P struct {
	Address   string   `toml:"Address"`
	Port      int      `toml:"Port"`
	BootNodes []string `toml:"BootNodes"`
}

// Fee 수수료 설정
type Fee struct {
	MinFee      uint64 `toml:"minFee"`      // 최소 수수료 (고정값)
	BlockReward uint64 `toml:"blockReward"` // 블록 생성 보상
}

type Config struct {
	Common     Common
	LogInfo    LogInfo
	DB         DB
	Wallet     Wallet
	Version    Version
	Genesis    Genesis
	Validators Validators // 제네시스 검증자 목록 (PoA)
	Server     Server
	P2P        P2P
	Fee        Fee // 수수료 설정
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
