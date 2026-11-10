package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type bookmarkTransport func(*http.Request) (*http.Response, error)

func (f bookmarkTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestBookmarkLifecycle(t *testing.T) {
	oldDir, oldAPI, oldHTTP := dataDir, apiBase, bookmarkHTTP
	t.Cleanup(func() { dataDir, apiBase, bookmarkHTTP = oldDir, oldAPI, oldHTTP })
	dataDir = t.TempDir()
	path := filepath.Join(dataDir, "playlists.json")
	if err := os.WriteFile(path, []byte("{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	const animeURL = "https://music.youtube.com/playlist?list=PLSF-X6q-YynI&si=ignored"
	var thumbnail bytes.Buffer
	if err := png.Encode(&thumbnail, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	var metadataRequests, thumbnailRequests int
	bookmarkHTTP = &http.Client{Transport: bookmarkTransport(func(r *http.Request) (*http.Response, error) {
		var body string
		switch r.URL.Host {
		case "www.youtube.com":
			metadataRequests++
			if r.Method != "GET" || r.URL.Path != "/oembed" || r.URL.Query().Get("format") != "json" {
				t.Fatalf("unexpected oEmbed request: %s", r.URL)
			}
			switch got := r.URL.Query().Get("url"); got {
			case "https://www.youtube.com/playlist?list=PLSF-X6q-YynI":
				body = "{\"title\":\"Anime\",\"author_name\":\"bassglyph\",\"thumbnail_url\":\"https://i.ytimg.com/vi/example/hqdefault.jpg\"}"
			case "https://www.youtube.com/playlist?list=PLfallback":
				body = "{\"title\":\"incomplete response\"}{"
			default:
				t.Fatalf("non-canonical playlist URL: %s", got)
			}
		case "i.ytimg.com":
			thumbnailRequests++
			body = thumbnail.String()
		default:
			t.Fatalf("unexpected external request: %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		err := json.NewDecoder(r.Body).Decode(&payload)
		if r.Method != "POST" || r.URL.Path != "/nav/playlist" ||
			r.Header.Get("Authorization") != "Bearer test-bookmark-token" ||
			err != nil || payload["play"] != true || payload["playlistId"] != "PLSF-X6q-YynI" {
			t.Errorf("unexpected playback request: %s %s %+v (%v)", r.Method, r.URL, payload, err)
			w.WriteHeader(400)
			return
		}
		w.WriteHeader(204)
	}))
	defer api.Close()
	apiBase = api.URL
	if err := os.WriteFile(filepath.Join(dataDir, "token"), []byte("test-bookmark-token"), 0600); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /bookmarks/{action}", bookmarkAction)
	call := func(path, want string) string {
		t.Helper()
		r := httptest.NewRequest("POST", "/bookmarks/"+path, nil)
		r.Header.Set("X-Ambient-Bookmarks", "1")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		line, _, _ := strings.Cut(w.Body.String(), "\n")
		status, id, _ := strings.Cut(strings.TrimPrefix(line, "bookmarks-v1|"), "|")
		if w.Code != 200 || status != want {
			t.Fatalf("%s: HTTP %d, got %q; want %q", path, w.Code, line, want)
		}
		return id
	}
	add := func(raw, want string) string {
		return call("add?urlHex="+hex.EncodeToString([]byte(raw)), want)
	}
	read := func() bookmarkFile {
		t.Helper()
		items, err := readBookmarks(path)
		if err != nil {
			t.Fatal(err)
		}
		return items
	}
	call("list", "ok")
	if got := read(); len(got.Playlists) != 0 {
		t.Fatalf("malformed file should be empty: %+v", got)
	}
	add("https://youtube.com.example.org/playlist?list=PLinvalid", "clipboard")
	add("https://music.youtube.com/watch?list=RDmix", "mix")
	id := add(animeURL, "added")
	if duplicate := add("https://youtube.com/playlist?list=PLSF-X6q-YynI", "duplicate"); duplicate != id {
		t.Fatalf("duplicate should reference %s, got %s", id, duplicate)
	}
	fallbackID := add("https://www.youtube.com/watch?v=ignored&list=PLfallback", "added")
	saved := read()
	if len(saved.Playlists) != 2 {
		t.Fatalf("expected two stored bookmarks: %+v", saved)
	}
	anime, fallback := saved.Playlists[0], saved.Playlists[1]
	if anime.ID != id || anime.URL != animeURL || anime.Name != "Anime" || anime.Author != "bassglyph" ||
		anime.NameSource != "oembed" || anime.Tracks != nil || anime.AddedAt == "" {
		t.Fatalf("unexpected saved oEmbed bookmark: %+v", anime)
	}
	if fallback.ID != fallbackID || fallback.Name != "Playlist 1" || fallback.NameSource != "fallback" ||
		fallback.Author != "" || fallback.ThumbnailURL != nil {
		t.Fatalf("unexpected fallback bookmark: %+v", fallback)
	}
	if _, err := os.Stat(bookmarkThumbPath(id)); err != nil {
		t.Fatal(err)
	}
	if metadataRequests != 2 || thumbnailRequests != 1 {
		t.Fatalf("unexpected network requests: %d metadata, %d thumbnails", metadataRequests, thumbnailRequests)
	}
	name := "作業用 🎵 [#Test#]"
	call("rename?id="+id+"&nameHex="+hex.EncodeToString([]byte(name)), "ok")
	call("rename?id="+id+"&nameHex=20", "ok")
	if got := read().Playlists[0]; got.Name != name || got.NameSource != "user" {
		t.Fatalf("rename not persisted: %+v", got)
	}
	call("play?id="+id, "played")
	api.Close()
	call("play?id="+id, "play-failed")
	call("delete?id="+id, "ok")
	if got := read(); len(got.Playlists) != 1 || got.Playlists[0].ID != fallbackID {
		t.Fatalf("wrong bookmarks after delete: %+v", got)
	}
	if _, err := os.Stat(bookmarkThumbPath(id)); !os.IsNotExist(err) {
		t.Fatalf("deleted bookmark thumbnail is still present: %v", err)
	}
}
