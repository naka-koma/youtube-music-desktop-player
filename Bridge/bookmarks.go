package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type bookmark struct {
	ID           string    `json:"id"`
	PlaylistID   string    `json:"playlistId"`
	URL          string    `json:"url"`
	Name         string    `json:"name"`
	NameSource   string    `json:"nameSource"`
	Author       string    `json:"author"`
	ThumbnailURL *string   `json:"thumbnailUrl"`
	AddedAt      string    `json:"addedAt"`
	Tracks       *struct{} `json:"tracks"`
}

type bookmarkFile struct {
	Version   int        `json:"version"`
	Playlists []bookmark `json:"playlists"`
}

var bookmarkMu sync.Mutex
var bookmarkHTTP = &http.Client{
	Timeout:       5 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

func playBookmark(playlistID string) bool {
	result, err := backend.Command(playbackCommand{Action: actionPlayBookmark, ID: playlistID})
	if err != nil {
		return false
	}
	defer result.Body.Close()
	return successful(result.StatusCode)
}

func bookmarkPlaylistID(raw string) (string, string) {
	link, err := url.Parse(raw)
	if err != nil || (link.Scheme != "http" && link.Scheme != "https") || link.User != nil {
		return "", "clipboard"
	}
	switch strings.ToLower(link.Host) {
	case "music.youtube.com", "www.youtube.com", "youtube.com":
	default:
		return "", "clipboard"
	}
	id := link.Query().Get("list")
	if strings.HasPrefix(id, "RD") {
		return "", "mix"
	}
	if len(id) < 2 || len(id) > 128 || playlistID(id) != id {
		return "", "clipboard"
	}
	return id, ""
}

func validBookmarkID(id string) bool {
	_, err := hex.DecodeString(id)
	return len(id) == 32 && err == nil && strings.ToLower(id) == id
}

func readBookmarks(path string) (bookmarkFile, error) {
	empty := bookmarkFile{Version: 1, Playlists: []bookmark{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return empty, nil
	}
	if err != nil {
		return empty, err
	}
	var saved bookmarkFile
	if json.Unmarshal(data, &saved) != nil || saved.Version != 1 || saved.Playlists == nil {
		return empty, nil
	}
	ids, playlists := map[string]bool{}, map[string]bool{}
	for _, item := range saved.Playlists {
		id, invalid := bookmarkPlaylistID(item.URL)
		if !validBookmarkID(item.ID) || ids[item.ID] || playlists[item.PlaylistID] ||
			invalid != "" || id != item.PlaylistID || strings.TrimSpace(item.Name) == "" ||
			(item.NameSource != "oembed" && item.NameSource != "fallback" && item.NameSource != "user") ||
			item.Tracks != nil {
			return empty, nil
		}
		ids[item.ID], playlists[item.PlaylistID] = true, true
	}
	return saved, nil
}

func saveBookmarks(path string, saved bookmarkFile) error {
	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return err
	}
	return saveBytesAtomic(path, append(data, '\n'))
}

func fallbackBookmarkName(items []bookmark) string {
	used := make(map[string]bool, len(items))
	for _, item := range items {
		used[strings.ToLower(item.Name)] = true
	}
	for n := 1; ; n++ {
		name := fmt.Sprintf("Playlist %d", n)
		if !used[strings.ToLower(name)] {
			return name
		}
	}
}

func fetchBookmarkInfo(item *bookmark) {
	canonical := "https://www.youtube.com/playlist?" + url.Values{"list": {item.PlaylistID}}.Encode()
	endpoint := "https://www.youtube.com/oembed?" + url.Values{"format": {"json"}, "url": {canonical}}.Encode()
	resp, err := bookmarkHTTP.Get(endpoint)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	var info struct {
		Title     string `json:"title"`
		Author    string `json:"author_name"`
		Thumbnail string `json:"thumbnail_url"`
	}
	const maxMetadata = 64 << 10
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxMetadata+1))
	if resp.StatusCode != http.StatusOK || err != nil || len(data) > maxMetadata || json.Unmarshal(data, &info) != nil {
		return
	}
	info.Title = strings.Join(strings.Fields(info.Title), " ")
	if info.Title == "" {
		return
	}
	item.Name, item.NameSource = info.Title, "oembed"
	item.Author = strings.Join(strings.Fields(info.Author), " ")
	if info.Thumbnail != "" {
		item.ThumbnailURL = &info.Thumbnail
	}
}

func bookmarkThumbPath(id string) string {
	return filepath.Join(dataDir, "bookmark-thumbnails", id+".png")
}

