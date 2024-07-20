// -*- coding: utf-8 -*-
package main

import (
	"errors"
	"fmt"
	"os"
	"runtime"

	"github.com/BurntSushi/toml"
	"github.com/subfusc/kjor/config"
	"github.com/subfusc/kjor/file_watcher"
	"github.com/subfusc/kjor/file_watcher/fanotify_watcher"
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

GOOS:                        %s
Fanotify Version:            %d
CAP_DAC_READ_SEARCH capable: %t

`

func checkSupport() {
	if fanotify_watcher.IsSupported() {
		fmt.Printf(banner, runtime.GOOS, fanotify_watcher.FanotifyVersion(), fanotify_watcher.CapabilityDacReadSearch())
	} else {
		fmt.Println("Sorry, your system is currently not supported")
		os.Exit(0)
	}
}

func createFileWatcher(c *config.Config) (*fanotify_watcher.FaNotifyWatcher, error) {
	watcher, err := fanotify_watcher.NewFaNotifyWatcher(c)
	if err != nil {
		return nil, fmt.Errorf("Got err starting watcher: [%v]\n", err)
	}

	if err := watcher.Watch("."); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("Failed to watch current directory: [%v]", err)
	}

	return watcher, nil
}

func main() {
	checkSupport()
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

	fw, err := file_watcher.NewFileWatcher(cfg)
	fw.Watch(os.Getenv("PWD"))

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
			sseServer.MsgChan <- sse.Event{Type: "server_message", Data: map[string]bool{"restarted": true}}
		}

		if err != nil {
			fmt.Println(err)
		}
	}
}
