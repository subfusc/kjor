// -*- coding: utf-8 -*-
package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"runtime"

	"github.com/BurntSushi/toml"
	"github.com/subfusc/kjor/fanotify_watcher"
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

GOOS:                    %s
Fanotify Version:        %d
DAC_READ_SEARCH capable: %t

`

func checkSupport() {
	if fanotify_watcher.IsSupported() {
		fmt.Printf(banner, runtime.GOOS, fanotify_watcher.FanotifyVersion(), fanotify_watcher.CapabilityDacReadSearch())
	} else {
		fmt.Println("Sorry, your system is currently not supported")
		os.Exit(0)
	}
}

func createFileWatcher(c *Config) (*fanotify_watcher.FaNotifyWatcher, error) {
	watcher, err := fanotify_watcher.NewFaNotifyWatcher()
	if err != nil {
		return nil, fmt.Errorf("Got err starting watcher: %+v\n", err)
	}

	if err := watcher.Watch("."); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("Failed to watch current directory: [%v]", err)
	}

	for _, ignore := range c.Filewatcher.Ignore {
		re, err := regexp.Compile(ignore)
		if err != nil {
			return nil, fmt.Errorf("Failed to compile re: %s [%v]", ignore, err)
		}
		watcher.IgnoreFileNameMatching(re)
	}

	return watcher, nil
}

func main() {
	checkSupport()
	cfg, err := readConfig()
	switch {
	case errors.Is(err, ConfigNotFound) :
		cfg = defaultConfig()
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

	watcher, err := createFileWatcher(cfg)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer watcher.Close()

	sseServer := sse.NewServer()
	defer sseServer.Close()

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

	go watcher.Start()
	go sseServer.Start()

	for e := range watcher.EventStream {
		fmt.Printf("File changed: %s\n", e.FileName)
		err, restarted := proc.Restart()
		if restarted && sseServer.MsgChan != nil {
			sseServer.MsgChan <- sse.Event{Type: "server_message", Data: map[string]bool{"restarted": true}}
		}

		if err != nil {
			fmt.Println(err)
		}
	}
}
