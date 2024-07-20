package fanotify_watcher

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"unsafe"

	"golang.org/x/sys/unix"
)

var (
	SizeofFsid                = unsafe.Sizeof(unix.Fsid{})
	SizeofFanotifyEventHeader = unsafe.Sizeof(FanotifyEventInfoHeader{})
)

type FanotifyEventInfoHeader struct {
	InfoType uint8
	Pad      uint8
	Len      uint16
}

type FanotifyEventInfoFid struct {
	Hdr           FanotifyEventInfoHeader
	Fsid          unix.Fsid
	FileHandle    unix.FileHandle
	handleContent []byte
	name          string
}

type FanotifyEventInfoPidfd struct {
	Hdr   FanotifyEventInfoHeader
	Pidfd int32
}

type FanotifyEventInfoError struct {
	Hdr        FanotifyEventInfoHeader
	Error      int32
	ErrorCount uint32
}

func NewEventInfo(pos uint32, buf []byte) (*FanotifyEventInfoHeader, *FanotifyEventInfoFid) {
	var infoFid *FanotifyEventInfoFid
	hdr := (*FanotifyEventInfoHeader)(unsafe.Pointer(&buf[pos]))

	if (hdr.InfoType&unix.FAN_EVENT_INFO_TYPE_FID) != 0 || (hdr.InfoType&unix.FAN_EVENT_INFO_TYPE_DFID) != 0 {
		fsidPos := pos + uint32(unsafe.Sizeof(*hdr))
		fileHandlePos := fsidPos + uint32(SizeofFsid)

		infoFid = &FanotifyEventInfoFid{
			Hdr:  *hdr,
			Fsid: *(*unix.Fsid)(unsafe.Pointer(&buf[fsidPos])),
		}

		var fileHandleType int32
		var fileHandleBytes uint32

		binary.Read(bytes.NewReader(buf[fileHandlePos:fileHandlePos+4]), binary.LittleEndian, &fileHandleBytes)
		binary.Read(bytes.NewReader(buf[fileHandlePos+4:fileHandlePos+8]), binary.LittleEndian, &fileHandleType)
		nameStart := fileHandlePos + 8 + fileHandleBytes
		infoFid.FileHandle = unix.NewFileHandle(fileHandleType, buf[fileHandlePos+8:nameStart])
		if (hdr.InfoType & unix.FAN_EVENT_INFO_TYPE_DFID_NAME) != 0 {
			nameBuf := bytes.NewBuffer(nil)
			for idx := nameStart; buf[idx] != 0; idx++ {
				nameBuf.WriteByte(buf[idx])
			}
			infoFid.name = nameBuf.String()
		}
	}

	return hdr, infoFid
}

func (feih *FanotifyEventInfoHeader) InfoTypeToString() []string {
	infoTypeMap := map[uint8]string{
		unix.FAN_EVENT_INFO_TYPE_FID:       "EVENT_INFO_TYPE_FID",
		unix.FAN_EVENT_INFO_TYPE_DFID:      "EVENT_INFO_TYPE_DFID",
		unix.FAN_EVENT_INFO_TYPE_DFID_NAME: "EVENT_INFO_TYPE_DFID_NAME",
		unix.FAN_EVENT_INFO_TYPE_PIDFD:     "EVENT_INFO_TYPE_PIDFD",
	}
	rval := []string{}
	for key, val := range infoTypeMap {
		if (key & feih.InfoType) != 0 {
			rval = append(rval, val)
		}
	}
	return rval
}

func (feif *FanotifyEventInfoFid) ReadHandle() error {
	efd, err := unix.OpenByHandleAt(unix.AT_FDCWD, feif.FileHandle, unix.O_RDONLY)
	if err != nil {
		fmt.Printf("Unable to open handle: %v\n", err)
		return err
	}

	link := fmt.Sprintf("/proc/self/fd/%d", efd)
	linkBuf := make([]byte, 64)
	n, err := unix.Readlink(link, linkBuf)
	if err != nil {
		return err
	}

	feif.handleContent = linkBuf[0:n]
	return nil
}

func (feif *FanotifyEventInfoFid) HandleAsString() string {
	if feif.handleContent == nil {
		return ""
	}
	return string(feif.handleContent)
}

func (feif *FanotifyEventInfoFid) Name() string {
	return feif.name
}
