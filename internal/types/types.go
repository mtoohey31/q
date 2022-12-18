package types

import (
	"fmt"
	"reflect"

	"github.com/alecthomas/kong"
)

type Repeat uint8

const (
	RepeatNone Repeat = iota
	RepeatQueue
	RepeatTrack
)

type RepeatMapper struct{}

func (rm RepeatMapper) Decode(ctx *kong.DecodeContext, target reflect.Value) error {
	var repeatString string
	if err := ctx.Scan.PopValueInto("string", &repeatString); err != nil {
		return err
	}

	switch repeatString {
	case "none":
		target.Set(reflect.ValueOf(RepeatNone))
	case "queue":
		target.Set(reflect.ValueOf(RepeatQueue))
	case "track":
		target.Set(reflect.ValueOf(RepeatTrack))
	default:
		return fmt.Errorf(`must be one of "none","queue","track" but got "%s"`, repeatString)
	}

	return nil
}
