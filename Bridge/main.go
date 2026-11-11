package main

import (
	"encoding/json"
	"errors"
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
	volume, err := backend.Volume()
	return volume.Percent, err
}

func setVolume(value float64) (commandResult, error) {
	volumeCommandMu.Lock()
	defer volumeCommandMu.Unlock()
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	return backend.Command(playbackCommand{Action: actionSetVolume, Value: value})
}

func seekToPercent(percent float64) (commandResult, error) {
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
		seconds, err := backend.SeekDuration()
		if err != nil {
			return commandResult{}, err
		}
		duration = seconds
		playbackDuration.Lock()
		playbackDuration.seconds = duration
		playbackDuration.Unlock()
	}
	return backend.Command(playbackCommand{Action: actionSeek, Value: duration * percent / 100})
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
		resp, err := backend.Command(playbackCommand{Action: actionSeek, Value: target})
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
func successful(status int) bool {
	return status >= 200 && status < 300
}

func readBody(body io.ReadCloser) json.RawMessage {
	defer body.Close()
	b, _ := io.ReadAll(body)
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
		data, mimeType := backend.Artwork(videoID)
		if len(data) == 0 && fallbackURL != "" {
			resp, err := client.Get(fallbackURL)
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
	track, err := backend.Track()
	if err != nil && !errors.Is(err, errInvalidTrack) {
		selectBackgroundTrack("", false)
		result.BackgroundFile = backgroundFileName()
		return result
	}
	result.Connected = true
	result.Title = "Nothing playing"
	result.Author = "Choose a playlist to begin"
	if track.Title != "" {
		result.Title = track.Title
		result.Author = track.Artist
		result.Liked = track.Liked
		result.trackID = track.VideoID
		result.artCandidates = track.ArtCandidates
		selectBackgroundTrack(track.VideoID, track.VideoID != "")
		result.Thumbnail = cacheArtwork(track.VideoID, track.ArtworkURL)
	} else {
		selectBackgroundTrack("", false)
		result.Thumbnail = cachedArtworkURL()
	}
	type playbackRead struct {
		state backendPlayback
		err   error
	}
	type volumeRead struct {
		state backendVolume
		err   error
	}
	stateCh := make(chan playbackRead, 1)
	volumeCh := make(chan volumeRead, 1)
	go func() {
		state, err := backend.Playback()
		stateCh <- playbackRead{state, err}
	}()
	go func() {
		volume, err := backend.Volume()
		volumeCh <- volumeRead{volume, err}
	}()
	playbackResult, volumeResult := <-stateCh, <-volumeCh
	if playbackResult.err == nil {
		playback := playbackResult.state
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
					playback.LegacyPercentage = target / playback.Duration * 100
				}
			}
		} else {
			optimisticSeek.expires = time.Time{}
		}
		optimisticSeek.Unlock()
		result.Playing = playback.Status == playbackPlaying
		result.Progress = playback.Progress
		result.Duration = playback.Duration
		result.Percentage = playback.LegacyPercentage
		result.Shuffle = playback.LegacyShuffle
		result.Repeat = playback.LegacyRepeat
		result.Position = formatTime(playback.Progress) + " / " + formatTime(playback.Duration)
		result.Elapsed = formatTime(playback.Progress)
		result.Total = formatTime(playback.Duration)
	}
	if volumeResult.err == nil {
		result.Volume = volumeResult.state.Percent
		result.Muted = volumeResult.state.Muted
		if volumeResult.state.Percent > 0 {
			volumeState.Lock()
			volumeState.previous = volumeResult.state.Percent
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

var skinCommands = map[string]playbackCommand{
	"toggle": {Action: actionToggle}, "next": {Action: actionNext}, "prev": {Action: actionPrevious},
	"shuffle": {Action: actionToggleShuffle}, "repeat": {Action: actionToggleRepeat},
	"volup": {Action: actionAdjustVolume, Value: 5}, "voldown": {Action: actionAdjustVolume, Value: -5},
	"forward": {Action: actionRelativeSeek, Value: 10}, "backward": {Action: actionRelativeSeek, Value: -10},
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
	addr := "127.0.0.1:38471"
	fmt.Println("Ambient Music bridge listening on http://" + addr)
	log.Fatal(http.ListenAndServe(addr, newSkinMux()))
}

func newSkinMux() *http.ServeMux {
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
		if cmd == "playlist" {
			http.Error(w, "use /playlist/{index}", 400)
			return
		}
		command, ok := skinCommands[cmd]
		if !ok {
			http.NotFound(w, r)
			return
		}
		resp, err := backend.Command(command)
		if err != nil {
			write(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		responseBody := readBody(resp.Body)
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(responseBody)
	})
	mux.HandleFunc("POST /like", func(w http.ResponseWriter, r *http.Request) {
		result, err := backend.Command(playbackCommand{Action: actionToggleLike})
		if errors.Is(err, errTrackUnavailable) {
			http.Error(w, err.Error(), 503)
			return
		}
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
		resp, err := backend.Command(playbackCommand{Action: actionPlayPlaylist, ID: id})
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		defer resp.Body.Close()
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	})
	return mux
}
