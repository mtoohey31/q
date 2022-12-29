package track

import (
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"
)

var header = [...]string{"format", "info", "cover", "lyrics", "metadata", "decode"}

type Cmd struct{}

func (c *Cmd) Run() error {
	boolToCheckOrX := func(b bool) string {
		if b {
			return "✓"
		}

		return "✘"
	}

	table := make([][len(header)]string, len(formatHandlers)+1)
	table[0] = header
	for i, handler := range formatHandlers {
		var row [len(header)]string
		row[0] = format(i).String()

		if handler == nil {
			// When the handler is nil, all operations are unsupported
			for j := 1; j < len(header); j++ {
				row[j] = "✘"
			}
			table[i+1] = row
			continue
		}

		row[1] = boolToCheckOrX(handler.info != nil)
		row[2] = boolToCheckOrX(handler.cover != nil)
		row[3] = boolToCheckOrX(handler.lyrics != nil)
		row[4] = boolToCheckOrX(handler.metadata != nil)
		row[5] = boolToCheckOrX(handler.decode != nil)

		table[i+1] = row
	}

	// maxWidths of each column
	var maxWidths [len(table[0])]int
	for col := 0; col < len(table[0]); col++ {
		for row := 0; row < len(table); row++ {
			currLen := runewidth.StringWidth(table[row][col])
			if currLen > maxWidths[col] {
				maxWidths[col] = currLen
			}
		}
	}

	// pad table
	for col := 0; col < len(table[0]); col++ {
		maxWidth := maxWidths[col]
		for row := 0; row < len(table); row++ {
			unpadded := table[row][col]
			paddingWidth := maxWidth - runewidth.StringWidth(unpadded)
			table[row][col] = unpadded + strings.Repeat(" ", paddingWidth)
		}
	}

	for _, row := range table {
		if _, err := fmt.Println(strings.Join(row[:], " ")); err != nil {
			return fmt.Errorf("write failed: %w", err)
		}
	}

	return nil
}
