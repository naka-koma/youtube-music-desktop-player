package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type config struct {
	Playlists []playlist `json:"playlists"`
}

var playlistIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func playlistID(value string) string {
	value = strings.TrimSpace(value)
	if strings.Contains(value, "://") {
		link, err := url.Parse(value)
		if err != nil || (link.Scheme != "https" && link.Scheme != "http") || link.User != nil {
			return ""
		}
		switch strings.ToLower(link.Hostname()) {
		case "music.youtube.com", "youtube.com", "www.youtube.com", "m.youtube.com", "youtu.be":
			value = link.Query().Get("list")
		default:
			return ""
		}
	}
	if strings.HasPrefix(value, "REPLACE_") || !playlistIDPattern.MatchString(value) {
		return ""
	}
	return value
}

type trackThumbnail struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type trackInfo struct {
	ID    string `json:"id"`
	Video struct {
		VideoID   string `json:"videoId"`
		Title     string `json:"title"`
		Author    string `json:"author"`
		Thumbnail struct {
			Thumbnails []trackThumbnail `json:"thumbnails"`
		} `json:"thumbnail"`
	} `json:"video"`
	Meta struct {
		Thumbnail string `json:"thumbnail"`
		Liked     bool   `json:"liked"`
	} `json:"meta"`
}
type playbackInfo struct {
	Playing    bool    `json:"playing"`
	Progress   float64 `json:"progress"`
	Duration   float64 `json:"duration"`
	Percentage float64 `json:"percentage"`
	Shuffle    bool    `json:"shuffle"`
	Loop       bool    `json:"loop"`
}
type volumeInfo struct {
	Volume float64 `json:"volume"`
}
type skinSnapshot struct {
	trackID        string
	artCandidates  []trackThumbnail
	Connected      bool    `json:"connected"`
	Title          string  `json:"title"`
	Author         string  `json:"author"`
	Thumbnail      string  `json:"thumbnail"`
	Playing        bool    `json:"playing"`
	Progress       float64 `json:"progress"`
	Duration       float64 `json:"duration"`
	Percentage     float64 `json:"percentage"`
	Position       string  `json:"position"`
	Shuffle        bool    `json:"shuffle"`
	Repeat         bool    `json:"repeat"`
	Liked          bool    `json:"liked"`
	Volume         float64 `json:"volume"`
	Muted          bool    `json:"muted"`
	BackgroundFile string  `json:"backgroundFile"`
	Elapsed        string  `json:"elapsed"`
	Total          string  `json:"total"`
}

var dataDir = filepath.Join(os.Getenv("LOCALAPPDATA"), "AmbientMusicDashboard")
var apiBase = env("YTM_API_URL", "http://127.0.0.1:13091")
var client = &http.Client{Timeout: 4 * time.Second}
var artCache struct {
	sync.RWMutex
	videoID  string
	loading  string
	failedID string
	failedAt time.Time
	data     []byte
	typeMIME string
}
var volumeState struct {
	sync.Mutex
	previous float64
}
var playbackDuration struct {
	sync.RWMutex
	seconds float64
}
var optimisticSeek struct {
	sync.Mutex
	progress float64
	expires  time.Time
}
var lastSkinState struct {
	sync.RWMutex
	value skinSnapshot
	valid bool
}
var volumeCommandMu sync.Mutex
var volumeDrag struct {
	sync.RWMutex
	active  bool
	percent float64
	left    float64
	width   float64
	session uint64
}
var seekDrag struct {
	sync.RWMutex
	active  bool
	percent float64
	left    float64
	width   float64
	session uint64
}

type volumeUpdate struct {
	percent float64
	session uint64
}

var volumeUpdates = make(chan volumeUpdate, 1)

func currentVolume() (float64, error) {
	resp, err := api("/track/volume", "POST", "{}")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var volume volumeInfo
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || json.NewDecoder(resp.Body).Decode(&volume) != nil {
		return 0, fmt.Errorf("volume is unavailable")
	}
	return volume.Volume, nil
}

func setVolume(value float64) (*http.Response, error) {
	volumeCommandMu.Lock()
	defer volumeCommandMu.Unlock()
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	payload, _ := json.Marshal(map[string]any{"volume": value})
	return api("/track/volume", "POST", string(payload))
}

