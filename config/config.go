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
}

type Server struct {
	RestPort int `toml:"RestPort"`
}

type P2P struct {
	Address   string   `toml:"Address"`
	Port      int      `toml:"Port"`
	BootNodes []string `toml:"BootNodes"`
}

type Config struct {
	Common  Common
	LogInfo LogInfo
	DB      DB
	Wallet  Wallet
	Version Version
	Genesis Genesis
	Server  Server
	P2P     P2P
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
