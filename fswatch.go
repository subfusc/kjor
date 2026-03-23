package main

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
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
	events                 chan Event
	ignorePatternsFile     []*regexp.Regexp
	ignorePatternsFullPath []*regexp.Regexp
}

func NewFSWatcher(c *config.Config) (*FSWatcher, error) {
	watcher, err := fsnotify.NewWatcher()

	ignorePatternsFile := make([]*regexp.Regexp, 0, len(c.Filewatcher.IgnoreFile))
	for i, ire := range c.Filewatcher.IgnoreFile {
		re, err := regexp.Compile(ire)
		if err != nil {
			return nil, fmt.Errorf("Failed to compile an IgnoreFile regexp, Ignore[%d]: [%w]", i, err)
		}
		ignorePatternsFile = append(ignorePatternsFile, re)
	}

	ignorePatternsFullPath := make([]*regexp.Regexp, 0, len(c.Filewatcher.IgnoreFullPath))
	for i, ire := range c.Filewatcher.IgnoreFullPath {
		re, err := regexp.Compile(ire)
		if err != nil {
			return nil, fmt.Errorf("Failed to compile an IgnoreFullPath regexp, Ignore[%d]: [%w]", i, err)
		}
		ignorePatternsFullPath = append(ignorePatternsFullPath, re)
	}

	return &FSWatcher{
		Watcher:        watcher,
		events:         make(chan Event),
		ignorePatternsFile: ignorePatternsFile,
		ignorePatternsFullPath: ignorePatternsFullPath,
	}, err
}

func (fsw *FSWatcher) EventStream() chan Event {
	return fsw.events
}

func (fsw *FSWatcher) Ignored(file, fullpath string) bool {
	for _, re := range fsw.ignorePatternsFile {
		if re.MatchString(file) {
			return true
		}
	}

	for _, re := range fsw.ignorePatternsFullPath {
		if re.MatchString(fullpath) {
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

				if !event.Op.Has(fsnotify.Chmod) && !fsw.Ignored(path.Base(event.Name), event.Name) {
					fsw.events <- Event{
						FileName: event.Name,
						Type:     uint64(event.Op),
						When:     time.Now(),
					}
				}
			case <-ctx.Done():
				fsw.Watcher.Close()
				return
			}
		}
	}()
}

// Watch adds the directory and all the sub directories to watch for events
func (fsw *FSWatcher) Watch(path string) error {
	filepath.WalkDir(path, func(path string, entry fs.DirEntry, err error) error {
		if entry.IsDir() {
			return fsw.Add(path)
		}

		return nil
	})

	return nil
}
