package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"mtoohey.com/q/internal/types"

	"github.com/alecthomas/kong"
	"github.com/faiface/beep/speaker"
)

// TODO: fix rendering bugs, can be reproduced by opening a large queue then
// holding down the remove from queue button

func loadConfig() ([]string, error) {
	cfgHome, ok := os.LookupEnv("XDG_CONFIG_HOME")
	if !ok {
		var home string
		home, ok = os.LookupEnv("HOME")
		if !ok {
			return nil, nil
		}
		cfgHome = filepath.Join(home, ".config")
	}

	b, err := os.ReadFile(filepath.Join(cfgHome, "q", "q.conf"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}

		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	return strings.Fields(string(b)), nil
}

type appOptions struct {
	Shuffle        bool         `short:"s" negatable:"true" default:"true"`
	Repeat         types.Repeat `short:"r" default:"queue"`
	InitialTab     types.Tab    `short:"i" default:"metadata"`
	SampleRate     uint         `short:"t" default:"44100"`
	MusicDir       string       `short:"m" default:"." type:"path"`
	ScrollOff      int          `short:"o" default:"7"`
	InitialQueries []string     `arg:"" optional:"true"`
}

func main() {
	var flags appOptions
	parser := kong.Must(&flags,
		kong.TypeMapper(reflect.TypeOf(types.RepeatNone), types.RepeatMapper{}),
		kong.TypeMapper(reflect.TypeOf(types.TabMetadata), types.TabMapper{}),
	)

	// from here on out, we use this same error variable for the current error,
	// and we call return if it is non-nil. by doing this, we allow future
	// defers with resource cleanups (such as closing the speaker and restoring
	// the screen) to be run before the exit happens

	var err error
	defer func() {
		parser.FatalIfErrorf(err)
	}()

	cfgArgs, err := loadConfig()
	if err != nil {
		return
	}

	_, err = parser.Parse(append(cfgArgs, os.Args[1:]...))
	if err != nil {
		return
	}

	app, err := newApp(flags)
	if err != nil {
		return
	}
	defer app.screen.Fini()

	err = speaker.Init(app.sampleRate, app.sampleRate.N(time.Millisecond*10))
	if err != nil {
		return
	}
	defer speaker.Close()

	speaker.Play(app)

	if err = app.loop(); err != nil {
		return
	}
}
