package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const queuePageSize = 4

type queueItem struct {
	Index       int     `json:"index"`
	VideoID     string  `json:"videoId"`
	Title       string  `json:"title"`
	Artist      string  `json:"artist"`
	Thumbnail   string  `json:"thumbnail"`
	DurationSec float64 `json:"durationSec"`
}

type queueSnapshot struct {
	Items       []queueItem `json:"items"`
	StoreHooked bool        `json:"storeHooked"`
}

func readQueue() (queueSnapshot, error) {
	var queue queueSnapshot
	resp, err := api("/nav/queue", http.MethodGet, "")
	if err != nil {
		return queue, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return queue, fmt.Errorf("queue request returned HTTP %d", resp.StatusCode)
	}
	err = json.NewDecoder(resp.Body).Decode(&queue)
	return queue, err
}

func uniqueQueuePosition(items []queueItem, videoID string) int {
	position := -1
	for i, item := range items {
		if videoID != "" && item.VideoID == videoID {
			if position >= 0 {
				return -1
			}
			position = i
		}
	}
	return position
}

func queuePlay(w http.ResponseWriter, r *http.Request) {
	index, err := strconv.Atoi(r.URL.Query().Get("index"))
	id := r.URL.Query().Get("videoId")
	if err != nil || index < 0 || len(id) != 11 || !playlistIDPattern.MatchString(id) {
		http.Error(w, "invalid queue selection", http.StatusBadRequest)
		return
	}
	queue, err := readQueue()
	if err != nil {
		http.Error(w, "queue unavailable", http.StatusBadGateway)
		return
	}
	selected := -1
	for i, item := range queue.Items {
		if item.Index == index && item.VideoID == id {
			selected = i
			break
		}
	}
	if selected < 0 {
		http.Error(w, "queue changed; select the track again", http.StatusConflict)
		return
	}
	trackResponse, err := api("/track/state", http.MethodGet, "")
	if err != nil {
		http.Error(w, "playback state unavailable", http.StatusBadGateway)
		return
	}
	var state struct {
		ID string `json:"id"`
	}
	decodeErr := json.NewDecoder(trackResponse.Body).Decode(&state)
	trackResponse.Body.Close()
	if trackResponse.StatusCode != http.StatusOK || decodeErr != nil || state.ID == "" {
		http.Error(w, "playback state unavailable", http.StatusBadGateway)
		return
	}

	path := "/nav/watch"
	payload, _ := json.Marshal(map[string]string{"videoId": id})
	body := string(payload)
	current := uniqueQueuePosition(queue.Items, state.ID)
	switch {
	case current == selected:
		path, body = "/track/play", "{}"
	case current >= 0 && selected == current+1:
		path, body = "/track/next", "{}"
	}
	resp, err := api(path, http.MethodPost, body)
	if err != nil {
		http.Error(w, "queue playback command failed", http.StatusBadGateway)
		return
	}
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		http.Error(w, "queue playback command failed", http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func queueThumbnail(raw string) string {
	if strings.ContainsAny(raw, "\"[]# \t\r\n") {
		return ""
	}
	link, err := url.Parse(raw)
	if err != nil || link.Scheme != "https" || link.User != nil {
		return ""
	}
	host := link.Hostname()
	if host == "i.ytimg.com" || host == "yt3.ggpht.com" || strings.HasSuffix(host, ".googleusercontent.com") {
		return raw
	}
	return ""
}

func queueView(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	header := "Queue unavailable"
	var rows [queuePageSize][6]string
	prev, next := 0, 0
	activeRow, highlightAlpha := 1, 0
	visibleRows := 0
	defer func() {
		fmt.Fprintln(w, header)
		for _, row := range rows {
			for _, field := range row {
				fmt.Fprintln(w, field)
			}
		}
		fmt.Fprintf(w, "%d\n%d\n%d\n%d\n%d\n", prev, next, activeRow, highlightAlpha, visibleRows)
	}()

	queue, err := readQueue()
	if err != nil {
		return
	}
	if len(queue.Items) == 0 {
		if queue.StoreHooked {
			header = "Queue is empty"
		} else {
			header = "Start playback to load the queue"
		}
		return
	}

	lastSkinState.RLock()
	currentID := lastSkinState.value.trackID
	lastSkinState.RUnlock()
	current := uniqueQueuePosition(queue.Items, currentID)
	pages := (len(queue.Items)-1)/queuePageSize + 1
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = max(0, current)/queuePageSize + 1
	}
	page = min(page, pages)
	prev, next = max(1, page-1), min(pages, page+1)
	header = fmt.Sprintf("Play queue  %d / %d  ·  %d tracks", page, pages, len(queue.Items))
	start := (page - 1) * queuePageSize
	pageItems := queue.Items[start:min(start+queuePageSize, len(queue.Items))]
	visibleRows = len(pageItems)
	for i, item := range pageItems {
		title := strings.Join(strings.Fields(item.Title), " ")
		if title == "" {
			title = "Unknown track"
		}
		artist := strings.Join(strings.Fields(item.Artist), " ")
		duration := ""
		if item.DurationSec > 0 {
			seconds := int(item.DurationSec)
			if seconds >= 3600 {
				duration = fmt.Sprintf("%d:%02d:%02d", seconds/3600, seconds/60%60, seconds%60)
			} else {
				duration = formatTime(item.DurationSec)
			}
		}
		if start+i == current {
			activeRow, highlightAlpha = i+1, 115
		}
		rows[i] = [6]string{title, artist, duration, queueThumbnail(item.Thumbnail), strconv.Itoa(item.Index), ""}
		if len(item.VideoID) == 11 && playlistIDPattern.MatchString(item.VideoID) {
			rows[i][5] = item.VideoID
		}
	}
}
