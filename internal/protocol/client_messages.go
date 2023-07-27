package protocol

import (
	"encoding/gob"
	"time"
)

func init() {
	gob.Register(Skip(0))
	gob.Register(Seek(0))
	gob.Register(Remove(0))
	gob.Register(RemoveAll{})
	gob.Register(Insert{})
	gob.Register(Query(""))
	gob.Register(QueryResults(nil))
	gob.Register(Reshuffle{})
	gob.Register(Later(0))
	gob.Register(Jump(0))
}

// Skip requests that the given number of songs be skipped (may be negative to
// request reverse skip).
type Skip int

// Seek requests that the given duration in the current song be seeked to.
type Seek time.Duration

// Remove requests that the item at the given index be removed from the queue.
type Remove int

// RemoveAll requests that the current queue be cleared.
type RemoveAll struct{}

// Insert requests that the file at Path be inserted into the queue at Index.
type Insert struct {
	// Index is the index within the queue to insert into.
	Index int

	// Path is the path of the song to start playing.
	Path string
}

// Query requests that the server report to the requesting client, the paths of
// all songs that match the given query.
type Query string

// Reshuffle requests that the server reshuffle the current queue (excluding
// the now-playing song, which should continue to play). This request is valid
// regardless of the current shuffle state.
type Reshuffle struct{}

// Later requests that the track at the given index be relocated to later in
// the queue. Its new index will be determined similarly to how it would be
// determined if queue repeat was enabled, the track was at the start of the
// queue, and it had just been skipped, but with the added guarantee that the
// new index must be greater than the starting index (which might not happen if
// not explicitly required in the case where the queue is being shuffled).
type Later int

// Jump requests that the track at the given index become the new head of the
// queue.
type Jump int
