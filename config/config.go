package config

import (
	"errors"
	"os"

	"github.com/BurntSushi/toml"
)

type LoggerConfig struct {
	Verbose bool
	Style   string
}

type ProgConfig struct {
	Name string
	Args []string
}

type ProcessConfig struct {
	Program ProgConfig
	Build   ProgConfig
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
	Process     ProcessConfig
	Program     ProgConfig
	Build       ProgConfig
	Filewatcher FileWatcherConfig
	SSE         SSEConfig
	Logger      LoggerConfig
}

var ConfigNotFound = errors.New("Config file not found")

func DefaultConfig() *Config {
	return &Config{
		Process: ProcessConfig{
			Program: ProgConfig{
				Name: "./a.out",
				Args: []string{},
			},
			Build: ProgConfig{
				Name: "go",
				Args: []string{"build", "-o", "a.out", "./"},
			},
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
		Logger: LoggerConfig{
			Verbose: false,
			Style: "terminal",
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

	config := DefaultConfig()
	if _, err := toml.DecodeFile(configFile, config); err != nil {
		return nil, err
	}
	return config, nil
}
