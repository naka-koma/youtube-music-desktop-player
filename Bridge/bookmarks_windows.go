package main

import (
	"errors"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

var (
	openBookmarkClipboard    = user32.NewProc("OpenClipboard")
	closeBookmarkClipboard   = user32.NewProc("CloseClipboard")
	getBookmarkClipboardData = user32.NewProc("GetClipboardData")
	bookmarkKernel           = syscall.NewLazyDLL("kernel32.dll")
	bookmarkGlobalLock       = bookmarkKernel.NewProc("GlobalLock")
	bookmarkGlobalUnlock     = bookmarkKernel.NewProc("GlobalUnlock")
	bookmarkGlobalSize       = bookmarkKernel.NewProc("GlobalSize")
	readBookmarkMemory       = bookmarkKernel.NewProc("ReadProcessMemory")
)

func readBookmarkClipboard() (string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	for attempt := 0; ; attempt++ {
		opened, _, _ := openBookmarkClipboard.Call(0)
		if opened != 0 {
			break
		}
		if attempt == 4 {
			return "", errors.New("clipboard unavailable")
		}
		time.Sleep(15 * time.Millisecond)
	}
	defer closeBookmarkClipboard.Call()
	const cfUnicodeText = 13
	handle, _, _ := getBookmarkClipboardData.Call(cfUnicodeText)
	if handle == 0 {
		return "", errors.New("clipboard has no text")
	}
	size, _, _ := bookmarkGlobalSize.Call(handle)
	if size < 2 || size > 65536 {
		return "", errors.New("clipboard text size is unsupported")
	}
	ptr, _, _ := bookmarkGlobalLock.Call(handle)
	if ptr == 0 {
		return "", errors.New("clipboard text unavailable")
	}
	defer bookmarkGlobalUnlock.Call(handle)
	buffer := make([]uint16, int(size/2))
	// The clipboard handle owns ptr. Copy the bounded data before releasing the lock.
	ok, _, _ := readBookmarkMemory.Call(^uintptr(0), ptr, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)*2), 0)
	if ok == 0 {
		return "", errors.New("clipboard text unreadable")
	}
	return syscall.UTF16ToString(buffer), nil
}
