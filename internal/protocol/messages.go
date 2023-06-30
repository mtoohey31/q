package protocol

import "encoding/gob"

func init() {
	gob.Register(PauseState(false))
	gob.Register(RepeatState(0))
	gob.Register(ShuffleState(false))
}

// This file contains messages that can be sent by the server as a notification
// of an updated state, but can also be sent by clients to request a change to
// the current state.

// Message is equivalent to any, but expresses that the given value is expected
// to be a protocol message.
type Message any

// PauseState indicates whether the player is currently paused.
type PauseState bool

// RepeatState indicates the current repeat mode of the player.
type RepeatState uint8

const (
	// RepeatStateNone means that songs will not be repeated.
	RepeatStateNone RepeatState = iota

	// RepeatStateQueue means that the whole queue will be repeated, after it
	// has been played through.
	RepeatStateQueue

	// RepeatStateTrack means that the current track will be repeated.
	RepeatStateTrack
)

// Next returns the repeat state following the current one.
func (r RepeatState) Next() RepeatState {
	return (r + 1) % 3
}

// Prev returns the repeat state preceding the current one.
func (r RepeatState) Prev() RepeatState {
	return (r + 2) % 3
}

// ShuffleState indicates whether the queue is being shuffled.
type ShuffleState bool
