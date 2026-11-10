package main

import (
	"fmt"
	"math"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	volumeMeterX      = 316.0
	volumeMeterWidth  = 76.0
	seekMeterX        = 108.0
	seekMeterWidth    = 264.0
	skinLogicalWidth  = 520.0
	leftMouseButton   = 0x01
	leftButtonPressed = 0x8000
)

type windowRect struct {
	Left, Top, Right, Bottom int32
}

var user32 = syscall.NewLazyDLL("user32.dll")
var enumWindowsProc = user32.NewProc("EnumWindows")
var getWindowTextProc = user32.NewProc("GetWindowTextW")
var getClassNameProc = user32.NewProc("GetClassNameW")
var getWindowRectProc = user32.NewProc("GetWindowRect")
var getCursorPosProc = user32.NewProc("GetCursorPos")
var getAsyncKeyStateProc = user32.NewProc("GetAsyncKeyState")

func ambientSkinRect() (windowRect, error) {
	var found windowRect
	callback := syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		className := make([]uint16, 128)
		classLen, _, _ := getClassNameProc.Call(hwnd, uintptr(unsafe.Pointer(&className[0])), uintptr(len(className)))
		if classLen == 0 || syscall.UTF16ToString(className) != "RainmeterMeterWindow" {
			return 1
		}
		title := make([]uint16, 1024)
		titleLen, _, _ := getWindowTextProc.Call(hwnd, uintptr(unsafe.Pointer(&title[0])), uintptr(len(title)))
		if titleLen == 0 || !strings.HasSuffix(strings.ToLower(syscall.UTF16ToString(title)), `\ambientmusic\ambientmusic.ini`) {
			return 1
		}
		_, _, _ = getWindowRectProc.Call(hwnd, uintptr(unsafe.Pointer(&found)))
		return 0
	})
	enumWindowsProc.Call(callback, 0)
	if found.Right <= found.Left {
		return windowRect{}, fmt.Errorf("AmbientMusic Rainmeter window was not found")
	}
	return found, nil
}

func startVolumeDrag(percent float64) error {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	window, err := ambientSkinRect()
	if err != nil {
		return err
	}
	scale := float64(window.Right-window.Left) / skinLogicalWidth
	if scale <= 0 {
		return fmt.Errorf("could not determine Rainmeter skin scale")
	}
	volumeDrag.Lock()
	volumeDrag.session++
	session := volumeDrag.session
	volumeDrag.active = true
	volumeDrag.percent = percent
	volumeDrag.left = float64(window.Left) + volumeMeterX*scale
	volumeDrag.width = volumeMeterWidth * scale
	volumeDrag.Unlock()
	go watchVolumeDrag(session)
	return nil
}

func startSeekDrag(percent float64) error {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	window, err := ambientSkinRect()
	if err != nil {
		return err
	}
	scale := float64(window.Right-window.Left) / skinLogicalWidth
	if scale <= 0 {
		return fmt.Errorf("could not determine Rainmeter skin scale")
	}
	seekDrag.Lock()
	seekDrag.session++
	session := seekDrag.session
	seekDrag.active = true
	seekDrag.percent = percent
	seekDrag.left = float64(window.Left) + seekMeterX*scale
	seekDrag.width = seekMeterWidth * scale
	seekDrag.Unlock()
	go watchSeekDrag(session)
	return nil
}

func stopSeekDrag() {
	seekDrag.Lock()
	seekDrag.active = false
	seekDrag.session++
	seekDrag.Unlock()
}

func watchSeekDrag(session uint64) {
	for {
		seekDrag.RLock()
		active := seekDrag.active && seekDrag.session == session
		left, width := seekDrag.left, seekDrag.width
		seekDrag.RUnlock()
		if !active {
			return
		}
		keyState, _, _ := getAsyncKeyStateProc.Call(leftMouseButton)
		if uint16(keyState)&uint16(leftButtonPressed) == 0 {
			seekDrag.Lock()
			if seekDrag.session == session {
				seekDrag.active = false
			}
			seekDrag.Unlock()
			return
		}
		var cursor struct{ X, Y int32 }
		if result, _, _ := getCursorPosProc.Call(uintptr(unsafe.Pointer(&cursor))); result != 0 && width > 0 {
			percent := math.Round((float64(cursor.X) - left) / width * 100)
			if percent < 0 {
				percent = 0
			}
			if percent > 100 {
				percent = 100
			}
			seekDrag.Lock()
			if seekDrag.session == session {
				seekDrag.percent = percent
			}
			seekDrag.Unlock()
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func stopVolumeDrag() {
	volumeDrag.Lock()
	volumeDrag.active = false
	volumeDrag.session++
	volumeDrag.Unlock()
}

func queueVolumeUpdate(update volumeUpdate) {
	select {
	case volumeUpdates <- update:
		return
	default:
	}
	select {
	case <-volumeUpdates:
	default:
	}
	select {
	case volumeUpdates <- update:
	default:
	}
}

func volumeCommandWorker() {
	for update := range volumeUpdates {
		volumeDrag.RLock()
		active := volumeDrag.active && volumeDrag.session == update.session
		volumeDrag.RUnlock()
		if !active {
			continue
		}
		resp, err := setVolume(update.percent)
		if err != nil {
			continue
		}
		resp.Body.Close()
		if update.percent > 0 {
			volumeState.Lock()
			volumeState.previous = update.percent
			volumeState.Unlock()
		}
	}
}

func watchVolumeDrag(session uint64) {
	lastQueued := -1.0
	lastQueueTime := time.Time{}
	for {
		volumeDrag.RLock()
		active := volumeDrag.active && volumeDrag.session == session
		left, width := volumeDrag.left, volumeDrag.width
		volumeDrag.RUnlock()
		if !active {
			return
		}
		keyState, _, _ := getAsyncKeyStateProc.Call(leftMouseButton)
		if uint16(keyState)&uint16(leftButtonPressed) == 0 {
			volumeDrag.Lock()
			if volumeDrag.session == session {
				volumeDrag.active = false
			}
			volumeDrag.Unlock()
			return
		}
		var cursor struct{ X, Y int32 }
		if result, _, _ := getCursorPosProc.Call(uintptr(unsafe.Pointer(&cursor))); result != 0 && width > 0 {
			percent := math.Round((float64(cursor.X) - left) / width * 100)
			if percent < 0 {
				percent = 0
			}
			if percent > 100 {
				percent = 100
			}
			volumeDrag.Lock()
			if volumeDrag.session == session {
				volumeDrag.percent = percent
			}
			volumeDrag.Unlock()
			if percent != lastQueued && time.Since(lastQueueTime) >= 80*time.Millisecond {
				queueVolumeUpdate(volumeUpdate{percent: percent, session: session})
				lastQueued, lastQueueTime = percent, time.Now()
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
}
