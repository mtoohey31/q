package remote

import (
	"fmt"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/alecthomas/kong"
	"mtoohey.com/q/internal/protocol"
	"mtoohey.com/q/internal/server/unixsocketconn"
	"mtoohey.com/q/internal/version"
)

func (c *Cmd) Run(ctx *kong.Context) (err error) {
	conn, err := unixsocketconn.NewUnixSocketClientConn(c.UnixSocket)
	if err != nil {
		return fmt.Errorf("failed to create connection: %w", err)
	}
	defer func() {
		closeErr := conn.Close()

		if err == nil {
			err = closeErr
		}
	}()

	im, err := conn.Receive()
	if err != nil {
		return fmt.Errorf("failed to receive initial state: %w", err)
	}
	state, ok := im.(protocol.State)
	if !ok {
		return fmt.Errorf("initial message was of unexpected type %T", im)
	}

	if state.Version != version.Version {
		return fmt.Errorf(`server version "%s" does not match remote version "%s"`,
			state.Version, version.Version)
	}

	var m protocol.Message
	command := strings.Split(ctx.Command(), " ")[1]
	switch command {
	case "state":
		templateText := c.State.Template
		if templateText == "" {
			templateText = "{{ if .Pause }} {{else}}契{{end}}{{ if .NowPlaying }}{{ .NowPlaying.Title }}{{ if .NowPlaying.Artist }} - {{ .NowPlaying.Artist }}{{end}}{{else}}nothing playing{{end}}\n"
		} else {
			// kong workaround: you can either set default:"" which only works
			// for subcommands without arguments, or default:"withargs" which
			// executes the command even if there are arguments and they are
			// provided. the behahviour we want here is for this subcommand to
			// be used by default only when no arguments are provided, so we
			// check here that "state" was in the original arguments when the
			// template is non-empty, because otherwise this results in the
			// very confusing behaviour of just printing the user's argument
			// back out for them..

			stateFound := false
			for _, arg := range ctx.Args {
				if arg == "state" {
					stateFound = true
					break
				}
			}
			if !stateFound {
				return fmt.Errorf("expected subcommand")
			}
		}

		t, err := template.New("state").Parse(templateText)
		if err != nil {
			return err
		}
		return t.Execute(os.Stdout, state)

	case "play":
		m = protocol.PauseState(false)

	case "pause":
		m = protocol.PauseState(true)

	case "repeat":
		m = c.Repeat.RepeatState

	case "shuffle":
		m = c.Shuffle.ShuffleState

	case "skip":
		m = c.Skip.Songs

	case "seek":
		d, err := time.ParseDuration(c.Seek.By)
		if err != nil {
			return err
		}

		// must be at least length 1 because "" is an invalid duration
		switch c.Seek.By[0] {
		case '+', '-':
			d = state.Progress.Current + d
		}
		m = protocol.Seek(d)

	case "remove":
		m = protocol.Remove(c.Remove.Index)

	case "remove-all":
		m = protocol.RemoveAll{}

	case "insert":
		m = protocol.Insert{
			Index: c.Insert.Index,
			Path:  c.Insert.Path,
		}

	default:
		panic(fmt.Sprintf(`unhandled command "%s"`, command))
	}

	if err := conn.Send(m); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

type Cmd struct {
	UnixSocket string `short:"u" required:"" help:"Path of the socket to connect to."`

	State struct {
		Template string `arg:"" optional:"" help:"Template using Go text/template syntax."`
	} `cmd:"" default:"withargs" help:"Display current state."`

	Play   struct{} `cmd:"" help:"Resume playback."`
	Pause  struct{} `cmd:"" help:"Pause playback."`
	Repeat struct {
		RepeatState protocol.RepeatState `arg:"" help:"New repeat state."`
	} `cmd:"" help:"Set repeat state."`
	Shuffle struct {
		ShuffleState protocol.ShuffleState `arg:"" type:"boolarg" help:"New shuffle state."`
	} `cmd:"" help:"Set shuffle state."`
	Skip struct {
		Songs protocol.Skip `arg:"" default:"1" help:"Number of songs to skip."`
	} `cmd:"" help:"Skip song(s)."`
	Seek struct {
		By string `arg:"" help:"Seek either +/- the current time, or absolutely if no prefix is given."`
	} `cmd:"" help:"Seek within the current song."`
	Remove struct {
		Index int `arg:"" help:"Song index to remove from queue."`
	} `cmd:"" help:"Remove a song from the queue."`
	RemoveAll struct{} `cmd:"" help:"Remove all songs from the queue."`
	Insert    struct {
		Index int    `arg:"" help:"Index to insert song at."`
		Path  string `arg:"" help:"Path of song to insert."`
	} `cmd:"" help:"Add a song to the queue."`
}