func seekToPercent(percent float64) (*http.Response, error) {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	playbackDuration.RLock()
	duration := playbackDuration.seconds
	playbackDuration.RUnlock()
	if duration <= 0 {
		resp, err := api("/track/state", "GET", "")
		if err != nil {
			return nil, err
		}
		var playback playbackInfo
		body := readBody(resp)
		if resp.StatusCode < 200 || resp.StatusCode >= 300 || json.Unmarshal(body, &playback) != nil || playback.Duration <= 0 {
			return nil, fmt.Errorf("track duration is unavailable")
		}
		duration = playback.Duration
		playbackDuration.Lock()
		playbackDuration.seconds = duration
		playbackDuration.Unlock()
	}
	targetMilliseconds := duration * percent * 10
	payload, _ := json.Marshal(map[string]any{"time": targetMilliseconds, "type": "seek"})
	return api("/track/seek", "POST", string(payload))
}

func seekOptimistically(percent float64) error {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	playbackDuration.RLock()
	duration := playbackDuration.seconds
	playbackDuration.RUnlock()
	if duration <= 0 {
		return fmt.Errorf("track duration is unavailable")
	}
	target := duration * percent / 100
	optimisticSeek.Lock()
	optimisticSeek.progress = target
	optimisticSeek.expires = time.Now().Add(5 * time.Second)
	optimisticSeek.Unlock()

	go func() {
		targetMilliseconds := target * 1000
		payload, _ := json.Marshal(map[string]any{"time": targetMilliseconds, "type": "seek"})
		resp, err := api("/track/seek", "POST", string(payload))
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				optimisticSeek.Lock()
				if optimisticSeek.progress == target {
					optimisticSeek.expires = time.Now().Add(1200 * time.Millisecond)
				}
				optimisticSeek.Unlock()
				return
			}
		}
		optimisticSeek.Lock()
		if optimisticSeek.progress == target {
			optimisticSeek.expires = time.Time{}
		}
		optimisticSeek.Unlock()
	}()
	return nil
}

func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return strings.TrimRight(v, "/")
	}
	return fallback
}
func api(path, method, body string) (*http.Response, error) {
	req, err := apiRequest(path, method, body)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func apiRequest(path, method, body string) (*http.Request, error) {
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
	return req, nil
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

func formatTime(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	total := int(seconds)
	return fmt.Sprintf("%d:%02d", total/60, total%60)
}

func cacheArtwork(videoID, fallbackURL string) string {
	if videoID == "" {
		return cachedArtworkURL()
	}
	artCache.Lock()
	if artCache.videoID == videoID && len(artCache.data) > 0 {
		artCache.Unlock()
		return "http://127.0.0.1:38471/artwork.jpg?id=" + url.QueryEscape(videoID)
	}
	if artCache.loading == videoID {
		previous := cachedArtworkURLLocked()
		artCache.Unlock()
		return previous
	}
	if artCache.failedID == videoID && time.Since(artCache.failedAt) < 30*time.Second {
		previous := cachedArtworkURLLocked()
		artCache.Unlock()
		return previous
	}
	previous := cachedArtworkURLLocked()
	artCache.loading = videoID
	artCache.failedID = ""
	artCache.Unlock()
	go func() {
		var data []byte
		mimeType := ""
		resp, err := api("/track/thumbnail?id="+url.QueryEscape(videoID), "GET", "")
		if err == nil {
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				data, _ = io.ReadAll(resp.Body)
				mimeType = resp.Header.Get("Content-Type")
			}
			resp.Body.Close()
		}
		if len(data) == 0 && fallbackURL != "" {
			resp, err = client.Get(fallbackURL)
			if err == nil {
				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					data, _ = io.ReadAll(resp.Body)
					mimeType = resp.Header.Get("Content-Type")
				}
				resp.Body.Close()
			}
		}
		if len(data) > 0 && !validCoverImage(data) {
			data = nil
		}
		artCache.Lock()
		cached := false
		if artCache.loading == videoID {
			artCache.loading = ""
			if len(data) > 0 {
				artCache.videoID, artCache.data, artCache.typeMIME = videoID, data, mimeType
				artCache.failedID = ""
				cached = true
			} else {
				artCache.failedID = videoID
				artCache.failedAt = time.Now()
			}
		}
		artCache.Unlock()
		if cached {
			publishCoverBackground(videoID, data)
		} else {
			failBackgroundTrack(videoID)
		}
	}()
	// Publish the URL only after the local cache is ready so a missing cover
	// never makes Rainmeter request a file that is not there yet.
	return previous
}

func cachedArtworkURL() string {
	artCache.RLock()
	defer artCache.RUnlock()
	return cachedArtworkURLLocked()
}

// The caller must hold at least a read lock on artCache.
func cachedArtworkURLLocked() string {
	if artCache.videoID == "" || len(artCache.data) == 0 {
		return ""
	}
	return "http://127.0.0.1:38471/artwork.jpg?id=" + url.QueryEscape(artCache.videoID)
}

