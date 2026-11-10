package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

type fontPreference struct {
	face   string
	weight int
}

var fontSlots = []string{"TitleEN", "TitleJA", "OtherEN", "OtherJA"}

func defaultFont(slot string) fontPreference {
	weight := 400
	if strings.HasPrefix(slot, "Title") {
		weight = 600
	}
	return fontPreference{"Segoe UI", weight}
}

func validFontFace(face string) bool {
	return face != "" && !strings.ContainsAny(face, "\x00\r\n#[]|")
}

func readFontPreferences(path string) map[string]fontPreference {
	fonts := map[string]fontPreference{}
	data, err := os.ReadFile(path)
	if err != nil || len(data) < 2 || data[0] != 0xff || data[1] != 0xfe {
		return fonts
	}
	units := make([]uint16, (len(data)-2)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(data[2+i*2:])
	}
	fields := map[string]string{}
	for _, line := range strings.Split(string(utf16.Decode(units)), "\n") {
		if key, value, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
			fields[key] = value
		}
	}
	for _, slot := range fontSlots {
		face := fields[slot+"Face"]
		weight, _ := strconv.Atoi(fields[slot+"Weight"])
		if validFontFace(face) && weight >= 1 && weight <= 999 {
			fonts[slot] = fontPreference{face, weight}
		}
	}
	return fonts
}

func writeFontPreferences(path string, fonts map[string]fontPreference) error {
	var b strings.Builder
	b.WriteString("[Variables]\r\n")
	for _, slot := range fontSlots {
		font, ok := fonts[slot]
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "%sFace=%s\r\n%sWeight=%d\r\n", slot, font.face, slot, font.weight)
		if strings.HasPrefix(slot, "Title") {
			fmt.Fprintf(&b, "%sFullWeight=%d\r\n", slot, font.weight)
		} else {
			fmt.Fprintf(&b, "%sMono=%s\r\n", slot, font.face)
			for _, variant := range []string{"Medium", "Semibold", "Bold", "Light"} {
				fmt.Fprintf(&b, "%s%s=%d\r\n", slot, variant, font.weight)
			}
		}
	}
	data := []byte{0xff, 0xfe}
	for _, unit := range utf16.Encode([]rune(b.String())) {
		data = binary.LittleEndian.AppendUint16(data, unit)
	}
	return saveBytesAtomic(path, data)
}

type logFontW struct {
	Height, Width, Escapement, Orientation, Weight                                              int32
	Italic, Underline, StrikeOut, CharSet, OutPrecision, ClipPrecision, Quality, PitchAndFamily byte
	FaceName                                                                                    [32]uint16
}

type chooseFontW struct {
	Size                                            uint32
	Owner, DC                                       uintptr
	Font                                            *logFontW
	PointSize                                       int32
	Flags, RGB                                      uint32
	CustomData, Hook, TemplateName, Instance, Style uintptr
	FontType, Padding                               uint16
	SizeMin, SizeMax                                int32
}

func rainmeterFontOwner() uintptr {
	var owner uintptr
	cb := syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		var name [512]uint16
		getClassNameProc.Call(hwnd, uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)))
		if syscall.UTF16ToString(name[:]) != "RainmeterMeterWindow" {
			return 1
		}
		getWindowTextProc.Call(hwnd, uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)))
		if !isPlayerSkinWindow(syscall.UTF16ToString(name[:])) {
			return 1
		}
		owner = hwnd
		return 0
	})
	enumWindowsProc.Call(cb, 0)
	return owner
}

func chooseFont(font fontPreference) (fontPreference, bool) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	logFont := logFontW{Height: -16, Weight: int32(font.weight), CharSet: 1}
	copy(logFont.FaceName[:], utf16.Encode([]rune(font.face)))
	dialog := chooseFontW{Owner: rainmeterFontOwner(), Font: &logFont, Flags: 0x00000001 | 0x00000040 | 0x00010000}
	dialog.Size = uint32(unsafe.Sizeof(dialog))
	choose := syscall.NewLazyDLL("comdlg32.dll").NewProc("ChooseFontW")
	ok, _, _ := choose.Call(uintptr(unsafe.Pointer(&dialog)))
	runtime.KeepAlive(&logFont)
	if ok == 0 {
		return fontPreference{}, false
	}
	result := fontPreference{syscall.UTF16ToString(logFont.FaceName[:]), int(logFont.Weight)}
	if result.weight < 1 || result.weight > 999 {
		result.weight = 400
	}
	return result, validFontFace(result.face)
}

func handleFontCommand(args []string) bool {
	if len(args) == 0 || (args[0] != "font-pick" && args[0] != "font-reset") {
		return false
	}
	exe, err := os.Executable()
	if err != nil {
		return true
	}
	path := filepath.Join(filepath.Dir(exe), "..", "Fonts.User.inc")
	fonts := map[string]fontPreference{}
	if args[0] == "font-pick" {
		if len(args) != 2 {
			return true
		}
		slot := args[1]
		valid := false
		for _, key := range fontSlots {
			if key == slot {
				valid = true
				break
			}
		}
		if !valid {
			return true
		}
		fonts = readFontPreferences(path)
		current, ok := fonts[slot]
		if !ok {
			current = defaultFont(slot)
		}
		picked, ok := chooseFont(current)
		if !ok {
			return true
		}
		fonts[slot] = picked
	}
	if writeFontPreferences(path, fonts) == nil {
		fmt.Print("changed")
	} else {
		fmt.Print("failed")
	}
	return true
}
