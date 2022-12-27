package server

import math_rand "math/rand"

// rand returns a random number in the range [min, max).
func rand(min, max int) int {
	return min + math_rand.Intn(max-min)
}

// shuffle randomly reshuffles s.
func shuffle[T any](s []T) {
	math_rand.Shuffle(len(s), func(i, j int) {
		s[i], s[j] = s[j], s[i]
	})
}
