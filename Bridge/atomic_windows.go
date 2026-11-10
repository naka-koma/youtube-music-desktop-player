package main

import (
	"syscall"
	"unsafe"
)

var moveFileExProc = syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")

func replaceFileAtomic(source, destination string) error {
	src, err := syscall.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	dst, err := syscall.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	const movefileReplaceExisting = 0x1
	const movefileWriteThrough = 0x8
	result, _, callErr := moveFileExProc.Call(uintptr(unsafe.Pointer(src)), uintptr(unsafe.Pointer(dst)), movefileReplaceExisting|movefileWriteThrough)
	if result == 0 {
		return callErr
	}
	return nil
}
