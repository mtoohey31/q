package main

import (
	"os"

	"mtoohey.com/q/internal/cmd"
	"mtoohey.com/q/internal/remote"
	"mtoohey.com/q/internal/server"
	"mtoohey.com/q/internal/track"
	"mtoohey.com/q/internal/tui"

	"github.com/alecthomas/kong"
)

// TODO(tui): move queue focus index when a song is done playing

// TODO(tui): add queue length indicator (both in # of songs and total duration
// of songs)

// TODO(tui): smoother progress display

// TODO(server): volume normalization

// TODO(server): song gap normalization

// TODO(server/tui): unsynchronized and synchronized lyrics

// TODO(tui): add mouse support

// TODO: proper docs (manpage, details in README, etc.)

// BUG(server): fix audio artifacts when seeking/skipping while paused then
// resuming; we probably need to clear some kind of buffer that's saved
// somewhere

// TODO: make comment style (capitalization and periods) consistent

// TODO: re-implement later

// PERF(server): launch broadcast stuff that doesn't need to be done before
// returning in goroutines. Pay careful attention to what needs to be computed
// before locks are released

// BUG: improve handling of zero values as arguments by using pointers

// BUG(server): fix reshuffle on startup

type cli struct {
	cmd.Globals
	Remote  remote.Cmd `cmd:"" aliases:"r" help:"Communicate with a server."`
	Server  server.Cmd `cmd:"" aliases:"s" help:"Start a server in the background."`
	Support track.Cmd  `cmd:"" aliases:"p" help:"Show info about supported formats."`
	TUI     tui.Cmd    `cmd:"" default:"withargs" aliases:"t" help:"Start an interactive TUI."`
}

func main() {
	var flags cli
	parser := kong.Parse(&flags, append(
		cmd.TypeMappers,
		kong.Description("A terminal music player."),
	)...)

	globalsArgs, err := cmd.LoadGlobalsConfig()
	parser.FatalIfErrorf(err)

	ctx, err := parser.Parse(append(globalsArgs, os.Args[1:]...))
	parser.FatalIfErrorf(err)
	parser.FatalIfErrorf(ctx.Run(flags.Globals))
}
