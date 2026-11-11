package main

import (
	"errors"
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
	Items           []queueItem `json:"items"`
	StoreHooked     bool        `json:"storeHooked"`
	CurrentPosition int         `json:"-"`
}

func readQueue() (queueSnapshot, error) {
	lastSkinState.RLock()
	currentID := lastSkinState.value.trackID
	lastSkinState.RUnlock()
	return backend.Queue(currentID)
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
	if err := backend.PlayQueue(index, id); err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, errQueueChanged) {
			status = http.StatusConflict
		}
		http.Error(w, err.Error(), status)
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

	current := queue.CurrentPosition
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
