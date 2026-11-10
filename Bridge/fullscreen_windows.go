package main

import (
	"fmt"
	"net/http"
	"unsafe"
)

var physicalCursorPos = user32.NewProc("GetPhysicalCursorPos")

func fullscreenPointer(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var point struct{ X, Y int32 }
	ok, _, _ := physicalCursorPos.Call(uintptr(unsafe.Pointer(&point)))
	if ok == 0 {
		return
	}
	button, _, _ := getAsyncKeyStateProc.Call(leftMouseButton)
	fmt.Fprintf(w, "%d|%d|%t", point.X, point.Y, button&leftButtonPressed != 0)
}
