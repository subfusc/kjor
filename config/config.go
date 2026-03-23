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
	Name           string
	Args           []string
	RestartTimeout int
	BuildDelay     int
}

type FileWatcherConfig struct {
	IgnoreFile     []string
	IgnoreFullPath []string
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
	Logger      LoggerConfig
}

var ConfigNotFound = errors.New("Config file not found")

func DefaultConfig() *Config {
	return &Config{
		Program: ProgConfig{
			Name:           "./a.out",
			Args:           []string{},
			RestartTimeout: 500,
			BuildDelay:     100,
		},
		Build: ProgConfig{
			Name: "go",
			Args: []string{"build", "-o", "a.out", "./"},
		},
		Filewatcher: FileWatcherConfig{
			IgnoreFile:     []string{"^\\.#", "^#", "~$", "_test\\.go$", "a\\.out(-go-tmp-unmask)?$"},
			IgnoreFullPath: []string{"/tmp/.*", ".*/\\.git/.*"},
		},
		SSE: SSEConfig{
			Enable:         false,
			Port:           8888,
			RestartTimeout: 1000,
		},
		Logger: LoggerConfig{
			Verbose: false,
			Style:   "terminal",
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
