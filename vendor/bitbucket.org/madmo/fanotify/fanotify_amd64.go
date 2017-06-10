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

package fanotify

import (
	"syscall"
	"unsafe"
)

// Add/Delete/Modify an Fanotify mark
func (nd *NotifyFD) Mark(flags int, mask uint64, dfd int, path string) error {
	_, _, errno := syscall.Syscall6(syscall.SYS_FANOTIFY_MARK, uintptr(nd.f.Fd()), uintptr(flags), uintptr(mask), uintptr(dfd), uintptr(unsafe.Pointer(syscall.StringBytePtr(path))), 0)

	var err error
	if errno != 0 {
		err = errno
	}

	return err
}
