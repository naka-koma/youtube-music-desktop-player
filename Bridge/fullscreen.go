package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var fullscreenCurrent struct {
	sync.Mutex
	snapshot  skinSnapshot
	updatedAt time.Time
}

var fullscreenCache struct {
	sync.Mutex
	nextID, nextTitle, nextLoading string
	nextCheck                      time.Time
	artID, artLoading              string
	art                            fullscreenArtPair
	artCheck                       time.Time
}

// Both skins can refresh playback independently. Coalesce overlapping polls so
// opening fullscreen does not double the upstream API traffic.
func currentSkinState() skinSnapshot {
	fullscreenCurrent.Lock()
	defer fullscreenCurrent.Unlock()
	if fullscreenCurrent.updatedAt.IsZero() || time.Since(fullscreenCurrent.updatedAt) >= 500*time.Millisecond {
		fullscreenCurrent.snapshot = collectSkinState()
		fullscreenCurrent.updatedAt = time.Now()
	}
	return fullscreenCurrent.snapshot
}

func fullscreenState(w http.ResponseWriter, r *http.Request) {
	snapshot := currentSkinState()
	if snapshot.Title == "" {
		snapshot.Title, snapshot.Position = "Nothing playing", "0:00 / 0:00"
	}
	fullscreenCache.Lock()
	next := ""
	var art fullscreenArtPair
	now := time.Now()
	id := snapshot.trackID
	if id != "" {
		if fullscreenCache.nextID == id {
			next = fullscreenCache.nextTitle
		}
		if fullscreenCache.nextLoading == "" && (fullscreenCache.nextID != id || now.Sub(fullscreenCache.nextCheck) > 3*time.Second) {
			fullscreenCache.nextLoading = id
			go updateFullscreenNext(id)
		}
		if r.URL.Query().Get("art") == "1" {
			art = fullscreenCache.art
			missingArt := art.original != "" && !art.exists(artworkCacheDirectory())
			if fullscreenCache.artLoading == "" && (fullscreenCache.artID != id ||
				missingArt ||
				(art.original == "" && now.Sub(fullscreenCache.artCheck) > 10*time.Second)) {
				fullscreenCache.artLoading = id
				go updateFullscreenArt(id, snapshot.artCandidates)
			}
			// Publish a blank state while rebuilding. Reusing a missing filename
			// would leave a freshly loaded skin with a stale failed image layout.
			if missingArt {
				art = fullscreenArtPair{}
			}
		}
	}
	fullscreenCache.Unlock()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprintf(w, "%x|%x|%x|%x|%t|%t|%.3f|%.3f|%s|%s|%d|%d|", asciiUpper(snapshot.Title),
		asciiUpper(snapshot.Author), next, snapshot.Position, snapshot.Playing, snapshot.Muted,
		snapshot.Percentage, snapshot.Volume, art.original, art.blur, art.width, art.height)
}

func asciiUpper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - ('a' - 'A')
		}
	}
	return string(b)
}

func updateFullscreenNext(id string) {
	queue, err := readQueue()
	title := ""
	if err == nil {
		pos := uniqueQueuePosition(queue.Items, id)
		if pos >= 0 && pos+1 < len(queue.Items) {
			title = queue.Items[pos+1].Title
		}
	}
	fullscreenCache.Lock()
	fullscreenCache.nextLoading, fullscreenCache.nextID = "", id
	fullscreenCache.nextTitle, fullscreenCache.nextCheck = title, time.Now()
	fullscreenCache.Unlock()
}

func fullscreenImageArea(data []byte) int64 {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 8192 || cfg.Height > 8192 {
		return 0
	}
	return int64(cfg.Width) * int64(cfg.Height)
}

func fullscreenCDN(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "ytimg.com" || strings.HasSuffix(host, ".ytimg.com") ||
		host == "yt3.ggpht.com" || strings.HasSuffix(host, ".googleusercontent.com")
}

