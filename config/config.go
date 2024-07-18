package config

import (
	"errors"
	"os"

	"github.com/BurntSushi/toml"
)

type ProgConf struct {
	Name string
	Args []string
}

type FaConf struct {
	Ignore []string
}

type SSEConfig struct {
	Enable bool
	Port int
}

type Config struct {
	Program ProgConf
	Build ProgConf
	Filewatcher FaConf
	SSE SSEConfig
}

var ConfigNotFound = errors.New("Config file not found")

func DefaultConfig() *Config {
	return &Config{
		Program: ProgConf{
			Name: "./a.out",
			Args: []string{},
		},
		Build: ProgConf{
			Name: "go",
			Args: []string{"build", "-o", "a.out", "./"},
		},
		Filewatcher: FaConf{
			Ignore: []string{"^\\.#", "^#", "~$", "_test\\.go$", "a\\.out$"},
		},
		SSE: SSEConfig{
			Enable: true,
			Port: 8888,
		},
	}
}

func (c *Config) IsValid() bool {
	return c.Program.Name != "" && c.Build.Name != ""
}

func ReadConfig() (*Config, error) {
	configFile := "kjor.toml"
	if len(os.Args) > 1 {
		configFile = os.Args[1]
	}

	if _, err := os.Stat(configFile); errors.Is(err, os.ErrNotExist) {
		return nil, ConfigNotFound
	}

	var config Config
	if _, err := toml.DecodeFile(configFile, &config); err != nil {
		return nil, err
	}
	return &config, nil
}
