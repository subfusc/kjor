package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/subfusc/kjor/config"
)

type AppProcessWriter struct {
	writer io.Writer
}

func NewAppProcessWriter(writer io.Writer) *AppProcessWriter {
	return &AppProcessWriter{writer: writer}
}

func (apw *AppProcessWriter) Write(out []byte) (int, error) {
	outs := bytes.Split(out, []byte{10})
	begin := "\x1b[48;2;0;0;255mApp\x1b[0m\x1b[38;2;0;0;255m🭬\x1b[0m"
	for _, o := range outs {
		if len(o) == 0 {
			continue
		}

		buf := bytes.NewBufferString(begin)
		buf.Write(o)
		buf.WriteByte(10)
		_, err := apw.writer.Write(buf.Bytes())
		if err != nil {
			slog.Error("Failed to write to apw pipe", "err", err)
			return 0, err
		}
	}
	return len(out), nil
}

type Executable struct {
	program string
	args    []string
	found   bool
}

func (e Executable) Exists(cached bool) bool {
	if e.found && cached {
		return true
	}

	str, err := exec.LookPath(e.program)
	if err != nil {
		return false
	}

	e.program = str
	return true
}

func (e Executable) Program() string {
	return e.program
}

func (e Executable) Args() []string {
	return e.args
}

var (
	ProcessBuildFailed = errors.New("Build failed")
)

type Process struct {
	appError      io.Writer
	appOutput     io.Writer
	cancel        context.CancelFunc
	cmd           *exec.Cmd
	lastRestarted time.Time
	builder       Executable
	runner        Executable
	buildtOnce    bool
	processLog    *slog.Logger
	timeout       time.Duration
	buildDelay time.Duration
	restart       chan struct{}
}

func ProgramNotFound(err error) error {
	return fmt.Errorf("Failed to find program: [%v]", err)
}

func NewProcess(c *config.Config, logger *slog.Logger, stdOut io.Writer, stdErr io.Writer) (*Process, error) {
	p := &Process{
		appError:      stdErr,
		appOutput:     stdOut,
		cancel:        nil,
		cmd:           nil,
		lastRestarted: time.Now(),
		runner: Executable{
			program: c.Program.Name,
			args:    c.Program.Args,
		},
		builder: Executable{
			program: c.Build.Name,
			args:    c.Build.Args,
		},
		buildtOnce: false,
		processLog: logger,
		timeout:    time.Duration(c.Program.RestartTimeout) * time.Millisecond,
		buildDelay: time.Duration(c.Program.BuildDelay) * time.Millisecond,
		restart:    make(chan struct{}),
	}

	if found := p.builder.Exists(false); !found {
		return nil, fmt.Errorf("Faild to find builder program: [%w]", ProcessBuildFailed)
	}

	if err := os.MkdirAll(filepath.Dir(p.runner.program), 0750); err != nil {
		return nil, err
	}

	return p, nil
}

func (p *Process) newCmd(e Executable) (*exec.Cmd, context.CancelFunc) {
	ctx, ctl := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, e.Program(), e.Args()...)
	cmd.Cancel = func() error {
		return cmd.Process.Kill()
	}

	// What about Stdin?
	cmd.Stdout = p.appOutput
	cmd.Stderr = p.appError

	return cmd, ctl
}

func (p *Process) build() error {
	cmd, _ := p.newCmd(p.builder)
	t := time.Now()
	err := cmd.Run()
	dx := time.Since(t)

	if err != nil {
		p.processLog.Warn("Build failed", "err", err, "time", dx)
		return ProcessBuildFailed
	}

	p.processLog.Info("Build", "time", dx)
	return nil
}

func (p *Process) Start(programCtx context.Context) {
	go func() {
		if err := p.build(); err != nil {
			p.processLog.Error("First build failed", "err", err)
		}

		var cmd *exec.Cmd
		var ccl context.CancelFunc

		for {
			startProcess := p.runner.Exists(false)

			if cmd != nil && cmd.Process != nil {
				if err := cmd.Process.Signal(syscall.Signal(0)); err == nil {
					startProcess = false
				}
			}

			if startProcess {
				cmd, ccl = p.newCmd(p.runner)
				err := cmd.Start()

				if err != nil {
					p.processLog.Error("Failed to start program", "err", err)
				}
			}

			sleeper := make(chan struct{})
			go func() {
				time.Sleep(p.timeout)
				sleeper <- struct{}{}
			}()

			select {
			case <-sleeper:
			case <-programCtx.Done():
				if ccl != nil {
					ccl()
				}
				return
			}

			select {
			case <-p.restart:
				time.Sleep(p.buildDelay)
			case <-programCtx.Done():
				if ccl != nil {
					ccl()
				}
				return
			}

			err := p.build()
			if err != nil {
				p.processLog.Error("Failed to build program", "err", err)
				continue
			}

			if ccl != nil {
				ccl()
			}

			if cmd != nil {
				// TODO: Should return an exit state error.
				// are there other kinds of errors needed to be catched here?
				cmd.Wait()
			}

			cmd, ccl = p.newCmd(p.runner)
		}
	}()
}
