package cmd

import "github.com/faiface/beep"

// Globals contains constant values that apply to multiple commands.
type Globals struct {
	// SampleRate is the Sample rate to use for the player as a whole. Audio
	// files with different sample rates will be resampled.
	SampleRate beep.SampleRate `short:"t" default:"44100" help:"Sample rate to use for the player as a whole. Audio files with different sample rates will be resampled."`
	// MusicDir is the directory containing music files.
	MusicDir string `short:"m" default:"." type:"path" help:"Directory containing music files."`
	// UnixSocket is the path of the socket to bind or connect to, depending on
	// the command. No socket is used if this flag is not provided.
	UnixSocket string `short:"u" help:"The path of the socket to bind or connect to, depending on the command. No socket is used if this flag is not provided."`
}
