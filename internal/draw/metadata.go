package draw

import (
	"fmt"
	"image"
	"sort"

	"mtoohey.com/q/internal/track"

	"github.com/gdamore/tcell/v2"
)

type MetadataDrawer struct {
	Queue *[]*track.Track

	scope
}

func (m *MetadataDrawer) Draw(d drawFunc) error {
	if len(*m.Queue) == 0 {
		clear(d, m.Rectangle)
		return nil
	}

	meta, err := (*m.Queue)[0].Metadata()
	if err != nil {
		return fmt.Errorf("failed to get metadata for queue[0]: %w", err)
	}

	if len(meta) == 0 {
		centeredString(d, m.Rectangle, "no metadata found")
		return nil
	}

	type idTextPair struct {
		id, text string
	}

	pairs := make([]idTextPair, 0, len(meta))
	for id, text := range meta {
		pairs = append(pairs, idTextPair{id, text})
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].id < pairs[j].id
	})

	y := m.Min.Y
	for _, pair := range pairs {
		if y >= m.Max.Y {
			break
		}
		x := drawString(d, image.Pt(m.Min.X, y), m.Max.X, fmt.Sprintf("%s: %s",
			pair.id, pair.text), tcell.StyleDefault)
		for ; x < m.Max.X; x++ {
			d(image.Pt(x, y), ' ', tcell.StyleDefault)
		}
		y++
	}

	clear(d, image.Rectangle{
		Min: image.Point{
			X: m.Min.X,
			Y: y,
		},
		Max: m.Max,
	})

	return nil
}

var _ Drawer = &MetadataDrawer{}
