package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
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
	Program string
	Args    []string
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
}

func ProgramNotFound(err error) error {
	return fmt.Errorf("Failed to find program: [%v]", err)
}


func NewProcess(c *config.Config, logger *slog.Logger, stdOut io.Writer, stdErr io.Writer) (*Process, error) {
	builder, err := exec.LookPath(c.Build.Name)
	if err != nil {
		return nil, ProgramNotFound(err)
	}

	return &Process{
		appError:      stdErr,
		appOutput:     stdOut,
		cancel:        nil,
		cmd:           nil,
		lastRestarted: time.Now(),
		runner: Executable{
			Program: c.Program.Name,
			Args:    c.Program.Args,
		},
		builder: Executable{
			Program: builder,
			Args:    c.Build.Args,
		},
		buildtOnce: false,
		processLog: logger,
	}, nil
}

func (p *Process) newCmd(e Executable) (*exec.Cmd, context.CancelFunc) {
	ctx, ctl := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, e.Program, e.Args...)
	cmd.Cancel = func() error {
		return cmd.Process.Kill()
	}

	// What about Stdin?
	cmd.Stdout = p.appOutput
	cmd.Stderr = p.appError

	return cmd, ctl
}

func (p *Process) firstBuild() error {
	program, err := exec.LookPath(p.runner.Program)
	if err != nil {
		return ProgramNotFound(err)
	}
	p.runner.Program = program
	p.buildtOnce = true
	return nil
}

func (p *Process) build() error {
	cmd, _ := p.newCmd(p.builder)
	t := time.Now()
	err := cmd.Run()
	dx := time.Now().Sub(t)
	if err == nil {
		p.processLog.Info("Build", "time", dx)
	} else {
		p.processLog.Warn("Build failed", "err", err)
		return ProcessBuildFailed
	}
	return nil
}

func (p *Process) Start() error {
	if err := p.build(); err != nil {
		return err
	}

	p.firstBuild()
	p.cmd, p.cancel = p.newCmd(p.runner)
	return p.cmd.Start()
}

func (p *Process) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
}

func (p *Process) restartable() bool {
	return p.lastRestarted.Add(1 * time.Second).After(time.Now())
}

func (p *Process) Restart() (error, bool) {
	if !p.buildtOnce {
		if p.cmd != nil || p.cancel != nil {
			panic("process exists even if buildtOnce is false. This can cause multiple process to spawn")
		}

		if err := p.build(); err != nil {
			return err, false
		}

		if err := p.firstBuild(); err != nil {
			return err, false
		}
	} else {
		if p.restartable() {
			return nil, false
		}

		if err := p.build(); err != nil {
			return err, false
		}

		p.cancel()
		err := p.cmd.Wait()
		if _, ok := err.(*exec.ExitError); !ok {
			return err, false
		}
	}

	p.cmd, p.cancel = p.newCmd(p.runner)
	p.lastRestarted = time.Now()

	p.processLog.Debug("Process restarted successfully")
	return p.cmd.Start(), true
}

func (p *Process) RestartWithArgs(args ...string) (error, bool) {
	if p.restartable() {
		return nil, false
	}
	p.runner.Args = args
	return p.Restart()
}