func artwork(w http.ResponseWriter, r *http.Request) {
	artCache.RLock()
	defer artCache.RUnlock()
	if artCache.videoID == "" || (r.URL.Query().Get("id") != "" && r.URL.Query().Get("id") != artCache.videoID) || len(artCache.data) == 0 {
		http.NotFound(w, r)
		return
	}
	mimeType := artCache.typeMIME
	if mimeType == "" {
		mimeType = http.DetectContentType(artCache.data)
	}
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(artCache.data)
}

func state(w http.ResponseWriter, _ *http.Request) {
	write(w, currentSkinState())
}

func collectSkinState() skinSnapshot {
	if snapshot, ok := optimisticStateSnapshot(); ok {
		return snapshot
	}
	result := skinSnapshot{Connected: false, Title: "YTMDesktop2 unavailable", Author: "Start YTMDesktop2 to listen", Position: "0:00 / 0:00", Elapsed: "0:00", Total: "0:00", Thumbnail: cachedArtworkURL()}
	tr, err := api("/track", "GET", "")
	if err != nil {
		selectBackgroundTrack("", false)
		result.BackgroundFile = backgroundFileName()
		return result
	}
	trackBytes := readBody(tr)
	if tr.StatusCode < 200 || tr.StatusCode >= 300 {
		selectBackgroundTrack("", false)
		result.BackgroundFile = backgroundFileName()
		return result
	}
	result.Connected = true
	result.Title = "Nothing playing"
	result.Author = "Choose a playlist to begin"
	var track trackInfo
	if json.Unmarshal(trackBytes, &track) == nil && track.Video.Title != "" {
		result.Title = track.Video.Title
		result.Author = track.Video.Author
		result.Liked = track.Meta.Liked
		art := track.Meta.Thumbnail
		if art == "" && len(track.Video.Thumbnail.Thumbnails) > 0 {
			art = track.Video.Thumbnail.Thumbnails[len(track.Video.Thumbnail.Thumbnails)-1].URL
		}
		videoID := track.ID
		if videoID == "" {
			videoID = track.Video.VideoID
		}
		result.trackID = videoID
		result.artCandidates = append(result.artCandidates, track.Video.Thumbnail.Thumbnails...)
		if track.Meta.Thumbnail != "" {
			result.artCandidates = append(result.artCandidates, trackThumbnail{URL: track.Meta.Thumbnail})
		}
		selectBackgroundTrack(videoID, videoID != "")
		result.Thumbnail = cacheArtwork(videoID, art)
	} else {
		selectBackgroundTrack("", false)
		result.Thumbnail = cachedArtworkURL()
	}
	type stateResult struct {
		playback playbackInfo
		volume   float64
		err      error
	}
	stateCh := make(chan stateResult, 1)
	volumeCh := make(chan stateResult, 1)
	go func() {
		sr, err := api("/track/state", "GET", "")
		if err != nil {
			stateCh <- stateResult{err: err}
			return
		}
		defer sr.Body.Close()
		var playback playbackInfo
		if sr.StatusCode < 200 || sr.StatusCode >= 300 || json.NewDecoder(sr.Body).Decode(&playback) != nil {
			stateCh <- stateResult{err: fmt.Errorf("playback state is unavailable")}
			return
		}
		stateCh <- stateResult{playback: playback}
	}()
	go func() {
		volume, err := currentVolume()
		volumeCh <- stateResult{volume: volume, err: err}
	}()
	playbackResult, volumeResult := <-stateCh, <-volumeCh
	if playbackResult.err == nil {
		playback := playbackResult.playback
		playbackDuration.Lock()
		playbackDuration.seconds = playback.Duration
		playbackDuration.Unlock()
		optimisticSeek.Lock()
		if optimisticSeek.expires.After(time.Now()) {
			target := optimisticSeek.progress
			if playback.Progress >= target-3 && playback.Progress <= target+3 {
				optimisticSeek.expires = time.Time{}
			} else {
				playback.Progress = target
				if playback.Duration > 0 {
					playback.Percentage = target / playback.Duration * 100
				}
			}
		} else {
			optimisticSeek.expires = time.Time{}
		}
		optimisticSeek.Unlock()
		result.Playing = playback.Playing
		result.Progress = playback.Progress
		result.Duration = playback.Duration
		result.Percentage = playback.Percentage
		result.Shuffle = playback.Shuffle
		result.Repeat = playback.Loop
		result.Position = formatTime(playback.Progress) + " / " + formatTime(playback.Duration)
		result.Elapsed = formatTime(playback.Progress)
		result.Total = formatTime(playback.Duration)
	}
	if volumeResult.err == nil {
		result.Volume = volumeResult.volume
		result.Muted = volumeResult.volume <= 0
		if volumeResult.volume > 0 {
			volumeState.Lock()
			volumeState.previous = volumeResult.volume
			volumeState.Unlock()
		}
	}
	result.BackgroundFile = backgroundFileName()
	lastSkinState.Lock()
	lastSkinState.value = result
	lastSkinState.valid = true
	lastSkinState.Unlock()
	return result
}

