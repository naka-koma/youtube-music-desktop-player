package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const (
	coverWidth   = 520
	coverHeight  = 148
	blurSigma    = 16.0
	cornerRadius = 18.0
)

var coverBackground struct {
	sync.RWMutex
	trackKey   string
	fileName   string
	sourceHash string
	directory  string
}

func initializeCoverBackground() {
	dir := artworkCacheDirectory()
	fallback := roundedFallback()
	if err := savePNGAtomic(filepath.Join(dir, "cover_blur.png"), fallback); err != nil {
		log.Printf("cover background: initialize fallback: %v", err)
		return
	}
	if err := savePNGAtomic(filepath.Join(dir, "cover_blur_fallback.png"), fallback); err != nil {
		log.Printf("cover background: initialize fallback: %v", err)
		return
	}
	coverBackground.Lock()
	coverBackground.directory = dir
	coverBackground.fileName = "cover_blur_fallback.png"
	coverBackground.Unlock()
}

func backgroundFileName() string {
	coverBackground.RLock()
	name := coverBackground.fileName
	dir := coverBackground.directory
	coverBackground.RUnlock()
	if name == "" {
		name = "cover_blur_fallback.png"
	}
	if dir == "" {
		return name
	}
	if cacheFileExists(filepath.Join(dir, name)) {
		return name
	}
	// TEMP can be emptied while the bridge is running. Rebuild from the in-memory
	// artwork rather than leaving an unchanged filename pointing at a missing file.
	artCache.RLock()
	id, data := artCache.videoID, artCache.data
	artCache.RUnlock()
	if len(data) > 0 && name != "cover_blur_fallback.png" {
		publishCoverBackground(id, data)
		if cacheFileExists(filepath.Join(dir, name)) {
			return name
		}
	}
	if err := savePNGAtomic(filepath.Join(dir, "cover_blur_fallback.png"), roundedFallback()); err != nil {
		log.Printf("cover background: recreate fallback: %v", err)
	}
	return "cover_blur_fallback.png"
}

func selectBackgroundTrack(trackID string, hasArtwork bool) {
	coverBackground.Lock()
	if coverBackground.trackKey != trackID {
		coverBackground.trackKey = trackID
	}
	if !hasArtwork {
		coverBackground.fileName = "cover_blur_fallback.png"
	}
	coverBackground.Unlock()
}

func failBackgroundTrack(trackID string) {
	coverBackground.Lock()
	if coverBackground.trackKey == trackID {
		coverBackground.fileName = "cover_blur_fallback.png"
	}
	coverBackground.Unlock()
}

func publishCoverBackground(trackID string, source []byte) {
	contentKey := sha256.Sum256(source)
	contentHash := hex.EncodeToString(contentKey[:])
	name := "cover_blur_" + hex.EncodeToString(contentKey[:8]) + ".png"
	coverBackground.RLock()
	unchanged := coverBackground.trackKey == trackID && coverBackground.directory != "" && coverBackground.sourceHash == contentHash && coverBackground.fileName == name
	dir := coverBackground.directory
	coverBackground.RUnlock()
	if unchanged && cacheFileExists(filepath.Join(dir, name)) {
		return
	}
	img, err := makeBlurredCover(source)
	if err != nil {
		log.Printf("cover background: decode image: %v", err)
		failBackgroundTrack(trackID)
		return
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		log.Printf("cover background: encode image: %v", err)
		failBackgroundTrack(trackID)
		return
	}
	coverBackground.Lock()
	defer coverBackground.Unlock()
	if coverBackground.trackKey != trackID || coverBackground.directory == "" {
		return
	}
	if coverBackground.sourceHash == contentHash && coverBackground.fileName == name && cacheFileExists(filepath.Join(coverBackground.directory, name)) {
		return
	}
	data := encoded.Bytes()
	if err := saveBytesAtomic(filepath.Join(coverBackground.directory, name), data); err != nil {
		log.Printf("cover background: publish image: %v", err)
		coverBackground.fileName = "cover_blur_fallback.png"
		return
	}
	if err := saveBytesAtomic(filepath.Join(coverBackground.directory, "cover_blur.png"), data); err != nil {
		log.Printf("cover background: update canonical image: %v", err)
	}
	coverBackground.fileName = name
	coverBackground.sourceHash = contentHash
	cleanupCoverFiles(coverBackground.directory, name)
}

