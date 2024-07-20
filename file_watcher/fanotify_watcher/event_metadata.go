package fanotify_watcher

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

type FanotifyEventMetadata unix.FanotifyEventMetadata

var sizeofFanotifyEventMetadata = unsafe.Sizeof(FanotifyEventMetadata{})

func (fem *FanotifyEventMetadata) MaskToString() []string {
	rval := make([]string, 0)
	maskMap := map[uint64]string{
		CREATE:      "CREATE",
		DELETE:      "DELETE",
		DELETE_SELF: "DELETE_SELF",
		RENAME:      "RENAME",
		MOVED_FROM:  "MOVED_FROM",
		MOVED_TO:    "MOVED_TO",
		MOVE_SELF:   "MOVE_SELF",
		MODIFY:      "MODIFY",
		CLOSE_WRITE: "CLOSE_WRITE",
	}
	for key, val := range maskMap {
		if (key & fem.Mask) != 0 {
			rval = append(rval, val)
		}
	}
	return rval
}

func (fem *FanotifyEventMetadata) MaskToDebugString() []string {
	rval := make([]string, 0)
	maskMap := map[uint64]string{
		unix.FAN_ACCESS:         "ACCESS",
		unix.FAN_OPEN:           "OPEN",
		unix.FAN_OPEN_EXEC:      "OPEN_EXEC",
		unix.FAN_ATTRIB:         "ATTRIB",
		unix.FAN_CREATE:         "CREATE",
		unix.FAN_DELETE:         "DELETE",
		unix.FAN_DELETE_SELF:    "DELETE_SELF",
		unix.FAN_FS_ERROR:       "FS_ERROR",
		unix.FAN_RENAME:         "RENAME",
		unix.FAN_MOVED_FROM:     "MOVED_FROM",
		unix.FAN_MOVED_TO:       "MOVED_TO",
		unix.FAN_MOVE_SELF:      "MOVE_SELF",
		unix.FAN_MODIFY:         "MODIFY",
		unix.FAN_CLOSE_WRITE:    "CLOSE_WRITE",
		unix.FAN_CLOSE_NOWRITE:  "CLOSE_NOWRITE",
		unix.FAN_Q_OVERFLOW:     "Q_OVERFLOW",
		unix.FAN_ACCESS_PERM:    "ACCESS_PERM",
		unix.FAN_OPEN_PERM:      "OPEN_PERM",
		unix.FAN_OPEN_EXEC_PERM: "OPEN_EXEC_PERM",
		unix.FAN_CLOSE:          "CLOSE",
		unix.FAN_MOVE:           "MOVE",
		unix.FAN_ONDIR:          "ON_DIR",
	}
	for key, val := range maskMap {
		if (key & fem.Mask) != 0 {
			rval = append(rval, val)
		}
	}
	return rval
}
