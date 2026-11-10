package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"
)

func TestFullscreenArtPairKeepsOriginalAndProducesScaledBlur(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 640, 320))
	for y := 0; y < 320; y++ {
		for x := 0; x < 640; x++ {
			if x < 320 {
				src.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
			} else {
				src.SetRGBA(x, y, color.RGBA{0, 0, 0, 255})
			}
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, src); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	pair, err := saveFullscreenArtPair(dir, encoded.Bytes())
	if err != nil || pair.width != 640 || pair.height != 320 {
		t.Fatalf("unexpected original artwork descriptor: %+v (%v)", pair, err)
	}
	original, err := os.ReadFile(filepath.Join(dir, pair.original))
	if err != nil || !bytes.Equal(original, encoded.Bytes()) {
		t.Fatal("fullscreen original must remain untouched")
	}
	f, err := os.Open(filepath.Join(dir, pair.blur))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	blur, err := png.Decode(f)
	if err != nil || blur.Bounds().Size() != image.Pt(480, 240) {
		t.Fatalf("unexpected blurred artwork dimensions: %v (%v)", blur, err)
	}
	red, _, _, alpha := blur.At(240, 120).RGBA()
	if red < 20000 || red > 45000 || alpha != 65535 {
		t.Fatalf("expected a softened edge over an opaque background, got red=%d alpha=%d", red, alpha)
	}
}

func TestFontPreferencesKeepUnicodeAndSeparateSlots(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Fonts.User.inc")
	want := fontPreference{"ＭＳ ゴシック", 700}
	if err := writeFontPreferences(path, map[string]fontPreference{"TitleJA": want}); err != nil {
		t.Fatal(err)
	}
	fonts := readFontPreferences(path)
	if len(fonts) != 1 || fonts["TitleJA"] != want {
		t.Fatalf("font settings did not round trip: %+v", fonts)
	}
	if err := writeFontPreferences(path, nil); err != nil {
		t.Fatal(err)
	}
	if len(readFontPreferences(path)) != 0 {
		t.Fatal("reset did not clear preferences")
	}
}

func TestChooseFontWindowsStructureLayout(t *testing.T) {
	if unsafe.Sizeof(logFontW{}) != 92 {
		t.Fatalf("invalid LOGFONTW size: %d", unsafe.Sizeof(logFontW{}))
	}
	want := uintptr(60)
	if unsafe.Sizeof(uintptr(0)) == 8 {
		want = 104
	}
	if unsafe.Sizeof(chooseFontW{}) != want {
		t.Fatalf("invalid CHOOSEFONTW size: %d", unsafe.Sizeof(chooseFontW{}))
	}
}

func TestFullscreenStatePreservesJapaneseAndEscapesSeparators(t *testing.T) {
	fullscreenCurrent.Lock()
	previous := fullscreenCurrent.snapshot
	fullscreenCurrent.snapshot = skinSnapshot{Title: "Ab|日本語", Author: "Artist", Position: "0:47 / 2:58", Playing: true}
	fullscreenCurrent.Unlock()
	t.Cleanup(func() {
		fullscreenCurrent.Lock()
		fullscreenCurrent.snapshot = previous
		fullscreenCurrent.Unlock()
	})
	w := httptest.NewRecorder()
	fullscreenState(w, httptest.NewRequest("GET", "/fullscreen/state", nil))
	line := w.Body.String()
	if !strings.HasPrefix(line, "41427ce697a5e69cace8aa9e|415254495354||303a3437202f20323a3538|true|false|") {
		t.Fatalf("unexpected fullscreen state: %q", line)
	}
}
