package server

import "github.com/faiface/beep"

// resampleStreamSeekCloser wraps a beep.Resampler and also stores the
// underlying beep.StreamSeekCloser so it's methods can be used.
type resampleStreamSeekCloser struct {
	// Resampler methods take precedence.
	*beep.Resampler
	// StreamSeekCloser are used for things that the resampler can't do.
	beep.StreamSeekCloser
}

func (r *resampleStreamSeekCloser) Stream(samples [][2]float64) (n int, ok bool) {
	return r.Resampler.Stream(samples)
}

func (r *resampleStreamSeekCloser) Err() error {
	return r.Resampler.Err()
}

func resampleSeekCloser(old beep.SampleRate, new beep.SampleRate, s beep.StreamSeekCloser) beep.StreamSeekCloser {
	return &resampleStreamSeekCloser{
		Resampler:        beep.Resample(4, old, new, s),
		StreamSeekCloser: s,
	}
}
