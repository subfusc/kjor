package inotify_watcher

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/unix"
)

type InotifyEvent unix.InotifyEvent

func (ie *InotifyEvent) MaskToString() []string {
	rval := make([]string, 0)
	masks := map[uint]string{
		unix.IN_ACCESS:        "ACCESS",
		unix.IN_ATTRIB:        "ATTRIB",
		unix.IN_CLOSE_WRITE:   "CLOSE_WRITE",
		unix.IN_CLOSE_NOWRITE: "CLOSE_NOWRITE",
		unix.IN_CREATE:        "CREATE",
		unix.IN_DELETE:        "DELETE",
		unix.IN_DELETE_SELF:   "DELETE_SELF",
		unix.IN_MODIFY:        "MODIFY",
		unix.IN_MOVE_SELF:     "MOVE_SELF",
		unix.IN_MOVED_FROM:    "MOVED_FROM",
		unix.IN_MOVED_TO:      "MOVED_TO",
		unix.IN_OPEN:          "OPEN",
		unix.IN_MASK_ADD:      "MASK_ADD",
		unix.IN_MASK_CREATE:   "MASK_CREATE",
	}

	for m, s := range masks {
		if (m & uint(ie.Mask)) != 0 {
			rval = append(rval, s)
		}
	}

	return rval
}

type InotifyWatcher struct {
	inotifyFD   int
	eventStream io.ReadCloser
	pathToWD    map[string]int
	wdToPath    []string
}

var sizeOfInotifyEvent = uint32(unsafe.Sizeof(InotifyEvent{}))

func NewInotifyWatcher() (*InotifyWatcher, error) {
	fd, err := unix.InotifyInit()
	if err != nil {
		return nil, fmt.Errorf("Unable to open an Inotify descriptor: [%v]", err)
	}

	es := os.NewFile(uintptr(fd), "")

	return &InotifyWatcher{
		inotifyFD:   fd,
		eventStream: es,
		pathToWD:    make(map[string]int),
		wdToPath:    make([]string, 0),
	}, nil
}

func (iw *InotifyWatcher) watch(dirPath string) error {
	if iw.pathToWD[dirPath] != 0 {
		return nil
	}

	wd, err := unix.InotifyAddWatch(iw.inotifyFD, dirPath, unix.IN_MOVE|unix.IN_CLOSE_WRITE|unix.IN_CREATE|unix.IN_DELETE|unix.IN_DELETE_SELF)
	if err != nil {
		return fmt.Errorf("Unable to add Watch: [%v]", err)
	}
	iw.pathToWD[dirPath] = wd
	iw.wdToPath = append(iw.wdToPath, dirPath)
	return nil
}

func (iw *InotifyWatcher) watchTraverse(dirPath string) error {
	err := filepath.WalkDir(dirPath, func(p string, d os.DirEntry, e error) error {
		if d.IsDir() {
			if []rune(d.Name())[0] == '.' || iw.pathToWD[p] != 0 {
				return fs.SkipDir
			}

			return iw.watch(p)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("Failed to traverse dirPath \"%s\": [%v]", dirPath, err)
	}
	return nil
}

func (iw *InotifyWatcher) Watch(dirPath string) error {
	return iw.watchTraverse(dirPath)
}

func (iw *InotifyWatcher) Close() error {
	return iw.eventStream.Close()
}

func (iw *InotifyWatcher) Start() error {
	fmt.Println("Starting Inotify Watcher")
	buf := make([]byte, 4096)

	for {
		i, err := iw.eventStream.Read(buf)
		if err != nil {
			return err
		}

		un := uint32(0)
		ui := uint32(i)
		for un < ui {
			event := (*InotifyEvent)(unsafe.Pointer(&buf[un]))
			name := ""
			if event.Len > 0 {
				name = string(buf[un+sizeOfInotifyEvent : un+sizeOfInotifyEvent+event.Len])
			}
			fmt.Printf("Inotify: Got event: %+v Mask: %v [%s:%s]\n", event, event.MaskToString(), name, iw.wdToPath[event.Wd - 1])
			un += sizeOfInotifyEvent + event.Len
		}
	}
}
