package main

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
	// "github.com/faiface/beep/vorbis"
	// "github.com/faiface/beep/wav"
)

// TODO: move this into a separate package

// track must not be copied
type track struct {
	Path string

	infoOnce       sync.Once
	title, artist  string
	descriptionErr error

	coverOnce sync.Once
	cover     image.Image
	coverErr  error
}

func (t *track) Cover() (image.Image, error) {
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

func (t *track) Description() (string, error) {
	t.infoOnce.Do(func() {
		tag, err := id3v2.Open(t.Path, id3v2.Options{
			Parse:       true,
			ParseFrames: []string{"Title", "Artist"},
		})
		if err != nil {
			t.descriptionErr = err
			return
		}

		t.title, t.artist = tag.Title(), tag.Artist()

		err = tag.Close()
		if err != nil {
			t.descriptionErr = err
			return
		}
	})

	if t.descriptionErr != nil {
		return "", t.descriptionErr
	}

	if t.title == "" {
		return filepath.Base(t.Path), nil
	}

	return fmt.Sprintf("%s - %s", t.artist, t.title), nil
}

func (t *track) decode() (beep.StreamSeekCloser, beep.Format, error) {
	f, err := os.Open(t.Path)
	if err != nil {
		return nil, beep.Format{}, err
	}

	var magic [4]byte
	_, err = f.Read(magic[:])
	if err != nil {
		return nil, beep.Format{}, err
	}
	_, err = f.Seek(0, 0)
	if err != nil {
		return nil, beep.Format{}, err
	}

	if magic == [4]byte{0x66, 0x4C, 0x61, 0x43} {
		// TODO: flac panics if you try to seek it after it hits EOF... this is
		// inconsistent with the behaviour of mp3
		return flac.Decode(f)
	}

	if bytes.Compare(magic[:3], []byte{0x49, 0x44, 0x33}) == 0 ||
		bytes.Compare(magic[:2], []byte{0xFF, 0xFB}) == 0 ||
		bytes.Compare(magic[:2], []byte{0xFF, 0xF3}) == 0 ||
		bytes.Compare(magic[:2], []byte{0xFF, 0xF2}) == 0 {
		return mp3.Decode(f)
	}

	// TODO: support other formats
	return nil, beep.Format{}, fmt.Errorf("unsupported format with magic: %v", magic)
}
