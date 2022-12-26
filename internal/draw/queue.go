package draw

import (
	"fmt"
	"image"

	"mtoohey.com/q/internal/track"

	"github.com/gdamore/tcell/v2"
)

type QueueDrawer struct {
	Queue         *[]*track.Track
	QueueFocusIdx *int
	ScrollOff     int

	scrollIdx int

	scope
}

func (q *QueueDrawer) Draw(d drawFunc) error {
	if len(*q.Queue) == 0 {
		centeredString(d, q.Rectangle, "queue empty")
		return nil
	}

	// make sure scrolloff is no greater than half the current height
	scrollOff := q.ScrollOff
	if scrollOff > q.Dy()/2 {
		scrollOff = q.Dy() / 2
	}

	distFromTop := *q.QueueFocusIdx - q.scrollIdx
	if distFromTop <= scrollOff {
		q.scrollIdx -= scrollOff - distFromTop
		if q.scrollIdx < 0 {
			q.scrollIdx = 0
		}
	}

	distFromBottom := q.Dy() - distFromTop
	if distFromBottom <= scrollOff {
		q.scrollIdx += 1 + scrollOff - distFromBottom
	}

	y := 0
	for ; y < q.Dy() && y+q.scrollIdx < len(*q.Queue); y++ {
		description, err := (*q.Queue)[y+q.scrollIdx].Description()
		if err != nil {
			return fmt.Errorf("failed to get description of queue[%d]: %w", y, err)
		}

		style := tcell.StyleDefault
		if y+q.scrollIdx == *q.QueueFocusIdx {
			style = style.Background(tcell.ColorAqua).Foreground(tcell.ColorBlack)
		}
		d(q.Min.Add(image.Pt(0, y)), ' ', style)
		x := drawString(d, q.Min.Add(image.Pt(1, y)), q.Max.X-1, description, style)
		for ; x < q.Max.X; x++ {
			d(image.Pt(x, y), ' ', style)
		}
	}
	clear(d, image.Rectangle{
		Min: image.Point{
			X: q.Min.X,
			Y: q.Min.Y + y,
		},
		Max: q.Max,
	})

	return nil
}

func (q *QueueDrawer) Height() int {
	return q.Dy()
}

var _ Drawer = &QueueDrawer{}
