package draw

import (
	"fmt"
	"sort"

	"mtoohey.com/q/internal/track"

	"github.com/gdamore/tcell/v2"
)

type MetadataDrawer struct {
	Queue *[]*track.Track

	scope
}

func (m *MetadataDrawer) Draw() error {
	if len(*m.Queue) == 0 {
		clear(m.d, m.w, m.h)
		return nil
	}

	meta, err := (*m.Queue)[0].Metadata()
	if err != nil {
		return fmt.Errorf("failed to get metadata for queue[0]: %w", err)
	}

	if len(meta) == 0 {
		centeredString(m.d, m.w, m.h, "no metadata found")
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

	y := 0
	for _, pair := range pairs {
		if y >= m.h {
			break
		}
		x := drawString(offset(m.d, 0, y), m.w, fmt.Sprintf("%s: %s",
			pair.id, pair.text), tcell.StyleDefault)
		for ; x < m.w; x++ {
			m.d(x, y, ' ', tcell.StyleDefault)
		}
		y++
	}

	if y < m.h {
		clear(offset(m.d, 0, y), m.w, m.h-y)
	}

	return nil
}

var _ Drawer = &MetadataDrawer{}
