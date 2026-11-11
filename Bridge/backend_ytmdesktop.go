package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type ytmDesktopBackend struct {
	mu        sync.Mutex
	lastError playbackError
}

var apiBase = env("YTM_API_URL", "http://127.0.0.1:13091")

func (b *ytmDesktopBackend) LastError() playbackError {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.lastError
}

func (b *ytmDesktopBackend) recordError(kind string, err error) {
	if err != nil {
		b.mu.Lock()
		b.lastError = playbackError{Kind: kind, Description: err.Error()}
		b.mu.Unlock()
	}
}

func (b *ytmDesktopBackend) request(path, method, body string) (*http.Response, error) {
	req, err := b.newRequest(path, method, body)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func (*ytmDesktopBackend) newRequest(path, method, body string) (*http.Request, error) {
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

type ytmTrack struct {
	ID    string `json:"id"`
	Video struct {
		VideoID   string          `json:"videoId"`
		Title     string          `json:"title"`
		Author    string          `json:"author"`
		Length    json.RawMessage `json:"lengthSeconds"`
		Thumbnail struct {
			Thumbnails []trackThumbnail `json:"thumbnails"`
		} `json:"thumbnail"`
	} `json:"video"`
	Meta struct {
		Thumbnail string `json:"thumbnail"`
		Liked     bool   `json:"liked"`
	} `json:"meta"`
}

func (b *ytmDesktopBackend) Track() (_ backendTrack, err error) {
	defer func() { b.recordError("track", err) }()
	resp, err := b.request("/track", http.MethodGet, "")
	if err != nil {
		return backendTrack{}, err
	}
	body := readBody(resp.Body)
	if !successful(resp.StatusCode) {
		return backendTrack{}, errTrackUnavailable
	}
	var track ytmTrack
	if err := json.Unmarshal(body, &track); err != nil {
		return backendTrack{}, errInvalidTrack
	}
	id := track.ID
	if id == "" {
		id = track.Video.VideoID
	}
	art := track.Meta.Thumbnail
	candidates := track.Video.Thumbnail.Thumbnails
	if art != "" {
		candidates = append(candidates, trackThumbnail{URL: art})
	} else if len(candidates) > 0 {
		art = candidates[len(candidates)-1].URL
	}
	duration, _ := strconv.ParseFloat(strings.Trim(string(track.Video.Length), "\""), 64)
	return backendTrack{VideoID: id, Title: track.Video.Title, Artist: track.Video.Author,
		ArtworkURL: art, ArtCandidates: candidates, Duration: duration, Liked: track.Meta.Liked}, nil
}

type ytmPlayback struct {
	ID         json.RawMessage `json:"id"`
	Playing    bool            `json:"playing"`
	Progress   float64         `json:"progress"`
	Duration   float64         `json:"duration"`
	Percentage float64         `json:"percentage"`
	Shuffle    bool            `json:"shuffle"`
	Loop       bool            `json:"loop"`
}

func (b *ytmDesktopBackend) Playback() (_ backendPlayback, err error) {
	defer func() { b.recordError("playback state", err) }()
	resp, err := b.request("/track/state", http.MethodGet, "")
	if err != nil {
		return backendPlayback{}, err
	}
	defer resp.Body.Close()
	var state ytmPlayback
	if !successful(resp.StatusCode) || json.NewDecoder(resp.Body).Decode(&state) != nil {
		return backendPlayback{}, errPlaybackUnavailable
	}
	status := playbackStopped
	var id string
	_ = json.Unmarshal(state.ID, &id)
	switch {
	case state.Playing:
		status = playbackPlaying
	case id != "" || state.Duration > 0:
		status = playbackPaused
	}
	return backendPlayback{Status: status, VideoID: id, Progress: state.Progress, Duration: state.Duration,
		Repeat: repeatUnknown, Shuffle: shuffleUnknown, LegacyPercentage: state.Percentage,
		LegacyShuffle: state.Shuffle, LegacyRepeat: state.Loop}, nil
}

func (b *ytmDesktopBackend) SeekDuration() (_ float64, err error) {
	defer func() { b.recordError("seek duration", err) }()
	resp, err := b.request("/track/state", http.MethodGet, "")
	if err != nil {
		return 0, err
	}
	body := readBody(resp.Body)
	var state ytmPlayback
	if !successful(resp.StatusCode) || json.Unmarshal(body, &state) != nil || state.Duration <= 0 {
		return 0, errDurationUnavailable
	}
	return state.Duration, nil
}

func (b *ytmDesktopBackend) Volume() (_ backendVolume, err error) {
	defer func() { b.recordError("volume", err) }()
	resp, err := b.request("/track/volume", http.MethodPost, "{}")
	if err != nil {
		return backendVolume{}, err
	}
	defer resp.Body.Close()
	var volume struct {
		Percent float64 `json:"volume"`
	}
	if !successful(resp.StatusCode) || json.NewDecoder(resp.Body).Decode(&volume) != nil {
		return backendVolume{}, errors.New("volume is unavailable")
	}
	return backendVolume{Percent: volume.Percent, Muted: volume.Percent <= 0}, nil
}

func (b *ytmDesktopBackend) Queue(currentVideoID string) (_ queueSnapshot, err error) {
	defer func() { b.recordError("queue", err) }()
	var queue queueSnapshot
	resp, err := b.request("/nav/queue", http.MethodGet, "")
	if err != nil {
		return queue, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return queue, fmt.Errorf("queue request returned HTTP %d", resp.StatusCode)
	}
	err = json.NewDecoder(resp.Body).Decode(&queue)
	queue.CurrentPosition = uniqueQueuePosition(queue.Items, currentVideoID)
	return queue, err
}

func (b *ytmDesktopBackend) PlayQueue(index int, videoID string) (err error) {
	defer func() { b.recordError("queue playback", err) }()
	queue, err := b.Queue("")
	if err != nil {
		return errQueueUnavailable
	}
	selected := -1
	for i, item := range queue.Items {
		if item.Index == index && item.VideoID == videoID {
			selected = i
			break
		}
	}
	if selected < 0 {
		return errQueueChanged
	}
	resp, err := b.request("/track/state", http.MethodGet, "")
	if err != nil {
		return errPlaybackUnavailable
	}
	var state struct {
		ID string `json:"id"`
	}
	decodeErr := json.NewDecoder(resp.Body).Decode(&state)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || decodeErr != nil || state.ID == "" {
		return errPlaybackUnavailable
	}
	command := playbackCommand{Action: actionPlayTrack, ID: videoID}
	current := uniqueQueuePosition(queue.Items, state.ID)
	switch {
	case current == selected:
		command.Action = actionPlay
	case current >= 0 && selected == current+1:
		command.Action = actionNext
	}
	result, err := b.Command(command)
	if err != nil {
		return errQueueCommand
	}
	result.Body.Close()
	if !successful(result.StatusCode) {
		return errQueueCommand
	}
	return nil
}

func (*ytmDesktopBackend) Capabilities() playbackCapabilities {
	return playbackCapabilities{BasicControls: true, PlayPlaylist: true, ViewQueue: true, YouTubeHistory: true}
}

var ytmCommandPaths = map[playbackAction]string{
	actionPlay: "/track/play", actionPause: "/track/pause", actionToggle: "/track/toggle-play-state",
	actionNext: "/track/next", actionPrevious: "/track/prev", actionSeek: "/track/seek",
	actionSetVolume: "/track/volume", actionPlayPlaylist: "/nav/playlist", actionPlayBookmark: "/nav/playlist",
	actionPlayTrack: "/nav/watch", actionToggleRepeat: "/track/repeat", actionToggleShuffle: "/track/shuffle",
	actionToggleLike: "/track/like",
}

func (b *ytmDesktopBackend) Command(command playbackCommand) (result commandResult, err error) {
	defer func() {
		if err != nil {
			b.recordError(string(command.Action), err)
		} else if !successful(result.StatusCode) {
			b.recordError(string(command.Action), fmt.Errorf("command returned HTTP %d", result.StatusCode))
		}
	}()
	path := ytmCommandPaths[command.Action]
	var payload any = struct{}{}
	switch command.Action {
	case actionSeek:
		payload = map[string]any{"time": command.Value * 1000, "type": "seek"}
	case actionSetVolume:
		payload = map[string]any{"volume": command.Value}
	case actionAdjustVolume:
		path = "/track/volume-up"
		if command.Value < 0 {
			path = "/track/volume-down"
		}
		payload = map[string]any{"amount": max(command.Value, -command.Value)}
	case actionRelativeSeek:
		path = "/track/forward"
		if command.Value < 0 {
			path = "/track/backward"
		}
		payload = map[string]any{"time": max(command.Value, -command.Value) * 1000}
	case actionPlayPlaylist, actionPlayBookmark:
		payload = map[string]any{"playlistId": command.ID, "play": true}
	case actionPlayTrack:
		payload = map[string]string{"videoId": command.ID}
	case actionToggleLike:
		track, err := b.Track()
		if errors.Is(err, errInvalidTrack) {
			err = errTrackUnavailable
		}
		if err != nil {
			return commandResult{}, err
		}
		payload = !track.Liked
	}
	if path == "" {
		return commandResult{}, errUnsupportedPlayback
	}
	body, _ := json.Marshal(payload)
	req, err := b.newRequest(path, http.MethodPost, string(body))
	if err != nil {
		return commandResult{}, err
	}
	httpClient := client
	if command.Action == actionPlayBookmark {
		host := req.URL.Hostname()
		if !strings.EqualFold(host, "localhost") && !net.ParseIP(host).IsLoopback() {
			return commandResult{}, errors.New("bookmark playback requires a local API")
		}
		localClient := *client
		localClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		httpClient = &localClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return commandResult{}, err
	}
	return commandResult{StatusCode: resp.StatusCode, Body: resp.Body}, nil
}

func (b *ytmDesktopBackend) Artwork(videoID string) ([]byte, string) {
	resp, err := b.request("/track/thumbnail?id="+url.QueryEscape(videoID), http.MethodGet, "")
	if err != nil {
		return nil, ""
	}
	defer resp.Body.Close()
	if !successful(resp.StatusCode) {
		return nil, ""
	}
	data, _ := io.ReadAll(resp.Body)
	return data, resp.Header.Get("Content-Type")
}