func cacheBookmarkThumbnail(item bookmark) {
	if item.ThumbnailURL == nil {
		return
	}
	link, err := url.Parse(*item.ThumbnailURL)
	if err != nil || link.Scheme != "https" || link.User != nil {
		return
	}
	switch strings.ToLower(link.Host) {
	case "i.ytimg.com", "i1.ytimg.com", "i2.ytimg.com", "i3.ytimg.com", "i4.ytimg.com", "yt3.ggpht.com", "yt3.googleusercontent.com":
	default:
		return
	}
	resp, err := bookmarkHTTP.Get(link.String())
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}
	const limit = 2 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil || len(data) > limit {
		return
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 1024 || cfg.Height > 1024 {
		return
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return
	}
	var encoded bytes.Buffer
	if png.Encode(&encoded, img) != nil {
		return
	}
	path := bookmarkThumbPath(item.ID)
	if os.MkdirAll(filepath.Dir(path), 0700) == nil {
		_ = saveBytesAtomic(path, encoded.Bytes())
	}
}

func bookmarkAction(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Ambient-Bookmarks") != "1" {
		http.Error(w, "missing bookmark client header", http.StatusForbidden)
		return
	}
	bookmarkMu.Lock()
	defer bookmarkMu.Unlock()
	path := filepath.Join(dataDir, "playlists.json")
	saved, err := readBookmarks(path)
	status, id := "ok", ""
	if err != nil {
		status = "storage"
	} else {
		status, id = applyBookmarkAction(r, path, &saved)
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprintf(w, "bookmarks-v1|%s|%s\n", status, id)
	for _, item := range saved.Playlists {
		thumb := 0
		if _, err := os.Stat(bookmarkThumbPath(item.ID)); err == nil {
			thumb = 1
		}
		fmt.Fprintf(w, "%s|%x|%x|%s|%d\n", item.ID, item.Name, item.Author, item.NameSource, thumb)
	}
}

func applyBookmarkAction(r *http.Request, path string, saved *bookmarkFile) (string, string) {
	query := r.URL.Query()
	action, id := r.PathValue("action"), query.Get("id")
	switch action {
	case "list":
		return "ok", ""
	case "prepare":
		raw, err := readBookmarkClipboard()
		if err != nil {
			return "clipboard", ""
		}
		raw = strings.TrimSpace(raw)
		if len(raw) > 8192 {
			return "clipboard", ""
		}
		playlistID, status := bookmarkPlaylistID(raw)
		if status != "" {
			return status, ""
		}
		for _, item := range saved.Playlists {
			if item.PlaylistID == playlistID {
				return "duplicate", item.ID
			}
		}
		return "ready", hex.EncodeToString([]byte(raw))
	case "add":
		raw, err := hex.DecodeString(query.Get("urlHex"))
		if err != nil || !utf8.Valid(raw) {
			return "clipboard", ""
		}
		return addBookmark(path, saved, string(raw))
	}

	for i, item := range saved.Playlists {
		if item.ID != id {
			continue
		}
		switch action {
		case "play":
			if !playBookmark(item.PlaylistID) {
				return "play-failed", ""
			}
			return "played", ""
		case "rename":
			raw, err := hex.DecodeString(query.Get("nameHex"))
			if err != nil || !utf8.Valid(raw) {
				return "invalid-name", ""
			}
			name := strings.TrimSpace(string(raw))
			if name == "" {
				return "ok", ""
			}
			if utf8.RuneCountInString(name) > 160 || strings.ContainsAny(name, "\r\n\x00") {
				return "invalid-name", ""
			}
			saved.Playlists[i].Name, saved.Playlists[i].NameSource = name, "user"
			if saveBookmarks(path, *saved) != nil {
				saved.Playlists[i] = item
				return "storage", ""
			}
			return "ok", ""
		case "delete":
			items := make([]bookmark, 0, len(saved.Playlists)-1)
			items = append(items, saved.Playlists[:i]...)
			items = append(items, saved.Playlists[i+1:]...)
			if saveBookmarks(path, bookmarkFile{Version: 1, Playlists: items}) != nil {
				return "storage", ""
			}
			saved.Playlists = items
			_ = os.Remove(bookmarkThumbPath(id))
			return "ok", ""
		}
	}
	return "missing", ""
}

func addBookmark(path string, saved *bookmarkFile, raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if len(raw) > 8192 {
		return "clipboard", ""
	}
	playlistID, status := bookmarkPlaylistID(raw)
	if status != "" {
		return status, ""
	}
	for _, item := range saved.Playlists {
		if item.PlaylistID == playlistID {
			return "duplicate", item.ID
		}
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "storage", ""
	}
	item := bookmark{
		ID: hex.EncodeToString(id[:]), PlaylistID: playlistID, URL: raw,
		Name: fallbackBookmarkName(saved.Playlists), NameSource: "fallback",
		AddedAt: time.Now().Format(time.RFC3339),
	}
	fetchBookmarkInfo(&item)
	saved.Playlists = append(saved.Playlists, item)
	if saveBookmarks(path, *saved) != nil {
		saved.Playlists = saved.Playlists[:len(saved.Playlists)-1]
		return "storage", ""
	}
	cacheBookmarkThumbnail(item)
	return "added", item.ID
}
