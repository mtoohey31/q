package track

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"sync"

	"github.com/bogem/id3v2"
	"github.com/faiface/beep"
	"github.com/faiface/beep/flac"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/vorbis"
	"github.com/faiface/beep/wav"
)

// Track represents a song on the filesystem. This type must not be copied
// after Title, Description, or Cover are accessed.
type Track struct {
	Path string

	infoOnce      sync.Once
	title, artist string
	infoErr       error

	coverOnce sync.Once
	cover     image.Image
	coverErr  error

	lyricsOnce sync.Once
	lyrics     string
	lyricsErr  error

	metaOnce sync.Once
	meta     map[string]string
	metaErr  error
}

func (t *Track) initInfo() {
	t.infoOnce.Do(func() {
		tag, err := id3v2.Open(t.Path, id3v2.Options{
			Parse:       true,
			ParseFrames: []string{"Title", "Artist"},
		})
		if err != nil {
			t.infoErr = err
			return
		}

		t.title, t.artist = tag.Title(), tag.Artist()

		err = tag.Close()
		if err != nil {
			t.infoErr = err
			return
		}
	})
}

// Description returns a short, friendly description of the track.
func (t *Track) Description() (string, error) {
	t.initInfo()
	if t.infoErr != nil {
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
	t.initInfo()
	if t.infoErr != nil {
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
		tag, err := id3v2.Open(t.Path, id3v2.Options{
			Parse:       true,
			ParseFrames: []string{"Attached picture"},
		})
		if err != nil {
			t.coverErr = err
			return
		}
		for _, f := range tag.GetFrames(tag.CommonID("Attached picture")) {
			pf, ok := f.(id3v2.PictureFrame)
			if !ok {
				t.coverErr = fmt.Errorf("picture assert failed")
				return
			}

			// 3 is "Cover (front)", see https://id3.org/id3v2.3.0#Attached_picture
			if pf.PictureType == 3 {
				img, _, err := image.Decode(bytes.NewBuffer(pf.Picture))
				if err != nil {
					t.coverErr = fmt.Errorf("failed to decode cover: %w", err)
					return
				}
				t.cover = img
				break
			}
		}

		err = tag.Close()
		if err != nil {
			t.coverErr = err
			return
		}
	})

	return t.cover, t.coverErr
}

// Lyrics returns the lyrics for this track, if it has any. This function will
// return nil, nil if no error is encountered and the file does not have any
// lyrics.
func (t *Track) Lyrics() (string, error) {
	t.lyricsOnce.Do(func() {
		tag, err := id3v2.Open(t.Path, id3v2.Options{
			Parse:       true,
			ParseFrames: []string{"Unsynchronised lyrics/text transcription"},
		})
		if err != nil {
			t.lyricsErr = err
			return
		}
		for _, f := range tag.GetFrames(tag.CommonID("Unsynchronised lyrics/text transcription")) {
			ulf, ok := f.(id3v2.UnsynchronisedLyricsFrame)
			if !ok {
				t.lyricsErr = fmt.Errorf("lyrics assert failed")
				return
			}

			// TODO: handle when there are multiple frames
			t.lyrics = ulf.Lyrics
			break
		}

		err = tag.Close()
		if err != nil {
			t.lyricsErr = err
			return
		}
	})

	return t.lyrics, t.coverErr
}

// Metadata returns all metadata for this track, if it can be fetched. This
// function will return nil, nil if no error is encountered but no metadata can
// be found.
func (t *Track) Metadata() (map[string]string, error) {
	t.metaOnce.Do(func() {
		tag, err := id3v2.Open(t.Path, id3v2.Options{Parse: true})
		if err != nil {
			t.metaErr = err
			return
		}

		t.meta = map[string]string{}

		var ids map[string]string
		if tag.Version() == 3 {
			ids = id3v2.V23CommonIDs
		} else {
			ids = id3v2.V24CommonIDs
		}

		getIDName := func(id string) string {
			var shortestName string
			for name, oID := range ids {
				if id == oID && (shortestName == "" || len(shortestName) > len(name)) {
					shortestName = name
				}
			}

			if shortestName == "" {
				return id
			}

			return shortestName
		}

		for id := range tag.AllFrames() {
			if textFrame, ok := tag.GetLastFrame(id).(id3v2.TextFrame); ok {
				t.meta[getIDName(id)] = textFrame.Text
			}
		}

		err = tag.Close()
		if err != nil {
			t.metaErr = err
			return
		}
	})

	return t.meta, t.metaErr
}

// Decode returns a beep.StreamSeekCloser and beep.Format for this track.
//
// It is the caller's responsibility to close the beep.StreamSeekCloser when
// they are finished with it.
func (t *Track) Decode() (beep.StreamSeekCloser, beep.Format, error) {
	f, err := os.Open(t.Path)
	if err != nil {
		return nil, beep.Format{}, err
	}

	var magic [12]byte
	_, err = f.Read(magic[:])
	if err != nil {
		return nil, beep.Format{}, err
	}
	_, err = f.Seek(0, 0)
	if err != nil {
		return nil, beep.Format{}, err
	}

	if bytes.Compare(magic[:3], []byte("ID3")) == 0 ||
		bytes.Compare(magic[:2], []byte{0xFF, 0xFB}) == 0 ||
		bytes.Compare(magic[:2], []byte{0xFF, 0xF3}) == 0 ||
		bytes.Compare(magic[:2], []byte{0xFF, 0xF2}) == 0 {
		return mp3.Decode(f)
	}

	if bytes.Compare(magic[:4], []byte("fLaC")) == 0 {
		// TODO: flac panics if you try to seek it after it hits EOF... this is
		// inconsistent with the behaviour of mp3
		return flac.Decode(f)
	}

	if bytes.Compare(magic[:4], []byte("RIFF")) == 0 ||
		bytes.Compare(magic[8:12], []byte("WAVE")) == 0 {
		return wav.Decode(f)
	}

	if bytes.Compare(magic[:4], []byte("OggS")) == 0 {
		return vorbis.Decode(f)
	}

	// TODO: support other formats, and return an error that can be handled on
	// the other end if we don't find a format we support.
	return nil, beep.Format{}, fmt.Errorf("unsupported format with magic: %v", magic)
}
