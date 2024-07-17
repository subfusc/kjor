package fanotify_watcher

import (
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

func CapabilityDacReadSearch() bool {
	hdr := &unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3, Pid: int32(os.Getpid())}
	data := &unix.CapUserData{}
	if err := unix.Capget(hdr, data); err != nil {
		slog.Warn("Error in Capget", "err", err)
		return false
	}
	return data.Permitted&unix.CAP_DAC_READ_SEARCH != 0
}

func FanotifyVersion() int {
	return unix.FANOTIFY_METADATA_VERSION
}

func IsSupported() bool {
	return runtime.GOOS == "linux" && unix.FANOTIFY_METADATA_VERSION == 3
}

type FaNotifyWatcher struct {
	EventStream chan Event

	ableToOpenFid    bool
	eventReader      io.ReadCloser
	eventTypes       uint
	fanFd            int
	ignoredFileNames []*regexp.Regexp
	path             string
	watchedDir       []string
}

func NewFaNotifyWatcher() (*FaNotifyWatcher, error) {
	fw := &FaNotifyWatcher{
		EventStream:      make(chan Event, 30),
		eventTypes:       ALL,
		ableToOpenFid:    CapabilityDacReadSearch(),
		watchedDir:       make([]string, 0),
		ignoredFileNames: make([]*regexp.Regexp, 0),
	}
	return fw, fw.initialize()
}

func (fw *FaNotifyWatcher) Watch(dirPath string) error {
	if err := fw.watchSubDirectories(dirPath); err != nil {
		return fmt.Errorf("Unable to add dirPath %s: [%v]", dirPath, err)
	}

	return nil
}

func (fw *FaNotifyWatcher) Close() {
	fw.eventReader.Close()
	unix.Close(fw.fanFd)
}

func (fw *FaNotifyWatcher) IgnoreFileNameMatching(matcher *regexp.Regexp) {
	fw.ignoredFileNames = append(fw.ignoredFileNames, matcher)
}

func (fw *FaNotifyWatcher) addDirToNotifyGroup(dirPath string) error {
	// Note: From the man pages it says
	// "If pathname is NULL, and dirfd takes the special value AT_FDCWD, the current working directory is to be marked."
	// But I only get ERRNO EBADF when trying that. This is not exclusive to the Go Implementation as it seem to behave
	// the same way in C. Marking The current directory is done with the string "."

	err := unix.FanotifyMark(
		fw.fanFd,
		unix.FAN_MARK_ADD|unix.FAN_MARK_ONLYDIR,
		uint64(fw.eventTypes|unix.FAN_EVENT_ON_CHILD|unix.FAN_ONDIR),
		unix.AT_FDCWD,
		dirPath,
	)

	if err != nil {
		return fmt.Errorf("Unable to watch path (%s): [%w]", dirPath, err)
	} else {
		fw.watchedDir = append(fw.watchedDir, dirPath)
		return err
	}
}

func (fw *FaNotifyWatcher) reInitialize() error {
	fw.Close()
	if err := fw.initialize(); err != nil {
		return err
	}

	for _, dir := range fw.watchedDir {
		if err := fw.Watch(dir); err != nil {
			slog.Warn("Failed to watch directory", "dir", dir)
		}
	}

	return nil
}

func (fw *FaNotifyWatcher) initialize() error {
	fd, err := unix.FanotifyInit(
		unix.FAN_CLASS_NOTIF|unix.FAN_REPORT_FID|unix.FAN_REPORT_DIR_FID|unix.FAN_REPORT_DFID_NAME,
		unix.O_RDONLY|unix.O_LARGEFILE,
	)

	if err != nil {
		return fmt.Errorf("Failed to init: [%v]", err)
	}

	eventReader := os.NewFile(uintptr(fd), "")

	if err != nil {
		eventReader.Close()
		return fmt.Errorf("Error opening mountFD: [%v]\n", err)
	}

	fw.fanFd = fd
	fw.eventReader = eventReader
	return nil
}

func (fw *FaNotifyWatcher) watchSubDirectories(dirPath string) error {
	return filepath.WalkDir(dirPath, func(cPath string, d fs.DirEntry, err error) error {
		// Note: Random segfault after delete and mkdir. Is WalkDir cached so deleted dir still exist?
		if d.IsDir() {
			runeName := []rune(d.Name())
			if len(runeName) > 2 && runeName[0] == '.' && runeName[1] != '/' {
				// Naive Unix Hidden File skip.
				return fs.SkipDir
			}

			return fw.addDirToNotifyGroup(cPath)
		}

		return nil
	})
}

func (fw *FaNotifyWatcher) Start() error {
	for {
		buf := make([]byte, 4096)

	QueueWatcher:
		for {
			n, err := fw.eventReader.Read(buf)
			if err != nil {
				return err
			}

			idx := uint32(0)
			un := uint32(n)

			for idx < un {
				event := (*FanotifyEventMetadata)(unsafe.Pointer(&buf[idx]))
				slog.Debug("Inbound Fanotify event", "Event", event, "Mask", event.MaskToDebugString())
				if (event.Mask&unix.FAN_ONDIR) != 0 && (event.Mask&unix.FAN_CREATE) != 0 {
					break QueueWatcher
				}

				fileName := ""
				fullName := ""

				var hdr *FanotifyEventInfoHeader
				var ei *FanotifyEventInfoFid

				for eidx := idx + uint32(event.Metadata_len); eidx < (idx + uint32(event.Event_len)); eidx += uint32(hdr.Len) {
					hdr, ei = NewEventInfo(eidx, buf)
					if ei == nil {
						continue
					}

					if fw.ableToOpenFid {
						ei.ReadHandle()
					}

					// A note on Fanotify events and names.
					// An event in fanotify on files will usually include 2 event info elements. One for the dir, and one for the file. But the
					// order is not specified. It doesn't really matter anyways, since if we have CAP_DAC_SEARCH_FILE capabilites we get the
					// full name from either, but without it we only get the filename from the directory targeted event, and then only the file name.
					// See: `$ man fanotify` for more info
					switch {
					case (ei.Hdr.InfoType&unix.FAN_EVENT_INFO_TYPE_DFID) != 0 && (ei.Hdr.InfoType&unix.FAN_EVENT_INFO_TYPE_DFID_NAME) != 0:
						if len(fullName) == 0 {
							fullName = path.Join(ei.HandleAsString(), ei.Name())
						}
						fileName = ei.Name()
					case len(fullName) == 0 && (ei.Hdr.InfoType&unix.FAN_EVENT_INFO_TYPE_FID) != 0 && (ei.Hdr.InfoType&unix.FAN_EVENT_INFO_TYPE_DFID) != 0:
						fullName = ei.HandleAsString()
					}

					slog.Debug("Inbound EventInfo", "EvendInfo", ei, "Type", ei.Hdr.InfoTypeToString(), "Handle", ei.HandleAsString())
				}

				if !regexpAny(fw.ignoredFileNames, fileName) {
					fw.EventStream <- Event{FileName: fullName, Type: event.Mask, When: time.Now()}
				}
				idx += event.Event_len
			}
		}
		slog.Warn("ReInitializing FaNotifyWatcher")
		fw.reInitialize()
	}
}
