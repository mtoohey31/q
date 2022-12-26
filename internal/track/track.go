package track

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/faiface/beep"
	"github.com/faiface/beep/flac"
	"github.com/faiface/beep/vorbis"
	"github.com/faiface/beep/wav"
)

type format uint8

const (
	formatMp3 format = iota
	formatFlac
	formatWav
	formatVorbis
	formatMp4
)

// String implements fmt.Stringer.
func (f format) String() string {
	switch f {
	case formatMp3:
		return "mp3"
	case formatFlac:
		return "flac"
	case formatWav:
		return "wav"
	case formatVorbis:
		return "vorbis"
	case formatMp4:
		return "mp4"
	default:
		return "<invalid format>"
	}
}

var _ fmt.Stringer = format(0)

// formatHandler defines functions for handling a given audio format. Functions
// may be nil, which indicates that the operation is not supported.
type formatHandler struct {
	info  func(io.Reader) (title, artist string, err error)
	cover func(io.Reader) (image.Image, error)
	// TODO: support lyrics with timestamps
	lyrics   func(io.Reader) (string, error)
	metadata func(io.Reader) (map[string]string, error)
	decode   func(io.ReadCloser) (beep.StreamSeekCloser, beep.Format, error)
}

// wrapReaderDecoder converts a decode function that takes an io.Reader to one
// that takes an io.ReadCloser.
func wrapReaderDecoder(d func(io.Reader) (beep.StreamSeekCloser, beep.Format, error)) func(io.ReadCloser) (beep.StreamSeekCloser, beep.Format, error) {
	return func(rc io.ReadCloser) (beep.StreamSeekCloser, beep.Format, error) {
		return d(rc)
	}
}

// formatHandlers contains formatHandler objects for the audio formats
// supported by this player.
var formatHandlers = [...]*formatHandler{
	formatMp3: mp3FormatHandler,
	formatFlac: {
		decode: wrapReaderDecoder(flac.Decode),
	},
	formatWav: {
		decode: wrapReaderDecoder(wav.Decode),
	},
	formatVorbis: {
		decode: vorbis.Decode,
	},
	formatMp4: nil,
}

// Track represents a song on the filesystem. This type must not be copied
// after Title, Description, or Cover are accessed.
type Track struct {
	Path string

	formatOnce sync.Once
	format     format
	formatErr  error

	infoOnce      sync.Once
	title, artist string
	infoErr       error

	coverOnce sync.Once
	cover     image.Image
	coverErr  error

	lyricsOnce sync.Once
	lyrics     string
	lyricsErr  error

	metadataOnce sync.Once
	metadata     map[string]string
	metadataErr  error
}

func (t *Track) initFormat() {
	t.formatOnce.Do(func() {
		f, err := os.Open(t.Path)
		if err != nil {
			t.formatErr = err
			return
		}

		var magic [12]byte
		_, err = f.Read(magic[:])
		if err != nil {
			t.formatErr = err
			return
		}

		switch {
		// TODO: does the current metadata parsing only work when we get a file
		// that has the "ID3" marker?
		case bytes.Compare(magic[:3], []byte("ID3")) == 0 ||
			bytes.Compare(magic[:2], []byte{0xFF, 0xFB}) == 0 ||
			bytes.Compare(magic[:2], []byte{0xFF, 0xF3}) == 0 ||
			bytes.Compare(magic[:2], []byte{0xFF, 0xF2}) == 0:
			t.format = formatMp3
		case bytes.Compare(magic[:4], []byte("fLaC")) == 0:
			t.format = formatFlac
		case bytes.Compare(magic[:4], []byte("RIFF")) == 0 ||
			bytes.Compare(magic[8:12], []byte("WAVE")) == 0:
			t.format = formatWav
		case bytes.Compare(magic[:4], []byte("OggS")) == 0:
			t.format = formatVorbis
		case bytes.Compare(magic[:8], []byte("ftypisom")) == 0:
			t.format = formatMp4
		default:
			t.formatErr = fmt.Errorf("unknown format with magic: %v", magic)
		}
	})
}

func (t *Track) initInfo() {
	t.infoOnce.Do(func() {
		if t.initFormat(); t.formatErr != nil {
			t.infoErr = fmt.Errorf("format error: %w", t.formatErr)
			return
		}

		handlers := formatHandlers[t.format]
		if handlers == nil || handlers.info == nil {
			// don't throw an error, but leave things empty
			return
		}

		f, err := os.Open(t.Path)
		if err != nil {
			t.infoErr = fmt.Errorf("open failed: %w", err)
			return
		}
		defer f.Close() // intentionally ignore close error

		t.title, t.artist, t.infoErr = handlers.info(f)
	})
}

