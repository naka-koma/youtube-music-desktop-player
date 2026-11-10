package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMakeBlurredCoverProducesRoundedDashboardImage(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 160, 160))
	for y := 0; y < 160; y++ {
		for x := 0; x < 160; x++ {
			if x < 80 {
				source.SetRGBA(x, y, color.RGBA{R: 240, A: 255})
			} else {
				source.SetRGBA(x, y, color.RGBA{B: 240, A: 255})
			}
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, source); err != nil {
		t.Fatal(err)
	}
	got, err := makeBlurredCover(encoded.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds().Dx() != coverWidth || got.Bounds().Dy() != coverHeight {
		t.Fatalf("dimensions = %v, want %dx%d", got.Bounds(), coverWidth, coverHeight)
	}
	if got.RGBAAt(0, 0).A != 0 {
		t.Fatal("outer corner should be transparent")
	}
	if got.RGBAAt(coverWidth/2, coverHeight/2).A != 255 {
		t.Fatal("center should be opaque")
	}
}

func TestRoundedFallbackUsesNavyAndRoundedAlpha(t *testing.T) {
	img := roundedFallback()
	if got := img.RGBAAt(0, 0); got.A != 0 {
		t.Fatalf("corner alpha = %d, want 0", got.A)
	}
	if got := img.RGBAAt(coverWidth/2, coverHeight/2); got != (color.RGBA{R: 32, G: 32, B: 64, A: 255}) {
		t.Fatalf("center color = %#v, want navy fallback", got)
	}
}

func TestValidCoverImageRejectsMissingArtwork(t *testing.T) {
	if validCoverImage([]byte("not an image")) {
		t.Fatal("invalid image data should not be exposed to Rainmeter")
	}
}

func TestSaveBytesAtomicDoesNotRewriteIdenticalArtwork(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cover_blur.png")
	data := []byte("same image bytes")
	if err := saveBytesAtomic(path, data); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	if err := saveBytesAtomic(path, data); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("identical artwork should not be written again")
	}
}
