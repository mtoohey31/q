package cmd

import (
	"fmt"
	"reflect"

	"mtoohey.com/q/internal/protocol"

	"github.com/alecthomas/kong"
)

// TypeMappers contains all the kong.TypeMapper options that should be used
// when parsing at the top-level.
var TypeMappers = []kong.Option{
	kong.NamedMapper("boolarg", kong.MapperFunc(func(ctx *kong.DecodeContext, target reflect.Value) error {
		var s string
		if err := ctx.Scan.PopValueInto("string", &s); err != nil {
			return err
		}

		var v bool
		switch s {
		case "false":
			v = false
		case "true":
			v = true
		default:
			return fmt.Errorf(`must be one of "false","true" but got "%s"`, s)
		}

		if target.Type().Kind() == reflect.Pointer {
			target.Set(reflect.ValueOf(&v).Convert(target.Type()))
		} else {
			target.Set(reflect.ValueOf(v).Convert(target.Type()))
		}

		return nil
	})),

	kong.TypeMapper(reflect.TypeOf(protocol.RepeatState(0)), kong.MapperFunc(func(ctx *kong.DecodeContext, target reflect.Value) error {
		var repeatString string
		if err := ctx.Scan.PopValueInto("string", &repeatString); err != nil {
			return err
		}

		switch repeatString {
		case "none":
			target.Set(reflect.ValueOf(protocol.RepeatStateNone))
		case "queue":
			target.Set(reflect.ValueOf(protocol.RepeatStateQueue))
		case "track":
			target.Set(reflect.ValueOf(protocol.RepeatStateTrack))
		default:
			return fmt.Errorf(`must be one of "none","queue","track" but got "%s"`, repeatString)
		}

		return nil
	})),
}