func cleanupCoverFiles(dir, current string) {
	files, err := filepath.Glob(filepath.Join(dir, "cover_blur_*.png"))
	if err != nil || len(files) <= 24 {
		return
	}
	sort.Slice(files, func(i, j int) bool {
		a, _ := os.Stat(files[i])
		b, _ := os.Stat(files[j])
		if a == nil || b == nil {
			return files[i] < files[j]
		}
		return a.ModTime().After(b.ModTime())
	})
	kept := 0
	for _, file := range files {
		base := filepath.Base(file)
		if strings.EqualFold(base, "cover_blur_fallback.png") || strings.EqualFold(base, current) || kept < 24 {
			kept++
			continue
		}
		_ = os.Remove(file)
	}
}

func savePNGAtomic(path string, img image.Image) error {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		return err
	}
	return saveBytesAtomic(path, encoded.Bytes())
}

func validCoverImage(data []byte) bool {
	_, _, err := image.DecodeConfig(bytes.NewReader(data))
	return err == nil
}

func saveBytesAtomic(path string, data []byte) error {
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, data) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".cover-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return replaceFileAtomic(tmpName, path)
}

func roundedFallback() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, coverWidth, coverHeight))
	for y := 0; y < coverHeight; y++ {
		for x := 0; x < coverWidth; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 32, G: 32, B: 64, A: roundedAlpha(x, y)})
		}
	}
	return img
}

func makeBlurredCover(data []byte) (*image.RGBA, error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	bounds := src.Bounds()
	sw, sh := bounds.Dx(), bounds.Dy()
	if sw < 1 || sh < 1 {
		return nil, fmt.Errorf("empty cover image")
	}
	// Crop to the target aspect ratio, then bilinearly scale to the dashboard size.
	targetAspect := float64(coverWidth) / float64(coverHeight)
	sourceAspect := float64(sw) / float64(sh)
	crop := image.Rect(0, 0, sw, sh)
	if sourceAspect > targetAspect {
		cw := int(math.Round(float64(sh) * targetAspect))
		crop.Min.X = (sw - cw) / 2
		crop.Max.X = crop.Min.X + cw
	} else if sourceAspect < targetAspect {
		ch := int(math.Round(float64(sw) / targetAspect))
		crop.Min.Y = (sh - ch) / 2
		crop.Max.Y = crop.Min.Y + ch
	}
	base := image.NewRGBA(image.Rect(0, 0, coverWidth, coverHeight))
	for y := 0; y < coverHeight; y++ {
		sy := float64(crop.Min.Y) + (float64(y)+0.5)*float64(crop.Dy())/coverHeight - 0.5
		for x := 0; x < coverWidth; x++ {
			sx := float64(crop.Min.X) + (float64(x)+0.5)*float64(crop.Dx())/coverWidth - 0.5
			base.SetRGBA(x, y, sampleCover(src, sx, sy))
		}
	}
	blurred := gaussianBlur(base, blurSigma)
	for y := 0; y < coverHeight; y++ {
		for x := 0; x < coverWidth; x++ {
			c := blurred.RGBAAt(x, y)
			c.R = uint8(math.Round(float64(c.R) * 0.88))
			c.G = uint8(math.Round(float64(c.G) * 0.88))
			c.B = uint8(math.Round(float64(c.B) * 0.88))
			c.A = roundedAlpha(x, y)
			blurred.SetRGBA(x, y, c)
		}
	}
	return blurred, nil
}

