package main

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

type Config struct {
	Program ProgConf
	Build ProgConf
	Filewatcher FaConf
}

var ConfigNotFound = errors.New("Config file not found")

func defaultConfig() *Config {
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
	}
}

func (c *Config) IsValid() bool {
	return c.Program.Name != "" && c.Build.Name != ""
}

func readConfig() (*Config, error) {
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
