package main

import (
	"bytes"
	"image"
	"image/png"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func isolatedArtworkCache(t *testing.T) string {
	t.Helper()
	base := filepath.Join(t.TempDir(), "user temp")
	t.Setenv("TMP", base)
	t.Setenv("TEMP", t.TempDir())
	dir := artworkCacheDirectory()
	if dir != filepath.Join(base, "YouTubeMusicDesktopPlayer", "artwork") {
		t.Fatalf("cache is not under the user's TEMP: %q", dir)
	}
	return dir
}

func testArtwork(t *testing.T) []byte {
	t.Helper()
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 8, 4))); err != nil {
		t.Fatal(err)
	}
	return encoded.Bytes()
}

func TestArtworkCacheRebuildsAfterTempCleanup(t *testing.T) {
	dir := isolatedArtworkCache(t)
	coverBackground.Lock()
	oldTrack, oldName, oldHash, oldDir := coverBackground.trackKey, coverBackground.fileName, coverBackground.sourceHash, coverBackground.directory
	coverBackground.Unlock()
	artCache.Lock()
	oldID, oldData := artCache.videoID, artCache.data
	artCache.Unlock()
	t.Cleanup(func() {
		coverBackground.Lock()
		coverBackground.trackKey, coverBackground.fileName, coverBackground.sourceHash, coverBackground.directory = oldTrack, oldName, oldHash, oldDir
		coverBackground.Unlock()
		artCache.Lock()
		artCache.videoID, artCache.data = oldID, oldData
		artCache.Unlock()
	})
	initializeCoverBackground()
	data := testArtwork(t)
	selectBackgroundTrack("temp-test", true)
	publishCoverBackground("temp-test", data)
	name := backgroundFileName()
	if !strings.HasPrefix(name, "cover_blur_") || name == "cover_blur_fallback.png" || !cacheFileExists(filepath.Join(dir, name)) {
		t.Fatalf("background was not generated in TEMP: %q", name)
	}
	artCache.Lock()
	artCache.videoID, artCache.data = "temp-test", data
	artCache.Unlock()
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if got := backgroundFileName(); got != name || !cacheFileExists(filepath.Join(dir, got)) {
		t.Fatalf("deleted cache was not rebuilt: %q", got)
	}
	if err := os.Remove(filepath.Join(dir, "cover_blur_fallback.png")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	selectBackgroundTrack("", false)
	if got := backgroundFileName(); got != "cover_blur_fallback.png" || !cacheFileExists(filepath.Join(dir, got)) {
		t.Fatal("fallback was not recreated")
	}
}

func TestFullscreenCacheRecreatesMissingBlur(t *testing.T) {
	dir := isolatedArtworkCache(t)
	data := testArtwork(t)
	pair, err := saveFullscreenArtPair(dir, data)
	if err != nil || !pair.exists(dir) {
		t.Fatalf("cannot create fullscreen cache: %v", err)
	}
	if err := os.Remove(filepath.Join(dir, pair.blur)); err != nil {
		t.Fatal(err)
	}
	if pair.exists(dir) {
		t.Fatal("missing blur must invalidate cached artwork")
	}
	if _, err := saveFullscreenArtPair(dir, data); err != nil || !pair.exists(dir) {
		t.Fatalf("cannot regenerate missing blur: %v", err)
	}
}

func TestFullscreenDoesNotPublishMissingCacheFiles(t *testing.T) {
	dir := isolatedArtworkCache(t)
	pair, err := saveFullscreenArtPair(dir, testArtwork(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, pair.blur)); err != nil {
		t.Fatal(err)
	}
	fullscreenCurrent.Lock()
	oldSnapshot, oldTime := fullscreenCurrent.snapshot, fullscreenCurrent.updatedAt
	fullscreenCurrent.snapshot = skinSnapshot{trackID: "cache-test", Title: "Track"}
	fullscreenCurrent.updatedAt = time.Now()
	fullscreenCurrent.Unlock()
	fullscreenCache.Lock()
	oldArt, oldLoading, oldNextLoading := fullscreenCache.art, fullscreenCache.artLoading, fullscreenCache.nextLoading
	fullscreenCache.art, fullscreenCache.artLoading, fullscreenCache.nextLoading = pair, "in-flight", "in-flight"
	fullscreenCache.Unlock()
	t.Cleanup(func() {
		fullscreenCurrent.Lock()
		fullscreenCurrent.snapshot, fullscreenCurrent.updatedAt = oldSnapshot, oldTime
		fullscreenCurrent.Unlock()
		fullscreenCache.Lock()
		fullscreenCache.art, fullscreenCache.artLoading, fullscreenCache.nextLoading = oldArt, oldLoading, oldNextLoading
		fullscreenCache.Unlock()
	})
	w := httptest.NewRecorder()
	fullscreenState(w, httptest.NewRequest("GET", "/fullscreen/state?art=1", nil))
	fields := strings.Split(w.Body.String(), "|")
	if fields[8] != "" || fields[9] != "" {
		t.Fatalf("missing cache files must not be published: %q", w.Body.String())
	}
}

func TestLegacyArtworkCleanupPreservesUserFiles(t *testing.T) {
	dir := t.TempDir()
	generated := []string{"cover_blur.png", "cover_blur_fallback.png", "cover_blur_0123456789abcdef.png", "fullscreen-art-0123456789abcdef.jpg", "fullscreen-blur-0123456789abcdef.png", ".cover-1234.tmp"}
	protected := []string{"playlists.json", "AmbientMusicBridge.exe", "cover_blur_custom.png", "my-artwork.png", "token"}
	for _, name := range append(generated, protected...) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "fullscreen-art-ffffffffffffffff.png"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := cleanupLegacyArtwork(dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range generated {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("generated legacy file remains: %s", name)
		}
	}
	for _, name := range protected {
		if !cacheFileExists(filepath.Join(dir, name)) {
			t.Fatalf("user file was removed: %s", name)
		}
	}
	if info, err := os.Stat(filepath.Join(dir, "fullscreen-art-ffffffffffffffff.png")); err != nil || !info.IsDir() {
		t.Fatal("cleanup must not recurse into directories")
	}
}
