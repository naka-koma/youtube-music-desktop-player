package main

import (
	"errors"
	"io"
)

// Playback state is read in parts so polling the skin does not fetch the queue.
type playbackBackend interface {
	Track() (backendTrack, error)
	Playback() (backendPlayback, error)
	SeekDuration() (float64, error)
	Volume() (backendVolume, error)
	Queue(currentVideoID string) (queueSnapshot, error)
	Capabilities() playbackCapabilities
	LastError() playbackError
	Command(playbackCommand) (commandResult, error)
	PlayQueue(index int, videoID string) error
	Artwork(videoID string) (data []byte, contentType string)
}

type playbackStatus string

const (
	playbackStopped playbackStatus = "stopped"
	playbackPaused  playbackStatus = "paused"
	playbackPlaying playbackStatus = "playing"
)

type repeatMode string

const (
	repeatUnknown repeatMode = ""
	repeatOff     repeatMode = "off"
	repeatAll     repeatMode = "all"
	repeatOne     repeatMode = "one"
)

type shuffleMode string

const (
	shuffleUnknown shuffleMode = ""
	shuffleOff     shuffleMode = "off"
	shuffleOn      shuffleMode = "on"
)

type backendTrack struct {
	VideoID       string
	Title         string
	Artist        string
	ArtworkURL    string
	ArtCandidates []trackThumbnail
	Duration      float64
	Liked         bool
}

type backendPlayback struct {
	Status   playbackStatus
	VideoID  string
	Progress float64
	Duration float64
	Repeat   repeatMode
	Shuffle  shuffleMode

	// Preserve the fields read by the existing skin's /state parser.
	LegacyPercentage float64
	LegacyShuffle    bool
	LegacyRepeat     bool
}

type backendVolume struct {
	Percent float64
	Muted   bool
}

type playbackError struct {
	Kind        string
	Description string
}

type playbackCapabilities struct {
	BasicControls, PlayPlaylist, ViewQueue, RepeatState, ShuffleState bool
	EditQueue, PlaylistTracks, YouTubeHistory                         bool
}

type playbackAction string

const (
	actionPlay          playbackAction = "play"
	actionPause         playbackAction = "pause"
	actionToggle        playbackAction = "toggle"
	actionNext          playbackAction = "next"
	actionPrevious      playbackAction = "previous"
	actionSeek          playbackAction = "seek"
	actionSetVolume     playbackAction = "set volume"
	actionAdjustVolume  playbackAction = "adjust volume"
	actionRelativeSeek  playbackAction = "relative seek"
	actionPlayPlaylist  playbackAction = "play playlist"
	actionPlayBookmark  playbackAction = "play bookmark"
	actionPlayTrack     playbackAction = "play track"
	actionToggleRepeat  playbackAction = "toggle repeat"
	actionToggleShuffle playbackAction = "toggle shuffle"
	actionSetRepeat     playbackAction = "set repeat"
	actionSetShuffle    playbackAction = "set shuffle"
	actionToggleLike    playbackAction = "toggle like"
	actionAddToQueue    playbackAction = "add to queue"
	actionRemoveQueue   playbackAction = "remove from queue"
	actionMoveQueue     playbackAction = "move in queue"
)

type playbackCommand struct {
	Action             playbackAction
	ID                 string
	Value              float64 // Seconds for seek and percent for volume.
	Repeat             repeatMode
	Shuffle            shuffleMode
	Index, TargetIndex int
}

// The existing HTTP handlers forward command status and body to Rainmeter.
type commandResult struct {
	StatusCode int
	Body       io.ReadCloser
}

var (
	errUnsupportedPlayback = errors.New("playback operation is unsupported")
	errTrackUnavailable    = errors.New("track state is unavailable")
	errInvalidTrack        = errors.New("invalid track response")
	errPlaybackUnavailable = errors.New("playback state unavailable")
	errDurationUnavailable = errors.New("track duration is unavailable")
	errQueueUnavailable    = errors.New("queue unavailable")
	errQueueChanged        = errors.New("queue changed; select the track again")
	errQueueCommand        = errors.New("queue playback command failed")
)

var backend playbackBackend = &ytmDesktopBackend{}