// Description returns a short, friendly description of the track.
func (t *Track) Description() (string, error) {
	if t.initInfo(); t.infoErr != nil {
		return "", t.infoErr
	}

	if t.title == "" {
		return filepath.Base(t.Path), nil
	}

	if t.artist == "" {
		return t.title, nil
	}

	return fmt.Sprintf("%s - %s", t.artist, t.title), nil
}

// Title returns the title of the track. The basename of the track's path may
// be used if no title was found.
func (t *Track) Title() (string, error) {
	if t.initInfo(); t.infoErr != nil {
		return "", t.infoErr
	}

	title := t.title
	if title == "" {
		return filepath.Base(t.Path), nil
	}

	return title, nil
}

// Artist returns the artist for the track. It may return "" if no artist was
// found.
func (t *Track) Artist() (string, error) {
	t.initInfo()
	return t.artist, t.infoErr
}

// Cover returns the cover image of this track, if it has one. This function
// will return nil, nil if no error is encountered and the file does not have
// a cover image.
func (t *Track) Cover() (image.Image, error) {
	t.coverOnce.Do(func() {
		if t.initFormat(); t.formatErr != nil {
			t.infoErr = fmt.Errorf("format error: %w", t.formatErr)
			return
		}

		handlers := formatHandlers[t.format]
		if handlers == nil || handlers.cover == nil {
			// leave the cover nil, since that's allowed
			return
		}

		f, err := os.Open(t.Path)
		if err != nil {
			t.coverErr = fmt.Errorf("open failed: %w", err)
			return
		}
		defer f.Close() // intentionally ignore close error

		t.cover, t.coverErr = handlers.cover(f)
	})

	return t.cover, t.coverErr
}

// Lyrics returns the lyrics for this track, if it has any. This function will
// return nil, nil if no error is encountered and the file does not have any
// lyrics.
func (t *Track) Lyrics() (string, error) {
	t.lyricsOnce.Do(func() {
		if t.initFormat(); t.formatErr != nil {
			t.infoErr = fmt.Errorf("format error: %w", t.formatErr)
			return
		}

		handlers := formatHandlers[t.format]
		if handlers == nil || handlers.lyrics == nil {
			// leave the lyrics empty
			return
		}

		f, err := os.Open(t.Path)
		if err != nil {
			t.lyricsErr = fmt.Errorf("open failed: %w", err)
			return
		}
		defer f.Close() // intentionally ignore close error

		t.lyrics, t.lyricsErr = handlers.lyrics(f)
	})

	return t.lyrics, t.lyricsErr
}

// Metadata returns all metadata for this track, if it can be fetched. This
// function will return nil, nil if no error is encountered but no metadata can
// be found.
func (t *Track) Metadata() (map[string]string, error) {
	t.metadataOnce.Do(func() {
		if t.initFormat(); t.formatErr != nil {
			t.infoErr = fmt.Errorf("format error: %w", t.formatErr)
			return
		}

		handlers := formatHandlers[t.format]
		if handlers == nil || handlers.metadata == nil {
			// leave the metadata empty
			return
		}

		f, err := os.Open(t.Path)
		if err != nil {
			t.metadataErr = fmt.Errorf("open failed: %w", err)
			return
		}
		defer f.Close() // intentionally ignore close error

		t.metadata, t.metadataErr = handlers.metadata(f)
	})

	return t.metadata, t.metadataErr
}

// Decode returns a beep.StreamSeekCloser and beep.Format for this track.
//
// It is the caller's responsibility to close the beep.StreamSeekCloser when
// they are finished with it.
func (t *Track) Decode() (beep.StreamSeekCloser, beep.Format, error) {
	if t.initFormat(); t.formatErr != nil {
		return nil, beep.Format{}, fmt.Errorf("format error: %w", t.formatErr)
	}

	handlers := formatHandlers[t.format]
	if handlers == nil || handlers.decode == nil {
		return nil, beep.Format{}, fmt.Errorf("decode not supported for format %s", t.format.String())
	}

	f, err := os.Open(t.Path)
	if err != nil {
		return nil, beep.Format{}, fmt.Errorf("open failed: %w", err)
	}
	// don't close, will be closed when the StreamSeekCloser gets closed

	return handlers.decode(f)
}
