package remote

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"mtoohey.com/q/internal/cmd"
	"mtoohey.com/q/internal/protocol"
	"mtoohey.com/q/internal/server/unixsocketconn"

	"github.com/alecthomas/kong"
)

//go:embed state_default.gotmpl
var defaultTemplateText string

type Cmd struct {
	// StateDefault works around the kong restriction that you can either have a
	// subcommand with no arguments that is the default when there are no
	// arguments, or a subcommand with arguments that is the default even when
	// there are arguments (as long as they match), but you can't have a
	// subcommand with arguments that is the default only when there are no
	// arguments, which is what we want, because if we pass state a
	// non-subcommand first argument, it uses that as the template which usually
	// means it's just echoing back the same string which is super confusing. So
	// instead we define a separate, hidden state subcommnd which is the default
	// only when there are no arguments.
	StateDefault struct{} `cmd:"" hidden:"" default:"1"`
	State        struct {
		Template *string `arg:"" optional:"" help:"Template using Go text/template syntax."`
	} `cmd:"" help:"Display current state."`

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
}

func (c *Cmd) Run(ctx *kong.Context, g cmd.Globals) (err error) {
	if g.UnixSocket == nil {
		return fmt.Errorf("missing flags: --unix-socket=UNIX-SOCKET")
	}

	conn, err := unixsocketconn.NewUnixSocketClientConn(*g.UnixSocket)
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
	switch ctx.Command() {
	case "remote state", "remote state-default", "remote state <template>":
		templateText := defaultTemplateText
		if c.State.Template != nil {
			templateText = *c.State.Template
		}

		t, err := template.New("state").Parse(templateText)
		if err != nil {
			return err
		}
		return t.Execute(os.Stdout, state)

	case "remote pause", "remote pause <pause-state>":
		if c.Pause.PauseState != nil {
			m = c.Pause.PauseState
		} else if c.Pause.Cycle {
			m = !state.Pause
		} else {
			m = protocol.PauseState(true)
		}

	case "remote repeat", "remote repeat <repeat-state>":
		if c.Repeat.RepeatState != nil {
			m = c.Repeat.RepeatState
		} else if c.Repeat.Cycle {
			m = state.Repeat.Next()
		} else {
			m = protocol.RepeatStateQueue
		}

	case "remote shuffle", "remote shuffle <shuffle-state>":
		if c.Shuffle.ShuffleState != nil {
			m = c.Shuffle.ShuffleState
		} else if c.Shuffle.Cycle {
			m = !state.Shuffle
		} else {
			m = protocol.ShuffleState(true)
		}

	case "remote skip", "remote skip <songs>":
		m = c.Skip.Songs

	case "remote seek <by>":
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

	case "remote remove <index>":
		m = protocol.Remove(c.Remove.Index)

	case "remote remove-all":
		m = protocol.RemoveAll{}

	case "remote insert <index> <path>":
		absPath, err := filepath.Abs(c.Insert.Path)
		if err != nil {
			return fmt.Errorf("failed to create absolute path: %w", err)
		}

		m = protocol.Insert{
			Index: c.Insert.Index,
			Path:  absPath,
		}

	default:
		panic(fmt.Sprintf(`unhandled command "%s"`, ctx.Command()))
	}

	if err := conn.Send(m); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}
