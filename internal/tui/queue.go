package tui

import (
	"image"

	"mtoohey.com/q/internal/util"

	"github.com/gdamore/tcell/v2"
)

func (t *tui) drawQueue() {
	if len(t.Queue) == 0 {
		t.centeredString(t.queueR, "queue empty")
		return
	}

	// make sure scrolloff is no greater than half the current height
	t.ScrollOff = util.Min(t.ScrollOff, t.queueR.Dy()/2)

	distFromTop := t.queueFocusIdx - t.queueScrollIdx
	if distFromTop <= t.ScrollOff {
		t.queueScrollIdx = util.Max(0, t.queueScrollIdx-(t.ScrollOff-distFromTop))
	}

	distFromBottom := t.queueR.Dy() - distFromTop
	if distFromBottom <= t.ScrollOff {
		t.queueScrollIdx += 1 + t.ScrollOff - distFromBottom
	}

	i := 0
	for ; i < t.queueR.Dy() && i+t.queueScrollIdx < len(t.Queue); i++ {
		style := styleDefault
		if i+t.queueScrollIdx == t.queueFocusIdx {
			style = style.Background(tcell.ColorAqua).Foreground(tcell.ColorBlack)
		}

		t.draw(t.queueR.Min.Add(image.Pt(0, i)), ' ', style)

		x := t.drawString(t.queueR.Min.Add(image.Pt(1, i)), t.queueR.Max.X-1, string(t.Queue[i+t.queueScrollIdx]), style)
		for ; x < t.queueR.Max.X; x++ {
			t.draw(image.Pt(x, i), ' ', style)
		}
	}
	t.clear(image.Rect(t.queueR.Min.X, t.queueR.Min.Y+i, t.queueR.Max.X, t.queueR.Max.Y))
}

func (t *tui) queueFocus(to int) {
	t.queueFocusIdx = util.Clamp(0, to, len(t.Queue)-1)
	t.drawQueue()
}

func (t *tui) queueShiftFocus(by int) {
	t.queueFocus(t.queueFocusIdx + by)
}
