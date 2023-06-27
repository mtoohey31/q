package assert

import (
	"reflect"
	"testing"
)

// Equal asserts that the two inputs are identical according to
// reflect.DeepEqual.
func Equal[T any](t *testing.T, expected, actual T) bool {
	t.Helper()

	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected: %#v, actual: %#v", expected, actual)
		return false
	}

	return true
}

// False asserts that the input is false.
func False(t *testing.T, actual bool) bool {
	t.Helper()

	return Equal(t, false, actual)
}

// True asserts that the input is true.
func True(t *testing.T, actual bool) bool {
	t.Helper()

	return Equal(t, true, actual)
}

// Zero asserts that the input is equal to T's zero value according to
// reflect.DeepEqual.
func Zero[T any](t *testing.T, actual T) bool {
	t.Helper()

	var zero T
	return Equal(t, zero, actual)
}
