package fanotify_watcher

import (
	"fmt"
	"time"

	"golang.org/x/sys/unix"
)

const (
	CREATE      = unix.FAN_CREATE
	DELETE      = unix.FAN_DELETE
	DELETE_SELF = unix.FAN_DELETE_SELF
	RENAME      = unix.FAN_RENAME
	MOVED_FROM  = unix.FAN_MOVED_FROM
	MOVED_TO    = unix.FAN_MOVED_TO
	MOVE_SELF   = unix.FAN_MOVE_SELF
	MODIFY      = unix.FAN_MODIFY
	CLOSE_WRITE = unix.FAN_CLOSE_WRITE
	ALL         = CREATE | DELETE | DELETE_SELF | RENAME | MOVED_FROM | MOVED_TO | MOVE_SELF | MODIFY | CLOSE_WRITE
)

type Event struct {
	FileName string // This is only the name as Fanotify does not give out full path without CAP_DAC_SEARCH_FILE
	Type     uint64
	When     time.Time
}

func (e Event) TypeToString() []string {
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
		if (key & e.Type) != 0 {
			rval = append(rval, val)
		}
	}
	return rval
}

func (e Event) String() string {
	return fmt.Sprintf("Event{FileName:%s Type:%v When:%s}", e.FileName, e.TypeToString(), e.When)
}
