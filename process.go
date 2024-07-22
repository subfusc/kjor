package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"github.com/subfusc/kjor/config"
)

type Executable struct {
	Program string
	Args    []string
}

type Process struct {
	cancel        context.CancelFunc
	cmd           *exec.Cmd
	lastRestarted time.Time
	builder       Executable
	runner        Executable
	firstBuild    bool
}

func ProgramNotFound(err error) error {
	return fmt.Errorf("Failed to find program: [%v]", err)
}

func NewProcess(c *config.Config) (*Process, error) {
	builder, err := exec.LookPath(c.Build.Name)
	if err != nil {
		return nil, ProgramNotFound(err)
	}

	return &Process{
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
		firstBuild: true,
	}, nil
}

func (p *Process) newCmd(e Executable) (*exec.Cmd, context.CancelFunc) {
	ctx, ctl := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, e.Program, e.Args...)
	cmd.Cancel = func() error {
		return cmd.Process.Kill()
	}

	// What about Stdin?
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd, ctl
}

func (p *Process) build() error {
	cmd, _ := p.newCmd(p.builder)
	t := time.Now()
	err := cmd.Run()
	dx := time.Now().Sub(t)
	slog.Info("Build", "time", dx)
	return err
}

func (p *Process) Start() error {
	if err := p.build(); err != nil {
		return err
	}
	if p.firstBuild {
		program, err := exec.LookPath(p.runner.Program)
		if err != nil {
			return ProgramNotFound(err)
		}
		p.runner.Program = program
		p.firstBuild = false
	}

	p.cmd, p.cancel = p.newCmd(p.runner)
	return p.cmd.Start()
}

func (p *Process) Stop() {
	p.cancel()
}

func (p *Process) restartable() bool {
	return p.lastRestarted.Add(1 * time.Second).After(time.Now())
}

func (p *Process) Restart() (error, bool) {
	if p.restartable() {
		return nil, false
	}

	p.build()

	p.cancel()
	err := p.cmd.Wait()
	if _, ok := err.(*exec.ExitError); !ok {
		return err, false
	}

	p.cmd, p.cancel = p.newCmd(p.runner)
	p.lastRestarted = time.Now()

	return p.cmd.Start(), true
}

func (p *Process) RestartWithArgs(args ...string) (error, bool) {
	if p.restartable() {
		return nil, false
	}
	p.runner.Args = args
	return p.Restart()
}
