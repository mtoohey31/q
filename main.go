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

type repeat uint8

const (
	repeatNone repeat = iota
	repeatQueue
	repeatTrack
)

type repeatMapper struct{}

func (rm repeatMapper) Decode(ctx *kong.DecodeContext, target reflect.Value) error {
	var repeatString string
	if err := ctx.Scan.PopValueInto("string", &repeatString); err != nil {
		return err
	}

	switch repeatString {
	case "none":
		target.Set(reflect.ValueOf(repeatNone))
	case "queue":
		target.Set(reflect.ValueOf(repeatQueue))
	case "track":
		target.Set(reflect.ValueOf(repeatTrack))
	default:
		return fmt.Errorf(`must be one of "none","queue","track" but got "%s"`, repeatString)
	}

	return nil
}

type appOptions struct {
	Shuffle    bool   `short:"s" negatable:"true" default:"true"`
	Repeat     repeat `short:"r" default:"queue"`
	SampleRate uint   `short:"t" default:"44100"`

	// TODO: remove this once I rework stuff
	Paths []string `arg:"" type:"path" optional:"true"`
}

func main() {
	var flags appOptions
	parser := kong.Must(&flags, kong.TypeMapper(reflect.TypeOf(repeatNone), repeatMapper{}))

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
