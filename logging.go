package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"
)

type KjorOutput struct {
	Build           slog.Handler
	ProgramStandard io.Writer
	ProgramError    io.Writer
	SSE             slog.Handler
	FileWatcher     slog.Handler
}

func FancyKjorLogger(buildLevel slog.Level, SSELevel slog.Level, fileWatcherLevel slog.Level) *KjorOutput {
	return &KjorOutput{
		Build: NewTerminalLoggerWithName(os.Stdout, buildLevel, "Prc", Color{0,0,0}, Color{0,255,0}),
		ProgramStandard: NewAppProcessWriter(os.Stdout),
		ProgramError: NewAppProcessWriter(os.Stdout),
		SSE: NewTerminalLoggerWithName(os.Stdout, SSELevel, "SSE", Color{0,0,0}, Color{255,0,0}),
		FileWatcher: NewTerminalLoggerWithName(os.Stdout, fileWatcherLevel, "FWt", Color{0,0,0}, Color{0,0,255}),
	}
}

func UnfancyKjorLogger(buildLevel slog.Level, SSELevel slog.Level, fileWatcherLevel slog.Level) *KjorOutput {
	return &KjorOutput{
		Build: slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{AddSource: false, Level: buildLevel}),
		ProgramStandard: os.Stdout,
		ProgramError: os.Stderr,
		SSE: slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{AddSource: false, Level: SSELevel}),
		FileWatcher: slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{AddSource: false, Level: fileWatcherLevel}),
	}
}

type TerminalLogger struct {
	streamName string
	level      slog.Level
	out        io.Writer
}

func NewTerminalLoggerWithName(out io.Writer, level slog.Level, name string, fg Color, bg Color) *TerminalLogger {
	logger := &TerminalLogger{
		level: level,
		out:   out,
	}
	logger.WithStreamName(name, fg, bg)
	return logger
}

func NewTerminalLogger(out io.Writer, level slog.Level) *TerminalLogger {
	return &TerminalLogger{
		level: level,
		out:   out,
	}
}

func (tl *TerminalLogger) lvlFormat(lvl slog.Level) (string, string) {
	cb := NewAnsiColorBuilder(lvl.String())
	arrow := NewAnsiColorBuilder("🭬")
	switch lvl {
	case slog.LevelDebug:
		cb.Colorize(Color{0, 0, 0}, Color{255, 255, 255})
		arrow.Fg(Color{255,255,255})
	case slog.LevelInfo:
		cb.Colorize(Color{255, 255, 255}, Color{0, 0, 255})
		arrow.Fg(Color{0,0,255})
	case slog.LevelWarn:
		cb.Colorize(Color{0, 0, 0}, Color{255, 255, 0})
		arrow.Fg(Color{255,255,0})
	case slog.LevelError:
		cb.Colorize(Color{255, 255, 255}, Color{255, 0, 0})
		arrow.Fg(Color{255,0,0})
	default:
		cb.Colorize(Color{255, 255, 255}, Color{102, 51, 0})
		arrow.Fg(Color{102,51,0})
	}
	return cb.String(), arrow.String()
}

func (tl *TerminalLogger) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= tl.level
}

func (tl *TerminalLogger) Handle(ctx context.Context, r slog.Record) error {
	buf := bytes.NewBuffer(nil)
	ti := " " + r.Time.Format(time.DateTime + ".000") + " "
	lvl, arr := tl.lvlFormat(r.Level)
	_, err := fmt.Fprintf(buf, "%s%s%s%s %s [", tl.streamName, ti, lvl, arr, r.Message)
	if err != nil {
		return err
	}

	i := 0
	r.Attrs(func(a slog.Attr) bool {
		if i > 0 {
			_, err = fmt.Fprintf(buf, " %s=%s", a.Key, a.Value)
		} else {
			_, err = fmt.Fprintf(buf, "%s=%s", a.Key, a.Value)
		}
		i++

		return true
	})

	fmt.Fprintln(buf, "]")
	i, err = tl.out.Write(buf.Bytes())

	if i != buf.Len() {
		return fmt.Errorf("TerminalLogger: Unable to write entire buffer to out\n")
	}

	return err
}

func (tl *TerminalLogger) WithAttrs(attrs []slog.Attr) slog.Handler {
	// Not supported yet
	return tl
}

func (tl *TerminalLogger) WithGroup(name string) slog.Handler {
	// Not supported yet
	return tl
}

func (tl *TerminalLogger) WithStreamName(name string, fg Color, bg Color) {
	cb := NewAnsiColorBuilder(name)
	cb.Colorize(fg, bg)
	tl.streamName = cb.String()
}