func updateFullscreenArt(id string, candidates []trackThumbnail) {
	artCache.RLock()
	var best []byte
	if artCache.videoID == id {
		best = artCache.data
	}
	artCache.RUnlock()
	area := fullscreenImageArea(best)
	candidates = append([]trackThumbnail(nil), candidates...)
	sort.SliceStable(candidates, func(i, j int) bool {
		return int64(candidates[i].Width)*int64(candidates[i].Height) > int64(candidates[j].Width)*int64(candidates[j].Height)
	})
	seen := make(map[string]bool)
	for _, item := range candidates {
		declared := int64(item.Width) * int64(item.Height)
		if (declared > 0 && declared <= area) || seen[item.URL] || !fullscreenCDN(item.URL) {
			continue
		}
		seen[item.URL] = true
		resp, err := bookmarkHTTP.Get(item.URL)
		if err != nil {
			continue
		}
		const maxArt = 16 << 20
		var data []byte
		if resp.StatusCode == http.StatusOK {
			data, _ = io.ReadAll(io.LimitReader(resp.Body, maxArt+1))
		}
		resp.Body.Close()
		if len(data) <= maxArt {
			if candidate := fullscreenImageArea(data); candidate > area {
				best, area = data, candidate
			}
		}
	}
	// A regular cover fetch may have completed while we were comparing sizes.
	artCache.RLock()
	if artCache.videoID == id && fullscreenImageArea(artCache.data) > area {
		best, area = artCache.data, fullscreenImageArea(artCache.data)
	}
	artCache.RUnlock()
	var art fullscreenArtPair
	dir := artworkCacheDirectory()
	if area > 0 {
		art, _ = saveFullscreenArtPair(dir, best)
	}
	fullscreenCache.Lock()
	old := fullscreenCache.art
	fullscreenCache.artLoading, fullscreenCache.artID, fullscreenCache.art = "", id, art
	fullscreenCache.artCheck = time.Now()
	fullscreenCache.Unlock()
	if old.original != "" && old.original != art.original {
		_ = os.Remove(filepath.Join(dir, old.original))
		_ = os.Remove(filepath.Join(dir, old.blur))
	}
}

type fullscreenArtPair struct {
	original, blur string
	width, height  int
}

func (pair fullscreenArtPair) exists(dir string) bool {
	return pair.original != "" && pair.blur != "" &&
		cacheFileExists(filepath.Join(dir, pair.original)) && cacheFileExists(filepath.Join(dir, pair.blur))
}

func saveFullscreenArtPair(dir string, data []byte) (fullscreenArtPair, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	ext := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/gif": ".gif"}[http.DetectContentType(data)]
	if err != nil || ext == "" {
		return fullscreenArtPair{}, fmt.Errorf("invalid fullscreen artwork")
	}
	hash := sha256.Sum256(data)
	key := fmt.Sprintf("%x", hash[:8])
	pair := fullscreenArtPair{"fullscreen-art-" + key + ext, "fullscreen-blur-" + key + ".png", cfg.Width, cfg.Height}
	blurPath := filepath.Join(dir, pair.blur)
	if _, err := os.Stat(blurPath); err != nil {
		src, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return fullscreenArtPair{}, err
		}
		// Blur a small copy in the original aspect ratio. Rainmeter crops it to the selected monitor.
		scale := math.Min(1, 480/float64(max(cfg.Width, cfg.Height)))
		width := max(1, int(math.Round(float64(cfg.Width)*scale)))
		height := max(1, int(math.Round(float64(cfg.Height)*scale)))
		base := image.NewRGBA(image.Rect(0, 0, width, height))
		for y := 0; y < height; y++ {
			sy := (float64(y)+0.5)*float64(cfg.Height)/float64(height) - 0.5
			for x := 0; x < width; x++ {
				sx := (float64(x)+0.5)*float64(cfg.Width)/float64(width) - 0.5
				base.SetRGBA(x, y, sampleCover(src, sx, sy))
			}
		}
		blurred := gaussianBlur(base, max(1, float64(max(width, height))*0.045))
		if err := savePNGAtomic(blurPath, blurred); err != nil {
			return fullscreenArtPair{}, err
		}
	}
	if err := saveBytesAtomic(filepath.Join(dir, pair.original), data); err != nil {
		return fullscreenArtPair{}, err
	}
	return pair, nil
}
