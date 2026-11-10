package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestQueuePlaySelection(t *testing.T) {
	for _, tc := range []struct {
		name, query, wantPath string
		wantStatus            int
	}{
		{"next keeps queue", "index=8&videoId=BBBBBBBBBBB", "/track/next", http.StatusNoContent},
		{"jump opens track", "index=9&videoId=CCCCCCCCCCC", "/nav/watch", http.StatusNoContent},
		{"stale row", "index=8&videoId=CCCCCCCCCCC", "", http.StatusConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			commands := make(chan string, 1)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/nav/queue":
					_, _ = io.WriteString(w, `{"items":[{"index":7,"videoId":"AAAAAAAAAAA"},{"index":8,"videoId":"BBBBBBBBBBB"},{"index":9,"videoId":"CCCCCCCCCCC"}]}`)
				case "/track/state":
					_, _ = io.WriteString(w, `{"id":"AAAAAAAAAAA"}`)
				default:
					if r.Method != http.MethodPost {
						t.Errorf("expected POST, got %s", r.Method)
					}
					if r.URL.Path == "/nav/watch" {
						var body map[string]string
						if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["videoId"] != "CCCCCCCCCCC" {
							t.Errorf("unexpected watch body: %v (%v)", body, err)
						}
					}
					select {
					case commands <- r.URL.Path:
					default:
						t.Error("multiple playback commands")
					}
					w.WriteHeader(http.StatusOK)
				}
			}))
			defer upstream.Close()
			originalAPI, originalDataDir := apiBase, dataDir
			apiBase, dataDir = upstream.URL, t.TempDir()
			defer func() { apiBase, dataDir = originalAPI, originalDataDir }()
			res := httptest.NewRecorder()
			queuePlay(res, httptest.NewRequest(http.MethodPost, "/queue/play?"+tc.query, nil))
			gotPath := ""
			select {
			case gotPath = <-commands:
			default:
			}
			if res.Code != tc.wantStatus || gotPath != tc.wantPath {
				t.Fatalf("got status %d, command %q; want %d, %q", res.Code, gotPath, tc.wantStatus, tc.wantPath)
			}
		})
	}
}