func sampleCover(src image.Image, x, y float64) color.RGBA {
	b := src.Bounds()
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x > float64(b.Dx()-1) {
		x = float64(b.Dx() - 1)
	}
	if y > float64(b.Dy()-1) {
		y = float64(b.Dy() - 1)
	}
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	x1, y1 := min(x0+1, b.Dx()-1), min(y0+1, b.Dy()-1)
	fx, fy := x-float64(x0), y-float64(y0)
	colors := [4]color.RGBA{
		compositeNavy(src.At(b.Min.X+x0, b.Min.Y+y0)), compositeNavy(src.At(b.Min.X+x1, b.Min.Y+y0)),
		compositeNavy(src.At(b.Min.X+x0, b.Min.Y+y1)), compositeNavy(src.At(b.Min.X+x1, b.Min.Y+y1)),
	}
	channel := func(i int) uint8 {
		component := func(c color.RGBA) uint8 {
			switch i {
			case 0:
				return c.R
			case 1:
				return c.G
			default:
				return c.B
			}
		}
		top := float64(component(colors[0]))*(1-fx) + float64(component(colors[1]))*fx
		bottom := float64(component(colors[2]))*(1-fx) + float64(component(colors[3]))*fx
		return uint8(math.Round(top*(1-fy) + bottom*fy))
	}
	return color.RGBA{R: channel(0), G: channel(1), B: channel(2), A: 255}
}

func compositeNavy(c color.Color) color.RGBA {
	r, g, b, a := c.RGBA()
	alpha := float64(a) / 65535
	return color.RGBA{
		R: uint8(math.Round(float64(r)/257*alpha + 32*(1-alpha))),
		G: uint8(math.Round(float64(g)/257*alpha + 32*(1-alpha))),
		B: uint8(math.Round(float64(b)/257*alpha + 64*(1-alpha))), A: 255,
	}
}

func gaussianBlur(src *image.RGBA, sigma float64) *image.RGBA {
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	radius := int(math.Ceil(3 * sigma))
	kernel := make([]float64, radius*2+1)
	var total float64
	for i := -radius; i <= radius; i++ {
		v := math.Exp(-float64(i*i) / (2 * sigma * sigma))
		kernel[i+radius] = v
		total += v
	}
	for i := range kernel {
		kernel[i] /= total
	}
	count := width * height * 4
	horizontal := make([]float32, count)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := (y*width + x) * 4
			for k := -radius; k <= radius; k++ {
				sx := min(max(x+k, 0), width-1)
				p := src.PixOffset(bounds.Min.X+sx, bounds.Min.Y+y)
				weight := float32(kernel[k+radius])
				for ch := 0; ch < 4; ch++ {
					horizontal[idx+ch] += float32(src.Pix[p+ch]) * weight
				}
			}
		}
	}
	out := image.NewRGBA(bounds)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			p := out.PixOffset(bounds.Min.X+x, bounds.Min.Y+y)
			for ch := 0; ch < 4; ch++ {
				var value float32
				for k := -radius; k <= radius; k++ {
					sy := min(max(y+k, 0), height-1)
					srcIdx := (sy*width + x) * 4
					value += horizontal[srcIdx+ch] * float32(kernel[k+radius])
				}
				out.Pix[p+ch] = uint8(min(255, max(0, int(math.Round(float64(value))))))
			}
		}
	}
	return out
}

func roundedAlpha(x, y int) uint8 {
	// Four-by-four coverage keeps the 18px silhouette smooth without extra assets.
	inside := 0
	for sy := 0; sy < 4; sy++ {
		for sx := 0; sx < 4; sx++ {
			px, py := float64(x)+(float64(sx)+0.5)/4, float64(y)+(float64(sy)+0.5)/4
			cx, cy := px, py
			if cx < cornerRadius {
				cx = cornerRadius
			}
			if cx > coverWidth-cornerRadius {
				cx = coverWidth - cornerRadius
			}
			if cy < cornerRadius {
				cy = cornerRadius
			}
			if cy > coverHeight-cornerRadius {
				cy = coverHeight - cornerRadius
			}
			dx, dy := px-cx, py-cy
			if dx*dx+dy*dy <= cornerRadius*cornerRadius {
				inside++
			}
		}
	}
	return uint8(inside * 255 / 16)
}
