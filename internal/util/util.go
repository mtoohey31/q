package util

import "golang.org/x/exp/constraints"

// Clamp returns Max(min, Min(v, max)).
func Clamp[T constraints.Ordered](min, v, max T) T {
	return Max(min, Min(v, max))
}

// Min returns the largest provided argument. If no arguments are provided, it
// returns the zero value for T.
func Max[T constraints.Ordered](v ...T) T {
	if len(v) == 0 {
		var z T
		return z
	}

	max := v[0]
	for _, o := range v[1:] {
		if o > max {
			max = o
		}
	}

	return max
}

// Min returns the smallest provided argument. If no arguments are provided, it
// returns the zero value for T.
func Min[T constraints.Ordered](v ...T) T {
	if len(v) == 0 {
		var z T
		return z
	}

	min := v[0]
	for _, o := range v[1:] {
		if o < min {
			min = o
		}
	}

	return min
}
