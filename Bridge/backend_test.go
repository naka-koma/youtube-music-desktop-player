package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBackendPreservesSkinHTTP(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/track":
			io.WriteString(w, `{"video":{"title":"Track","author":"Artist","lengthSeconds":"100"},"meta":{"liked":true}}`)
		case r.Method == http.MethodGet && r.URL.Path == "/track/state":
			io.WriteString(w, `{"playing":true,"progress":25,"duration":100,"percentage":25,"shuffle":true,"loop":true}`)
		case r.Method == http.MethodPost && r.URL.Path == "/track/volume":
			io.WriteString(w, `{"volume":45}`)
		case r.Method == http.MethodPost && r.URL.Path == "/track/seek":
			body, _ := io.ReadAll(r.Body)
			if string(body) != `{"time":50000,"type":"seek"}` {
				t.Errorf("unexpected seek payload %q", body)
			}
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, "seek accepted")
		case r.Method == http.MethodPost && r.URL.Path == "/track/shuffle":
			io.WriteString(w, `{"ok":true}`)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	originalAPI, originalDir, originalBackend := apiBase, dataDir, backend
	apiBase, dataDir, backend = upstream.URL, t.TempDir(), &ytmDesktopBackend{}
	fullscreenCurrent.Lock()
	originalSnapshot, originalUpdate := fullscreenCurrent.snapshot, fullscreenCurrent.updatedAt
	fullscreenCurrent.updatedAt = time.Time{}
	fullscreenCurrent.Unlock()
	lastSkinState.Lock()
	originalSkin, originalValid := lastSkinState.value, lastSkinState.valid
	lastSkinState.Unlock()
	playbackDuration.Lock()
	originalDuration := playbackDuration.seconds
	playbackDuration.seconds = 0
	playbackDuration.Unlock()
	volumeState.Lock()
	originalVolume := volumeState.previous
	volumeState.Unlock()
	t.Cleanup(func() {
		apiBase, dataDir, backend = originalAPI, originalDir, originalBackend
		fullscreenCurrent.Lock()
		fullscreenCurrent.snapshot, fullscreenCurrent.updatedAt = originalSnapshot, originalUpdate
		fullscreenCurrent.Unlock()
		lastSkinState.Lock()
		lastSkinState.value, lastSkinState.valid = originalSkin, originalValid
		lastSkinState.Unlock()
		playbackDuration.Lock()
		playbackDuration.seconds = originalDuration
		playbackDuration.Unlock()
		volumeState.Lock()
		volumeState.previous = originalVolume
		volumeState.Unlock()
	})

	mux := newSkinMux()
	request := func(method, path string) *httptest.ResponseRecorder {
		result := httptest.NewRecorder()
		mux.ServeHTTP(result, httptest.NewRequest(method, path, nil))
		return result
	}
	state := request(http.MethodGet, "/state")
	var snapshot skinSnapshot
	if err := json.Unmarshal(state.Body.Bytes(), &snapshot); err != nil ||
		state.Code != http.StatusOK || !snapshot.Connected || !snapshot.Playing || !snapshot.Repeat ||
		!snapshot.Shuffle || !snapshot.Liked || snapshot.Title != "Track" || snapshot.Position != "0:25 / 1:40" ||
		snapshot.Percentage != 25 || snapshot.Volume != 45 {
		t.Fatalf("unexpected /state result: %d %s", state.Code, state.Body.String())
	}
	fullscreen := request(http.MethodGet, "/fullscreen/state")
	if fullscreen.Code != http.StatusOK ||
		!strings.HasPrefix(fullscreen.Body.String(), "545241434b|415254495354||303a3235202f20313a3430|true|false|25.000|45.000|") {
		t.Fatalf("unexpected fullscreen result: %d %s", fullscreen.Code, fullscreen.Body.String())
	}
	playbackDuration.Lock()
	playbackDuration.seconds = 0
	playbackDuration.Unlock()
	if result := request(http.MethodPost, "/seek?percent=50"); result.Code != http.StatusAccepted || result.Body.String() != "seek accepted" {
		t.Fatalf("unexpected seek result: %d %s", result.Code, result.Body.String())
	}
	if result := request(http.MethodPost, "/shuffle"); result.Code != http.StatusOK || result.Body.String() != `{"ok":true}` {
		t.Fatalf("unexpected shuffle result: %d %s", result.Code, result.Body.String())
	}
	track, err := backend.Track()
	if err != nil || track.Duration != 100 {
		t.Fatalf("unexpected backend track: %+v (%v)", track, err)
	}
	playback, err := backend.Playback()
	if err != nil || playback.Status != playbackPlaying || playback.Repeat != repeatUnknown || playback.Shuffle != shuffleUnknown {
		t.Fatalf("unexpected backend playback: %+v (%v)", playback, err)
	}
	capabilities := backend.Capabilities()
	if !capabilities.BasicControls || !capabilities.ViewQueue || !capabilities.PlayPlaylist || !capabilities.YouTubeHistory ||
		capabilities.RepeatState || capabilities.ShuffleState || capabilities.EditQueue || capabilities.PlaylistTracks {
		t.Fatalf("unexpected capabilities: %+v", capabilities)
	}
	if _, err := backend.Command(playbackCommand{Action: actionMoveQueue}); !errors.Is(err, errUnsupportedPlayback) {
		t.Fatalf("unexpected queue edit result: %v", err)
	}
	if got := backend.LastError(); got.Kind != string(actionMoveQueue) || got.Description == "" {
		t.Fatalf("unexpected backend error: %+v", got)
	}
}
