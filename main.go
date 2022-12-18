package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/faiface/beep/speaker"
	"mtoohey.com/q/internal/types"
)

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
	Shuffle    bool         `short:"s" negatable:"true" default:"true"`
	Repeat     types.Repeat `short:"r" default:"queue"`
	SampleRate uint         `short:"t" default:"44100"`

	// TODO: remove this once I rework stuff
	Paths []string `arg:"" type:"path" optional:"true"`
}

func main() {
	var flags appOptions
	parser := kong.Must(&flags, kong.TypeMapper(
		reflect.TypeOf(types.RepeatNone), types.RepeatMapper{}))

	cfgArgs, err := loadConfig()
	parser.FatalIfErrorf(err)

	_, err = parser.Parse(append(cfgArgs, os.Args[1:]...))
	parser.FatalIfErrorf(err)

	app, err := newApp(flags)
	if err != nil {
		log.Fatalln(err)
	}
	defer app.close()

	err = speaker.Init(app.sampleRate, app.sampleRate.N(time.Millisecond*5))
	if err != nil {
		log.Fatalln(err)
	}
	defer speaker.Close()

	speaker.Play(app)

	if err := app.loop(); err != nil {
		log.Fatalln(err)
	}
}
