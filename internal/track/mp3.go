package track

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"

	"github.com/bogem/id3v2"
	"github.com/faiface/beep/mp3"
)

var mp3FormatHandler = &formatHandler{
	info: func(r io.Reader) (title string, artist string, err error) {
		tag, err := id3v2.ParseReader(r, id3v2.Options{
			Parse:       true,
			ParseFrames: []string{"Title", "Artist"},
		})
		if err != nil {
			return "", "", fmt.Errorf("tag parse failed: %w", err)
		}

		return tag.Title(), tag.Artist(), nil
	},
	cover: func(r io.Reader) (image.Image, error) {
		tag, err := id3v2.ParseReader(r, id3v2.Options{
			Parse:       true,
			ParseFrames: []string{"Attached picture"},
		})
		if err != nil {
			return nil, fmt.Errorf("tag parse failed: %w", err)
		}

		for _, f := range tag.GetFrames(tag.CommonID("Attached picture")) {
			pf, ok := f.(id3v2.PictureFrame)
			if !ok {
				return nil, fmt.Errorf("picture assert failed")
			}

			// 3 is "Cover (front)", see https://id3.org/id3v2.3.0#Attached_picture
			if pf.PictureType == 3 {
				img, _, err := image.Decode(bytes.NewBuffer(pf.Picture))
				if err != nil {
					return nil, fmt.Errorf("cover decode failed: %w", err)
				}

				return img, nil
			}
		}

		return nil, nil
	},
	lyrics: func(r io.Reader) (string, error) {
		tag, err := id3v2.ParseReader(r, id3v2.Options{
			Parse:       true,
			ParseFrames: []string{"Unsynchronised lyrics/text transcription"},
		})
		if err != nil {
			return "", fmt.Errorf("tag parse failed: %w", err)
		}

		for _, f := range tag.GetFrames(tag.CommonID("Unsynchronised lyrics/text transcription")) {
			ulf, ok := f.(id3v2.UnsynchronisedLyricsFrame)
			if !ok {
				return "", fmt.Errorf("lyrics assert failed")
			}

			// TODO: can there be multiple lyrics frames? what would that mean?
			// How should that be handled? Should we try to do something with
			// the Language field to figure out what might be preferred, or
			// should we let the user switch between languages if there are
			// multiple?
			return ulf.Lyrics, nil
		}

		return "", nil
	},
	metadata: func(r io.Reader) (map[string]string, error) {
		tag, err := id3v2.ParseReader(r, id3v2.Options{Parse: true})
		if err != nil {
			return nil, fmt.Errorf("tag parse failed: %w", err)
		}

		m := map[string]string{}

		var ids map[string]string
		if tag.Version() == 3 {
			ids = id3v2.V23CommonIDs
		} else {
			ids = id3v2.V24CommonIDs
		}

		// Returns the shortest friendly name for an ID
		getIDName := func(id string) string {
			var shortestName string
			for name, oID := range ids {
				if id == oID && (shortestName == "" || len(shortestName) > len(name)) {
					shortestName = name
				}
			}

			// If we didn't find a friendly name, return the original ID
			// instead of returning nothing
			if shortestName == "" {
				return id
			}

			return shortestName
		}

		for id := range tag.AllFrames() {
			if textFrame, ok := tag.GetLastFrame(id).(id3v2.TextFrame); ok {
				m[getIDName(id)] = textFrame.Text
			}
		}

		return m, nil
	},
	decode: mp3.Decode,
}
