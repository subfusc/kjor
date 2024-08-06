package file_watcher

import (
	"log/slog"

	"github.com/subfusc/kjor/config"
	"github.com/subfusc/kjor/file_watcher/common"
	"github.com/subfusc/kjor/file_watcher/fanotify_watcher"
	"github.com/subfusc/kjor/file_watcher/inotify_watcher"
)
type FileWatcher interface {
	Close() error
	EventStream() chan common.Event
	Start() error
	Watch(path string) error
}

func NewFileWatcher(c *config.Config, logger *slog.Logger) (FileWatcher, error) {
	switch c.Filewatcher.Backend {
	case "fanotify":
		return fanotify_watcher.NewFaNotifyWatcher(c, logger)
	case "inotify":
		return inotify_watcher.NewInotifyWatcher(c, logger)
	default:
		slog.Info("FileWatch backend not configured, using Inotify as fallback")
		return inotify_watcher.NewInotifyWatcher(c, logger)
	}
}
