package config

import (
	"errors"
	"os"

	"github.com/BurntSushi/toml"
)

type ProgConfig struct {
	Name string
	Args []string
}

type FileWatcherConfig struct {
	Backend string
	Ignore  []string
}

type SSEConfig struct {
	Enable         bool
	Port           int
	RestartTimeout int
}

type Config struct {
	Program     ProgConfig
	Build       ProgConfig
	Filewatcher FileWatcherConfig
	SSE         SSEConfig
}

var ConfigNotFound = errors.New("Config file not found")

func DefaultConfig() *Config {
	return &Config{
		Program: ProgConfig{
			Name: "./a.out",
			Args: []string{},
		},
		Build: ProgConfig{
			Name: "go",
			Args: []string{"build", "-o", "a.out", "./"},
		},
		Filewatcher: FileWatcherConfig{
			Backend: "inotify",
			Ignore:  []string{"^\\.#", "^#", "~$", "_test\\.go$", "a\\.out$"},
		},
		SSE: SSEConfig{
			Enable:         true,
			Port:           8888,
			RestartTimeout: 1000,
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
