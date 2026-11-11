package main

import (
	"os"
	"path/filepath"
	"regexp"
)

// Match Components/CachePaths.lua: the Windows user temp directory, not the
// executable/skin folder. Everything here is disposable and can be rebuilt.
func artworkCacheDirectory() string {
	return filepath.Join(os.TempDir(), "YouTubeMusicDesktopPlayer", "artwork")
}

func cacheFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

var legacyArtworkName = regexp.MustCompile(`^(cover_blur(?:_fallback|_[0-9a-f]{16})?\.png|fullscreen-art-[0-9a-f]{16}\.(?:jpg|png|gif)|fullscreen-blur-[0-9a-f]{16}\.png|\.cover-[0-9]+\.tmp)$`)

// Only remove this application's generated legacy files. Never recurse or
// follow links; custom images, playlists, credentials and executables stay put.
func cleanupLegacyArtwork(directory string) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Type().IsRegular() && legacyArtworkName.MatchString(entry.Name()) {
			if err := os.Remove(filepath.Join(directory, entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}
