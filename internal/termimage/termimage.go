/*
Adapted from github.com/BourgeoisBear/rasterm

# MIT License

# Copyright (c) 2021 Jason Stewart

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/
package termimage

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"strings"
)

// ErrTerminalUnsupported is an error indicating that the current terminal is not
// supported.
var ErrTerminalUnsupported = fmt.Errorf("terminal unsupported")

// WriteImage writes the image m to w, using whatever protocol is detected as
// supported by the current terminal.
//
// r describes where to place the image. The positions given by r.Min and r.Max
// should correspond to the terminal's cells (which are not square, be sure to
// account for this!), and should refer to them with zero-based indexing. If
// no supported protocol is detected for the current terminal, the sentinel
// error TerminalUnsupported is returned.
func WriteImage(w io.Writer, m image.Image, r image.Rectangle) error {
	// TODO: support iterm and others

	if strings.ToLower(os.Getenv("TERM")) != "xterm-kitty" {
		return ErrTerminalUnsupported
	}

	var buf bytes.Buffer
	err := png.Encode(base64.NewEncoder(base64.StdEncoding, &buf), m)
	if err != nil {
		return fmt.Errorf("failed to encode image: %w", err)
	}

	// move cursor to target location
	_, err = fmt.Fprintf(w, "\x1b 7\x1b[%d;%dH", r.Min.Y+1, r.Min.X+1)
	if err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	initialCodes := []byte(fmt.Sprintf("a=T,f=100,S=%d,c=%d,r=%d,",
		buf.Len(), r.Dx(), r.Dy()))

	for buf.Len() > 0 {
		more := []byte("m=1;")
		if buf.Len() <= 4096 {
			more = []byte("m=0;")
		}

		_, err = w.Write(bytes.Join([][]byte{
			[]byte("\x1b_G"),
			initialCodes,
			more,
			buf.Next(4096),
			[]byte("\x1b\\"),
		}, nil))
		if err != nil {
			return fmt.Errorf("write failed: %w", err)
		}

		initialCodes = nil
	}

	// restore cursor position
	_, err = fmt.Fprint(w, "\x1b 8")
	if err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	return nil
}

// WriteImage clears all images by writing to w, using whatever protocol is
// detected as supported by the current terminal.
//
// If no supported protocol is detected for the current terminal, the sentinel
// error TerminalUnsupported is returned.
func ClearImages(w io.Writer) error {
	if strings.ToLower(os.Getenv("TERM")) != "xterm-kitty" {
		return ErrTerminalUnsupported
	}

	_, err := fmt.Fprint(w, "\x1b_Ga=d;\x1b\\")
	if err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	return nil
}
