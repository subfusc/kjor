// -*- coding: utf-8 -*-
package main

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
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
`

var info = `
GOOS:                %s
FileWatcher Backend: %s
SSE:                 %t
SSE Port:            %d
`

func bannerRandomColor() string {
	buf := bytes.NewBuffer(nil)
	fgs := make([]Color, 4)
	for i := range fgs {
		fgs[i] = Color{byte(rand.Int() % 255), byte(rand.Int() % 255), byte(rand.Int() % 255)}
	}

	i := 0
	for _, r := range []rune(banner) {
		if r == '\n' {
			i = 0
		}

		if r == '#' {
			cb := NewAnsiColorBuilder(string(r))
			switch i {
			case 0,1,2,3,4,5,6:
				cb.Fg(fgs[0])
			case 9,10,11,12,13,14,15:
				cb.Fg(fgs[1])
			case 18,19,20,21,22,23,24:
				cb.Fg(fgs[2])
			case 27,28,29,30,31,32,33:
				cb.Fg(fgs[3])
			default:
				cb.Fg(Color{255,255,255})
			}

			buf.WriteString(cb.String())
		} else {
			buf.WriteRune(r)
		}

		i++
	}

	return buf.String()
}

func checkSupport(c *config.Config) {
	if runtime.GOOS == "linux" {
		if c.Logger.Style == "terminal" {
			fmt.Print(bannerRandomColor())
		} else {
			fmt.Print(banner)
		}
		fmt.Printf(info, runtime.GOOS, c.Filewatcher.Backend, c.SSE.Enable, c.SSE.Port)
	} else {
		fmt.Println("Sorry, your system is currently not supported")
		os.Exit(0)
	}
}

func loggerFromConfig(c *config.Config) *KjorOutput {
	levels := []slog.Level{slog.LevelInfo, slog.LevelWarn, slog.LevelError}
	if c.Logger.Verbose {
		levels = []slog.Level{slog.LevelDebug, slog.LevelDebug, slog.LevelDebug}
	}
	if c.Logger.Style == "terminal" {
		return FancyKjorLogger(levels[0], levels[1], levels[2])
	}

	return UnfancyKjorLogger(levels[0], levels[1], levels[2])
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

	loggers := loggerFromConfig(cfg)

	fw, err := file_watcher.NewFileWatcher(
		cfg,
		slog.New(loggers.FileWatcher),
	)
	fw.Watch(wd)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer fw.Close()

	proc, err := NewProcess(
		cfg,
		slog.New(loggers.Build),
		loggers.ProgramStandard,
		loggers.ProgramError,
	)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer proc.Stop()

	proc.Start()

	sseLog := slog.New(loggers.SSE)
	var sseServer *sse.Server
	if cfg.SSE.Enable {
		sseServer = sse.NewServer(cfg, sseLog)
		defer sseServer.Close()

		go sseServer.Start()
	}

	go fw.Start()

	var mainSlog *slog.Logger
	if cfg.Logger.Style == "terminal" {
		mainSlog = slog.New(NewTerminalLoggerWithName(os.Stdout, slog.LevelInfo, "Mn ", Color{255,255,255}, Color{200,30,30}))
	} else {
		mainSlog = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))
	}

	for range fw.EventStream() {
		err, restarted := proc.Restart()

		if cfg.SSE.Enable && len(sseServer.MsgChan) < cap(sseServer.MsgChan) {
			switch {
			case errors.Is(err, ProcessBuildFailed):
				sseServer.MsgChan <- sse.Event{Type: "build_message", Source: sse.WATCHER, Data: map[string]any{"message": "Build failed"}, When: time.Now()}
			case restarted && cap(sseServer.MsgChan) > len(sseServer.MsgChan):
				sseServer.MsgChan <- sse.Event{Type: "build_action", Source: sse.WATCHER, Data: map[string]any{"restarted": true}, When: time.Now()}
			}
		}

		if err != nil && !errors.Is(err, ProcessBuildFailed) {
			mainSlog.Error("Got an error thrown into main loop", "err", err)
		}
	}
}
