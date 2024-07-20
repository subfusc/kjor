package common

import "time"

type Event struct {
	FileName string // This is only the name as Fanotify does not give out full path without CAP_DAC_SEARCH_FILE
	Type     uint64
	When     time.Time
}
