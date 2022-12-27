package tui

import (
	"image"
	"math"
	"strings"
	"time"

	"mtoohey.com/q/internal/protocol"
	"mtoohey.com/q/internal/util"
)

func (t *tui) drawBar() {
	var zeroProgress protocol.ProgressState
	if t.Progress == zeroProgress {
		t.clear(t.barR)
		return
	}

	currS := t.Progress.Current.Truncate(time.Second).String()
	totalS := t.Progress.Total.Truncate(time.Second).String()

	dW := 2 // first seconds digit and 's'

	// the second seconds digit
	if t.Progress.Total >= time.Second*10 {
		dW++
	}

	// the first minutes digit and 'm'
	if t.Progress.Total >= time.Minute {
		dW += 2
	}

	// the second minutes digit
	if t.Progress.Total >= time.Minute*10 {
		dW++
	}

	// the remaining hours digits and 'h'
	if t.Progress.Total >= time.Hour {
		dW += int(math.Floor(math.Log10(float64(t.Progress.Total/time.Hour)))) + 2
	}

	barW := t.barR.Dx() - (2*dW + 2)

	currS = strings.Repeat(" ", dW-len(currS)) + currS
	totalS = strings.Repeat(" ", dW-len(totalS)) + totalS

	t.drawString(t.barR.Min, t.barR.Max.X, currS, styleDefault)
	t.draw(t.barR.Min.Add(image.Pt(dW, 0)), '|', styleDefault)

	progressRatio := float64(t.Progress.Current) / float64(t.Progress.Total)
	barCompleteW := util.Max(0, int(progressRatio*float64(barW)))
	t.drawString(t.barR.Min.Add(image.Pt(dW+1, 0)), t.barR.Max.X, strings.Repeat("█", barCompleteW), styleDefault)

	if barCompleteW > barW {
		barCompleteW = barW
	}

	if barCompleteW != barW {
		// the fractional part of a box not drawn, should be between 0 and 1
		remainder := progressRatio*float64(barW) - float64(barCompleteW)
		partialBoxes := [8]rune{' ', '▏', '▎', '▍', '▌', '▋', '▊', '▉'}
		t.draw(t.barR.Min.Add(image.Pt(dW+1+barCompleteW, 0)), partialBoxes[int(remainder*8)], styleDefault)

		t.drawString(t.barR.Min.Add(image.Pt(dW+1+barCompleteW+1, 0)), t.barR.Max.X,
			strings.Repeat(" ", barW-barCompleteW-1), styleDefault)
	}

	t.draw(image.Pt(t.barR.Max.X-dW-1, t.barR.Min.Y), '|', styleDefault)
	t.drawString(image.Pt(t.barR.Max.X-dW, t.barR.Min.Y), t.barR.Max.X, totalS, styleDefault)
}
