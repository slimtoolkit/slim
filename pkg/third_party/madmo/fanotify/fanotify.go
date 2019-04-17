// Copyright (c) 2012, Moritz Bitsch <moritzbitsch@googlemail.com>
//
// Permission to use, copy, modify, and/or distribute this software for any
// purpose with or without fee is hereby granted, provided that the above
// copyright notice and this permission notice appear in all copies.
//
// THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
// WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
// MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
// ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
// WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
// ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
// OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.

// The fanotify package provides a simple fanotify api
package fanotify

import (
	"bufio"
	"encoding/binary"
	"os"
	"syscall"
)

// Flags used as first parameter to Initiliaze
const (
	/* flags used for fanotify_init() */
	FAN_CLOEXEC  = 0x00000001
	FAN_NONBLOCK = 0x00000002

	/* These are NOT bitwise flags.  Both bits are used togther.  */
	FAN_CLASS_NOTIF       = 0x00000000
	FAN_CLASS_CONTENT     = 0x00000004
	FAN_CLASS_PRE_CONTENT = 0x00000008

	FAN_ALL_CLASS_BITS = FAN_CLASS_NOTIF |
		FAN_CLASS_CONTENT |
		FAN_CLASS_PRE_CONTENT

	FAN_UNLIMITED_QUEUE = 0x00000010
	FAN_UNLIMITED_MARKS = 0x00000020

	FAN_ALL_INIT_FLAGS = FAN_CLOEXEC |
		FAN_NONBLOCK |
		FAN_ALL_CLASS_BITS |
		FAN_UNLIMITED_QUEUE |
		FAN_UNLIMITED_MARKS
)

// Flags used for the Mark Method
const (
	/* flags used for fanotify_modify_mark() */
	FAN_MARK_ADD                 = 0x00000001
	FAN_MARK_REMOVE              = 0x00000002
	FAN_MARK_DONT_FOLLOW         = 0x00000004
	FAN_MARK_ONLYDIR             = 0x00000008
	FAN_MARK_MOUNT               = 0x00000010
	FAN_MARK_IGNORED_MASK        = 0x00000020
	FAN_MARK_IGNORED_SURV_MODIFY = 0x00000040
	FAN_MARK_FLUSH               = 0x00000080

	FAN_ALL_MARK_FLAGS = FAN_MARK_ADD |
		FAN_MARK_REMOVE |
		FAN_MARK_DONT_FOLLOW |
		FAN_MARK_ONLYDIR |
		FAN_MARK_MOUNT |
		FAN_MARK_IGNORED_MASK |
		FAN_MARK_IGNORED_SURV_MODIFY |
		FAN_MARK_FLUSH
)

// Event types
const (
	FAN_ACCESS        = 0x00000001 /* File was accessed */
	FAN_MODIFY        = 0x00000002 /* File was modified */
	FAN_CLOSE_WRITE   = 0x00000008 /* Writtable file closed */
	FAN_CLOSE_NOWRITE = 0x00000010 /* Unwrittable file closed */
	FAN_OPEN          = 0x00000020 /* File was opened */

	FAN_Q_OVERFLOW = 0x00004000 /* Event queued overflowed */

	FAN_OPEN_PERM   = 0x00010000 /* File open in perm check */
	FAN_ACCESS_PERM = 0x00020000 /* File accessed in perm check */

	FAN_ONDIR = 0x40000000 /* event occurred against dir */

	FAN_EVENT_ON_CHILD = 0x08000000 /* interested in child events */

	/* helper events */
	FAN_CLOSE = FAN_CLOSE_WRITE | FAN_CLOSE_NOWRITE /* close */

	/*
	 * All of the events - we build the list by hand so that we can add flags in
	 * the future and not break backward compatibility.  Apps will get only the
	 * events that they originally wanted.  Be sure to add new events here!
	 */
	FAN_ALL_EVENTS = FAN_ACCESS |
		FAN_MODIFY |
		FAN_CLOSE |
		FAN_OPEN

		/*
		 * All events which require a permission response from userspace
		 */
	FAN_ALL_PERM_EVENTS = FAN_OPEN_PERM |
		FAN_ACCESS_PERM

	FAN_ALL_OUTGOING_EVENTS = FAN_ALL_EVENTS |
		FAN_ALL_PERM_EVENTS |
		FAN_Q_OVERFLOW

	FANOTIFY_METADATA_VERSION = 3

	FAN_ALLOW = 0x01
	FAN_DENY  = 0x02
	FAN_NOFD  = -1
)

// Internal eventMetadata struct, used for fanotify comm
type eventMetadata struct {
	Len         uint32
	Version     uint8
	Reserved    uint8
	MetadataLen uint16
	Mask        uint64
	Fd          int32
	Pid         int32
}

// Internal response struct, used for fanotify comm
type response struct {
	Fd       int32
	Response uint32
}

// Event struct returned from NotifyFD.GetEvent
//
// The File member needs to be Closed after usage, to prevent
// an Fd leak
type EventMetadata struct {
	Len         uint32
	Version     uint8
	Reserved    uint8
	MetadataLen uint16
	Mask        uint64
	File        *os.File
	Pid         int32
}

// A notify handle, used by all notify functions
type NotifyFD struct {
	f *os.File
	r *bufio.Reader
}

// Initialize the notify support
func Initialize(faflags, openflags int) (*NotifyFD, error) {
	fd, _, errno := syscall.Syscall(syscall.SYS_FANOTIFY_INIT, uintptr(faflags), uintptr(openflags), uintptr(0))

	var err error
	if errno != 0 {
		err = errno
	}

	f := os.NewFile(fd, "")
	return &NotifyFD{f, bufio.NewReader(f)}, err
}

// Get an event from the fanotify handle
func (nd *NotifyFD) GetEvent() (*EventMetadata, error) {
	ev := &eventMetadata{}

	err := binary.Read(nd.r, binary.LittleEndian, ev)
	if err != nil {
		return nil, err
	}

	res := &EventMetadata{ev.Len, ev.Version, ev.Reserved, ev.MetadataLen, ev.Mask, os.NewFile(uintptr(ev.Fd), ""), ev.Pid}

	return res, nil
}

// Send an allow message back to fanotify, used for permission checks
// If allow is set to true, access is granted
func (nd *NotifyFD) Response(ev *EventMetadata, allow bool) error {
	resp := &response{Fd: int32(ev.File.Fd())}

	if allow {
		resp.Response = FAN_ALLOW
	} else {
		resp.Response = FAN_DENY
	}

	return binary.Write(nd.f, binary.LittleEndian, resp)
}