func optimisticStateSnapshot() (skinSnapshot, bool) {
	optimisticSeek.Lock()
	pending := optimisticSeek.expires.After(time.Now())
	target := optimisticSeek.progress
	if !pending {
		optimisticSeek.expires = time.Time{}
	}
	optimisticSeek.Unlock()
	if !pending {
		return skinSnapshot{}, false
	}
	lastSkinState.RLock()
	snapshot, valid := lastSkinState.value, lastSkinState.valid
	lastSkinState.RUnlock()
	if !valid {
		return skinSnapshot{}, false
	}
	snapshot.Progress = target
	if snapshot.Duration > 0 {
		snapshot.Percentage = target / snapshot.Duration * 100
	}
	snapshot.Position = formatTime(target) + " / " + formatTime(snapshot.Duration)
	snapshot.Elapsed = formatTime(target)
	snapshot.Total = formatTime(snapshot.Duration)
	return snapshot, true
}
func main() {
	if handleFontCommand(os.Args[1:]) {
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "import-playlists" {
		if len(os.Args) != 4 {
			log.Fatal("usage: import-playlists <Google desktop client JSON> <installed playlists.json>")
		}
		if err := importPlaylists(os.Args[2], os.Args[3]); err != nil {
			log.Fatal(err)
		}
		return
	}
	go volumeCommandWorker()
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		log.Fatal(err)
	}
	initializeCoverBackground()
	if cacheFileExists(filepath.Join(artworkCacheDirectory(), "cover_blur_fallback.png")) {
		if exe, err := os.Executable(); err == nil {
			if err := cleanupLegacyArtwork(filepath.Dir(exe)); err != nil {
				log.Printf("legacy artwork cleanup: %v", err)
			}
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /queue/view", queueView)
	mux.HandleFunc("POST /queue/play", queuePlay)
	mux.HandleFunc("POST /bookmarks/{action}", bookmarkAction)
	mux.HandleFunc("GET /state", state)
	mux.HandleFunc("GET /fullscreen/state", fullscreenState)
	mux.HandleFunc("GET /fullscreen/pointer", fullscreenPointer)
	mux.HandleFunc("GET /artwork", artwork)
	mux.HandleFunc("GET /artwork.jpg", artwork)
	mux.HandleFunc("POST /seek", func(w http.ResponseWriter, r *http.Request) {
		percent, err := strconv.ParseFloat(r.URL.Query().Get("percent"), 64)
		if err != nil {
			http.Error(w, "percent must be a number", 400)
			return
		}
		result, err := seekToPercent(percent)
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		defer result.Body.Close()
		w.WriteHeader(result.StatusCode)
		_, _ = io.Copy(w, result.Body)
	})
	mux.HandleFunc("POST /seek-drag/start", func(w http.ResponseWriter, r *http.Request) {
		percent, err := strconv.ParseFloat(r.URL.Query().Get("percent"), 64)
		if err != nil {
			http.Error(w, "percent must be a number", 400)
			return
		}
		if err := startSeekDrag(percent); err != nil {
			http.Error(w, err.Error(), 503)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("GET /seek-drag/state", func(w http.ResponseWriter, _ *http.Request) {
		seekDrag.RLock()
		defer seekDrag.RUnlock()
		write(w, map[string]any{"active": seekDrag.active, "percent": seekDrag.percent})
	})
	mux.HandleFunc("POST /seek-drag/stop", func(w http.ResponseWriter, r *http.Request) {
		percent, err := strconv.ParseFloat(r.URL.Query().Get("percent"), 64)
		if err != nil {
			http.Error(w, "percent must be a number", 400)
			return
		}
		stopSeekDrag()
		if err := seekOptimistically(percent); err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("POST /volume", func(w http.ResponseWriter, r *http.Request) {
		percent, err := strconv.ParseFloat(r.URL.Query().Get("percent"), 64)
		if err != nil {
			http.Error(w, "percent must be a number", 400)
			return
		}
		resp, err := setVolume(percent)
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 && percent > 0 {
			volumeState.Lock()
			volumeState.previous = percent
			volumeState.Unlock()
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	})
	mux.HandleFunc("POST /volume-drag/start", func(w http.ResponseWriter, r *http.Request) {
		percent, err := strconv.ParseFloat(r.URL.Query().Get("percent"), 64)
		if err != nil {
			http.Error(w, "percent must be a number", 400)
			return
		}
		if err := startVolumeDrag(percent); err != nil {
			http.Error(w, err.Error(), 503)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("GET /volume-drag/state", func(w http.ResponseWriter, _ *http.Request) {
		volumeDrag.RLock()
		defer volumeDrag.RUnlock()
		write(w, map[string]any{"active": volumeDrag.active, "percent": volumeDrag.percent})
	})
	mux.HandleFunc("POST /volume-drag/stop", func(w http.ResponseWriter, r *http.Request) {
		percent, err := strconv.ParseFloat(r.URL.Query().Get("percent"), 64)
		if err != nil {
			http.Error(w, "percent must be a number", 400)
			return
		}
		stopVolumeDrag()
		resp, err := setVolume(percent)
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 && percent > 0 {
			volumeState.Lock()
			volumeState.previous = percent
			volumeState.Unlock()
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	})
	mux.HandleFunc("POST /mute", func(w http.ResponseWriter, r *http.Request) {
		volumeState.Lock()
		defer volumeState.Unlock()
		current, err := currentVolume()
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		var target float64
		if current > 0 {
			volumeState.previous = current
			target = 0
		} else {
			target = volumeState.previous
			if target <= 0 {
				target = 50
			}
		}
		resp, err := setVolume(target)
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		defer resp.Body.Close()
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	})
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
	mux.HandleFunc("GET /playlists/labels", func(w http.ResponseWriter, _ *http.Request) {
		exe, err := os.Executable()
		if err != nil {
			http.Error(w, "cannot locate playlist settings", http.StatusInternalServerError)
			return
		}
		data, err := os.ReadFile(filepath.Join(filepath.Dir(exe), "playlists.json"))
		var settings config
		if err != nil || json.Unmarshal(data, &settings) != nil {
			http.Error(w, "cannot read playlist settings", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		var configured [playlistSlots]byte
		for i := range configured {
			label := "-"
			configured[i] = '0'
			if i < len(settings.Playlists) && playlistID(settings.Playlists[i].ID) != "" {
				label = strings.Join(strings.Fields(settings.Playlists[i].Name), " ")
				if label == "" {
					label = fmt.Sprintf("Playlist %d", i+1)
				}
				configured[i] = '1'
			}
			fmt.Fprintln(w, label)
		}
		fmt.Fprintln(w, string(configured[:]))
	})
	mux.HandleFunc("POST /{cmd}", func(w http.ResponseWriter, r *http.Request) {
		cmd := r.PathValue("cmd")
		routes := map[string]string{"toggle": "/track/toggle-play-state", "next": "/track/next", "prev": "/track/prev", "shuffle": "/track/shuffle", "repeat": "/track/repeat", "volup": "/track/volume-up", "voldown": "/track/volume-down", "forward": "/track/forward", "backward": "/track/backward"}
		if cmd == "playlist" {
			http.Error(w, "use /playlist/{index}", 400)
			return
		}
		path, ok := routes[cmd]
		if !ok {
			http.NotFound(w, r)
			return
		}
		body := "{}"
		if cmd == "volup" || cmd == "voldown" {
			body = `{"amount":5}`
		}
		if cmd == "forward" || cmd == "backward" {
			// YTMDesktop2 consumes seek offsets in milliseconds, while state progress is seconds.
			body = `{"time":10000}`
		}
		resp, err := api(path, "POST", body)
		if err != nil {
			write(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		responseBody := readBody(resp)
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(responseBody)
	})
	mux.HandleFunc("POST /like", func(w http.ResponseWriter, r *http.Request) {
		resp, err := api("/track", "GET", "")
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		trackBytes := readBody(resp)
		var track trackInfo
		if resp.StatusCode < 200 || resp.StatusCode >= 300 || json.Unmarshal(trackBytes, &track) != nil {
			http.Error(w, "track state is unavailable", 503)
			return
		}
		payload, _ := json.Marshal(!track.Meta.Liked)
		result, err := api("/track/like", "POST", string(payload))
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		defer result.Body.Close()
		w.WriteHeader(result.StatusCode)
		_, _ = io.Copy(w, result.Body)
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
		id := playlistID(c.Playlists[idx].ID)
		if id == "" {
			http.Error(w, "configure a YouTube playlist ID or share URL first", 400)
			return
		}
		payload, _ := json.Marshal(map[string]any{"playlistId": id, "play": true})
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
