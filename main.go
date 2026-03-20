// -*- coding: utf-8 -*-
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"runtime"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/subfusc/kjor/config"
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
SSE:                 %t
SSE Port:            %d
Working Directory:   %s
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
			case 0, 1, 2, 3, 4, 5, 6:
				cb.Fg(fgs[0])
			case 9, 10, 11, 12, 13, 14, 15:
				cb.Fg(fgs[1])
			case 18, 19, 20, 21, 22, 23, 24:
				cb.Fg(fgs[2])
			case 27, 28, 29, 30, 31, 32, 33:
				cb.Fg(fgs[3])
			default:
				cb.Fg(Color{255, 255, 255})
			}

			buf.WriteString(cb.String())
		} else {
			buf.WriteRune(r)
		}

		i++
	}

	return buf.String()
}

func printAndExitWithFailState(msg string, err error) {
	slog.Error(msg, "err", err)
	os.Exit(1)
}

func printBanner(c *config.Config, wd string) {
	if c.Logger.Style == "terminal" {
		fmt.Print(bannerRandomColor())
	} else {
		fmt.Print(banner)
	}
	fmt.Printf(info, runtime.GOOS, c.SSE.Enable, c.SSE.Port, wd)
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

func mustReadConfig() *config.Config {
	cfg, err := config.ReadConfig()
	switch {
	case errors.Is(err, config.ConfigNotFound):
		cfg = config.DefaultConfig()
		file, err := os.Create("kjor.toml")
		if err != nil {
			printAndExitWithFailState("Failed to create standard config", err)
		}
		enc := toml.NewEncoder(file)
		enc.Encode(cfg)
		file.Close()
	case err != nil:
		printAndExitWithFailState("Unable to read config", err)
	case !cfg.IsValid():
		printAndExitWithFailState("Config is not complete", nil)
	}

	return cfg
}

func mustSetupFSWatch(ctx context.Context, cfg *config.Config, wd string) *FSWatcher {
	fw, err := NewFSWatcher(cfg)
	if err != nil {
		slog.Error("Failed to create filewatcher", "err", err)
		os.Exit(1)
	}

	if err := fw.Watch(wd); err != nil {
		slog.Error("Failed to watch current pwd", "err", err)
	}

	fw.Start(ctx)

	return fw
}

func start(cfg *config.Config, fw *FSWatcher, proc *Process, sseServer *sse.Server) {
	for range fw.EventStream() {
		restarted := false

		select {
		case proc.restart <- struct{}{}:
			restarted = true
		default:
		}

		if cfg.SSE.Enable && len(sseServer.MsgChan) < cap(sseServer.MsgChan) {
			if restarted && cap(sseServer.MsgChan) > len(sseServer.MsgChan){
				sseServer.MsgChan <- sse.Event{
					Type: "build_action",
					Source: sse.WATCHER,
					Data: map[string]any{"restarted": true},
					When: time.Now(),
				}
			}
		}
	}
}

func main() {
	mainCtx, mainCcl := context.WithCancel(context.Background())
	defer mainCcl()

	cfg := mustReadConfig()
	loggers := loggerFromConfig(cfg)
	wd, err := os.Getwd()
	if err != nil {
		fmt.Printf("Unable to find Working Directory: %s\n", wd)
		os.Exit(1)
	}

	printBanner(cfg, wd)

	fw := mustSetupFSWatch(mainCtx, cfg, wd)

	proc, err := NewProcess(cfg, slog.New(loggers.Build), loggers.ProgramStandard, loggers.ProgramError)
	if err != nil {
		slog.Error("Failed to create process", "err", err)
		os.Exit(1)
	}

	proc.Start(mainCtx)

	sseLog := slog.New(loggers.SSE)
	var sseServer *sse.Server
	if cfg.SSE.Enable {
		sseServer = sse.NewServer(cfg, sseLog)
		sseServer.Start(mainCtx)
	}

	start(cfg, fw, proc, sseServer)
}
