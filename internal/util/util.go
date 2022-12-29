package util

import "golang.org/x/exp/constraints"

// Clamp returns Max(min, Min(v, max)).
func Clamp[T constraints.Ordered](min, v, max T) T {
	return Max(min, Min(v, max))
}

// Max returns the larger value of a and b.
func Max[T constraints.Ordered](a, b T) T {
	if a > b {
		return a
	}

	return b
}

// Min returns the smaller value of a and b.
func Min[T constraints.Ordered](a, b T) T {
	if a < b {
		return a
	}

	return b
}
