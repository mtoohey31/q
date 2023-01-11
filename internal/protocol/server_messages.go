package protocol

import (
	"encoding/gob"
	"image"
	"time"
)

func init() {
	gob.Register(Error(""))
	gob.Register(State{})
	gob.Register(&image.Rectangle{})
	gob.Register(&image.RGBA{})
	gob.Register(&image.RGBA64{})
	gob.Register(&image.NRGBA{})
	gob.Register(&image.NRGBA64{})
	gob.Register(&image.Alpha{})
	gob.Register(&image.Alpha16{})
	gob.Register(&image.Gray{})
	gob.Register(&image.Gray16{})
	gob.Register(&image.CMYK{})
	gob.Register(&image.Paletted{})
	gob.Register(&image.Uniform{})
	gob.Register(&image.YCbCr{})
	gob.Register(&image.NYCbCrA{})
	gob.Register(NowPlayingState{})
	gob.Register(ProgressState{})
	gob.Register(ShuffleIdxState(0))
	gob.Register(QueueState{})
	gob.Register(Removed(""))
}

// Error reports an error that may be general, or specific to this client.
type Error string

func (e Error) Error() string {
	return string(e)
}

// State contains the complete current state of the player. This is sent once
// to each newly connected client.
type State struct {
	// NowPlaying is the current now playing state.
	NowPlaying NowPlayingState

	// Pause is the current pause state.
	Pause PauseState

	// Progress is the current progress state.
	Progress ProgressState

	// Repeat is the current repeat state.
	Repeat RepeatState

	// Shuffle is the current shuffle state.
	Shuffle ShuffleState

	// ShuffleIdx is the current position of the shuffle index.
	ShuffleIdx ShuffleIdxState

	// Queue is the current queue state.
	Queue QueueState

	// Version is the version of the server. This should be checked to ensure
	// compatability with the client.
	Version string
}

// NowPlayingState contains information about the current song.
type NowPlayingState struct {
	// Title is the title of the current song, if it was available in the
	// metadata. Otherwise, this field contains the basename of the path of the
	// audio file.
	Title string

	// Artist is the artist of the current song, if it was available in the
	// metadata. Otherwise, this field is empty.
	Artist string

	// Cover is the cover art for the current song, if it was available in the
	// metadata. Otherwise, this field is nil.
	Cover image.Image
}

// ProgressState contains information about the player's progress through the
// current song.
type ProgressState struct {
	// Current is the player's position within the current song.
	Current time.Duration
	// Total is the total length of the song.
	Total time.Duration
}

// ShuffleIdxState is the index within the queue of the first song that was
// played during the current repeat of the queue.
type ShuffleIdxState int

// QueueState contains information about the current queue.
type QueueState []QueueItem

// QueueItem contains the friendly name of the queue item.
type QueueItem string

// QueryResults returns path results of a query.
type QueryResults []string

// Removed reports to the client it was sent to that the song whose path is the
// contents of the message was successfully removed from the queue in response
// to a message from this client. This message is only sent in response to
// Remove; RemoveAll receives no response.
type Removed string
