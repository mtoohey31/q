package types

import (
	"fmt"
	"reflect"

	"github.com/alecthomas/kong"
)

type Tab int

const (
	TabMetadata Tab = iota
	TabLyrics
	TabSearch
)

type TabMapper struct{}

func (tm TabMapper) Decode(ctx *kong.DecodeContext, target reflect.Value) error {
	var repeatString string
	if err := ctx.Scan.PopValueInto("string", &repeatString); err != nil {
		return err
	}

	switch repeatString {
	case "metadata":
		target.Set(reflect.ValueOf(TabMetadata))
	case "lyrics":
		target.Set(reflect.ValueOf(TabLyrics))
	case "search":
		target.Set(reflect.ValueOf(TabSearch))
	default:
		return fmt.Errorf(`must be one of "metadata","lyrics","search" but got "%s"`, repeatString)
	}

	return nil
}
