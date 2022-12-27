package main

import (
	"os"

	"mtoohey.com/q/internal/cmd"
	"mtoohey.com/q/internal/remote"
	"mtoohey.com/q/internal/server"
	"mtoohey.com/q/internal/tui"

	"github.com/alecthomas/kong"
)

type cli struct {
	Remote  remote.Cmd `cmd:"" aliases:"r" help:"Communicate with a server."`
	Server  server.Cmd `cmd:"" aliases:"s" help:"Start a server in the background."`
	Support struct{}   `cmd:"" aliases:"p" help:"Show info about supported formats."`
	TUI     tui.Cmd    `cmd:"" aliases:"t" default:"withargs" help:"Start an interactive TUI."`
}

func main() {
	var flags cli
	parser := kong.Parse(&flags, append(
		cmd.TypeMappers,
		kong.Description("A terminal music player."),
	)...)

	// TODO: support some sort of configuration file

	ctx, err := parser.Parse(os.Args[1:])
	parser.FatalIfErrorf(err)
	parser.FatalIfErrorf(ctx.Run())
}
