package remote

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"mtoohey.com/q/internal/cmd"
	"mtoohey.com/q/internal/protocol"
	"mtoohey.com/q/internal/server/unixsocketconn"

	"github.com/alecthomas/kong"
)

type Cmd struct {
	State struct {
		Template string `arg:"" optional:"" help:"Template using Go text/template syntax."`
	} `cmd:"" default:"withargs" help:"Display current state."`

	Pause struct {
		PauseState *protocol.PauseState `arg:"" optional:"true" type:"boolarg" help:"New pause state."`
		Cycle      bool                 `short:"c" help:"Cycle current pause state."`
	} `cmd:"" help:"Pause playback."`
	Repeat struct {
		RepeatState *protocol.RepeatState `arg:"" optional:"true" help:"New repeat state."`
		Cycle       bool                  `short:"c" help:"Cycle current repeat state."`
	} `cmd:"" help:"Set repeat state."`
	Shuffle struct {
		ShuffleState *protocol.ShuffleState `arg:"" optional:"true" type:"boolarg" help:"New shuffle state."`
		Cycle        bool                   `short:"c" help:"Cycle current shuffle state."`
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
	Later struct {
		Index int `arg:"" help:"Song index to move to later in the queue."`
	} `cmd:"" help:"Move a song to later in the queue."`
}

func (c *Cmd) Run(ctx *kong.Context, g cmd.Globals) (err error) {
	conn, err := unixsocketconn.NewUnixSocketClientConn(g.UnixSocket)
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

	if state.Version != protocol.Version {
		return fmt.Errorf(`server version "%s" does not match remote version "%s"`,
			state.Version, protocol.Version)
	}

	var m protocol.Message
	command := strings.Split(ctx.Command(), " ")[1]
	switch command {
	case "state":
		templateText := c.State.Template
		if templateText == "" {
			templateText = "{{ if .Pause }}契 {{else}} {{end}}{{ if .NowPlaying }}{{ .NowPlaying.Title }}{{ if .NowPlaying.Artist }} - {{ .NowPlaying.Artist }}{{end}}{{else}}nothing playing{{end}}\n"
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

	case "pause":
		if c.Pause.PauseState != nil {
			m = c.Pause.PauseState
		} else if c.Pause.Cycle {
			m = !state.Pause
		} else {
			m = protocol.PauseState(true)
		}

	case "repeat":
		if c.Repeat.RepeatState != nil {
			m = c.Repeat.RepeatState
		} else if c.Repeat.Cycle {
			m = state.Repeat.Next()
		} else {
			m = protocol.RepeatStateQueue
		}

	case "shuffle":
		if c.Shuffle.ShuffleState != nil {
			m = c.Shuffle.ShuffleState
		} else if c.Shuffle.Cycle {
			m = !state.Shuffle
		} else {
			m = protocol.ShuffleState(true)
		}

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
		absPath, err := filepath.Abs(c.Insert.Path)
		if err != nil {
			return fmt.Errorf("failed to create absolute path: %w", err)
		}

		m = protocol.Insert{
			Index: c.Insert.Index,
			Path:  absPath,
		}

	case "later":
		m = protocol.Later(c.Later.Index)

	default:
		panic(fmt.Sprintf(`unhandled command "%s"`, command))
	}

	if err := conn.Send(m); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}
