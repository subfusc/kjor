package main

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/subfusc/kjor/config"
)

type Event struct {
	FileName string // This is only the name as Fanotify does not give out full path without CAP_DAC_SEARCH_FILE
	Type     uint64
	When     time.Time
}

type FSWatcher struct {
	*fsnotify.Watcher
	events         chan Event
	ignorePatterns []*regexp.Regexp
}

func NewFSWatcher(c *config.Config) (*FSWatcher, error) {
	watcher, err := fsnotify.NewWatcher()

	ignorePatterns := make([]*regexp.Regexp, 0, len(c.Filewatcher.Ignore))
	for i, ire := range c.Filewatcher.Ignore {
		re, err := regexp.Compile(ire)
		if err != nil {
			return nil, fmt.Errorf("Failed to compile an IgnoreFile regexp, Ignore[%d]: [%w]", i, err)
		}
		ignorePatterns = append(ignorePatterns, re)
	}

	return &FSWatcher{
		Watcher:        watcher,
		events:         make(chan Event),
		ignorePatterns: ignorePatterns,
	}, err
}

func (fsw *FSWatcher) Close() error {
	return fsw.Watcher.Close()
}

func (fsw *FSWatcher) EventStream() chan Event {
	return fsw.events
}

func (fsw *FSWatcher) Ignored(pattern string) bool {
	for _, re := range fsw.ignorePatterns {
		if re.MatchString(pattern) {
			return true
		}
	}

	return false
}

func (fsw *FSWatcher) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case event, ok := <-fsw.Watcher.Events:
				if !ok {
					close(fsw.events)
					return
				}

				if !event.Op.Has(fsnotify.Chmod) && !fsw.Ignored(path.Base(event.Name)) {
					fsw.events <- Event{
						FileName: event.Name,
						Type:     uint64(event.Op),
						When:     time.Now(),
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (fsw *FSWatcher) Watch(path string) error {
	return fsw.Add(path)
}
