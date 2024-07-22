// -*- coding: utf-8 -*-
package main

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/subfusc/kjor/config"
	"github.com/subfusc/kjor/file_watcher"
	"github.com/subfusc/kjor/sse"
)

var banner = `
#    #        #  #######  ######
#   #         #  #     #  #     #
#  #          #  #     #  #     #
###           #  #     #  ######
#  #    #     #  #     #  #   #
#   #   #     #  #     #  #    #
#    #   #####   #######  #     #

GOOS:                %s
FileWatcher Backend: %s
SSE:                 %t
SSE Port:            %d
`

func checkSupport(c *config.Config) {
	if runtime.GOOS == "linux" {
		fmt.Printf(banner, runtime.GOOS, c.Filewatcher.Backend, c.SSE.Enable, c.SSE.Port)
	} else {
		fmt.Println("Sorry, your system is currently not supported")
		os.Exit(0)
	}
}

func main() {
	cfg, err := config.ReadConfig()
	switch {
	case errors.Is(err, config.ConfigNotFound) :
		cfg = config.DefaultConfig()
		file, err := os.Create("kjor.toml")
		if err != nil {
			fmt.Println("Failed to create standard config")
			os.Exit(1)
		}
		enc := toml.NewEncoder(file)
		enc.Encode(cfg)
		file.Close()
	case err != nil:
		fmt.Println("Unable to read config")
		os.Exit(1)
	case !cfg.IsValid():
		fmt.Println("Config is not complete")
		os.Exit(1)
	}

	checkSupport(cfg)

	wd, err := os.Getwd()
	if err != nil {
		fmt.Printf("Unable to find Working Directory: %s\n", wd)
		os.Exit(1)
	}

	fw, err := file_watcher.NewFileWatcher(cfg)
	fw.Watch(wd)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer fw.Close()

	proc, err := NewProcess(cfg)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer proc.Stop()

	if err := proc.Start(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var sseServer *sse.Server
	if cfg.SSE.Enable {
		sseServer = sse.NewServer(cfg)
		defer sseServer.Close()

		go sseServer.Start()
	}

	go fw.Start()

	for range fw.EventStream() {
		err, restarted := proc.Restart()
		if cfg.SSE.Enable && restarted && cap(sseServer.MsgChan) > len(sseServer.MsgChan) {
			sseServer.MsgChan <- sse.Event{Type: "server_message", Source: sse.WATCHER, Data: map[string]any{"restarted": true}, When: time.Now()}
		}

		if err != nil {
			fmt.Println(err)
		}
	}
}
