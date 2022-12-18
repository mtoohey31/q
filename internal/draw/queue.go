package draw

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"mtoohey.com/q/internal/track"
)

type QueueDrawer struct {
	Queue         *[]*track.Track
	QueueFocusIdx *int

	scope
}

func (q *QueueDrawer) Draw() error {
	if len(*q.Queue) == 0 {
		centeredString(q.d, q.w, q.h, "queue empty")
		return nil
	}

	y := 0
	for ; y < q.h && y < len(*q.Queue); y++ {
		description, err := (*q.Queue)[y].Description()
		if err != nil {
			return fmt.Errorf("failed to get description of queue[%d]: %w", y, err)
		}

		style := tcell.StyleDefault
		if y == *q.QueueFocusIdx {
			style = style.Background(tcell.ColorAqua).Foreground(tcell.ColorBlack)
		}
		q.d(0, y, ' ', style)
		x := drawString(offset(q.d, 1, y), q.w-2, description, style)
		for x++; x < q.w; x++ {
			q.d(x, y, ' ', style)
		}
	}
	clear(offset(q.d, 0, y), q.w, q.h-y)

	return nil
}

var _ Drawer = &QueueDrawer{}
