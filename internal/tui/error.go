package tui

import (
	"image"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

func (t *tui) drawError() {
	if t.visibleErr == nil {
		t.clear(t.errorR)
		return
	}

	errString := t.visibleErr.Error()
	errLen := runewidth.StringWidth(errString)

	style := styleDefault.Background(tcell.ColorRed).Foreground(tcell.ColorBlack)

	if errLen+2 > t.errorR.Dx() {
		t.draw(t.errorR.Min, ' ', style)
		t.drawString(t.errorR.Min.Add(image.Pt(1, 0)), t.errorR.Max.X-1, errString, style)
	} else {
		padP := t.errorR.Min.Add(image.Pt(t.errorR.Dx()-errLen-2, 0))
		t.draw(padP, ' ', style)
		t.drawString(padP.Add(image.Pt(1, 0)), t.errorR.Max.X-1, errString, style)
	}
	t.draw(t.errorR.Max.Add(image.Pt(-1, -1)), ' ', style)
}
