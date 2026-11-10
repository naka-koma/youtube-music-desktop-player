package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type config struct {
	Playlists []struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	} `json:"playlists"`
}
type trackInfo struct {
	Video struct {
		Title     string `json:"title"`
		Author    string `json:"author"`
		Thumbnail struct {
			Thumbnails []struct {
				URL string `json:"url"`
			} `json:"thumbnails"`
		} `json:"thumbnail"`
	} `json:"video"`
	Meta struct {
		Thumbnail string `json:"thumbnail"`
	} `json:"meta"`
}
type playbackInfo struct {
	Playing bool `json:"playing"`
}

var dataDir = filepath.Join(os.Getenv("LOCALAPPDATA"), "AmbientMusicDashboard")
var apiBase = env("YTM_API_URL", "http://127.0.0.1:13091")
var client = &http.Client{Timeout: 4 * time.Second}

func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return strings.TrimRight(v, "/")
	}
	return fallback
}
func api(path, method, body string) (*http.Response, error) {
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, apiBase+path, r)
	if err != nil {
		return nil, err
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token, err := os.ReadFile(filepath.Join(dataDir, "token")); err == nil {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(token)))
	}
	return client.Do(req)
}
func readBody(r *http.Response) json.RawMessage {
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	if !json.Valid(b) {
		return json.RawMessage(`null`)
	}
	return b
}
func write(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}
func state(w http.ResponseWriter, _ *http.Request) {
	result := map[string]any{"connected": false, "title": "YTMDesktop2 unavailable", "author": "Start YTMDesktop2 to listen", "thumbnail": "", "playing": false}
	tr, err := api("/track", "GET", "")
	if err != nil {
		write(w, result)
		return
	}
	trackBytes := readBody(tr)
	if tr.StatusCode < 200 || tr.StatusCode >= 300 {
		write(w, result)
		return
	}
	result["connected"] = true
	result["title"] = "Nothing playing"
	result["author"] = "Choose a playlist to begin"
	var track trackInfo
	if json.Unmarshal(trackBytes, &track) == nil && track.Video.Title != "" {
		result["title"] = track.Video.Title
		result["author"] = track.Video.Author
		art := track.Meta.Thumbnail
		if art == "" && len(track.Video.Thumbnail.Thumbnails) > 0 {
			art = track.Video.Thumbnail.Thumbnails[len(track.Video.Thumbnail.Thumbnails)-1].URL
		}
		result["thumbnail"] = art
	}
	sr, err := api("/track/state", "GET", "")
	if err == nil {
		stateBytes := readBody(sr)
		var playback playbackInfo
		if sr.StatusCode >= 200 && sr.StatusCode < 300 && json.Unmarshal(stateBytes, &playback) == nil {
			result["playing"] = playback.Playing
		}
	}
	write(w, result)
}
func main() {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /state", state)
	mux.HandleFunc("GET /playlists", func(w http.ResponseWriter, _ *http.Request) {
		exe, _ := os.Executable()
		b, err := os.ReadFile(filepath.Join(filepath.Dir(exe), "playlists.json"))
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	})
	mux.HandleFunc("POST /{cmd}", func(w http.ResponseWriter, r *http.Request) {
		cmd := r.PathValue("cmd")
		routes := map[string]string{"toggle": "/track/toggle-play-state", "next": "/track/next", "prev": "/track/prev", "shuffle": "/track/shuffle", "repeat": "/track/repeat"}
		if cmd == "playlist" {
			http.Error(w, "use /playlist/{index}", 400)
			return
		}
		path, ok := routes[cmd]
		if !ok {
			http.NotFound(w, r)
			return
		}
		resp, err := api(path, "POST", "{}")
		if err != nil {
			write(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		body := readBody(resp)
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(body)
	})
	mux.HandleFunc("POST /playlist/{index}", func(w http.ResponseWriter, r *http.Request) {
		idx, err := strconv.Atoi(r.PathValue("index"))
		if err != nil {
			http.Error(w, "invalid playlist index", 400)
			return
		}
		exe, _ := os.Executable()
		b, err := os.ReadFile(filepath.Join(filepath.Dir(exe), "playlists.json"))
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		var c config
		if err = json.Unmarshal(b, &c); err != nil || idx < 0 || idx >= len(c.Playlists) {
			http.Error(w, "playlist index out of range", 404)
			return
		}
		p := c.Playlists[idx]
		if p.ID == "" || strings.HasPrefix(p.ID, "REPLACE_") {
			http.Error(w, "configure playlist ID first", 400)
			return
		}
		payload, _ := json.Marshal(map[string]any{"playlistId": p.ID, "play": true})
		resp, err := api("/nav/playlist", "POST", string(payload))
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		defer resp.Body.Close()
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	})
	addr := "127.0.0.1:38471"
	fmt.Println("Ambient Music bridge listening on http://" + addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
