package queue

import (
	"testing"

	"mtoohey.com/q/internal/protocol"
	"mtoohey.com/q/internal/testutil/assert"
)

func TestQueueFrom(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		assert.Equal(t, Queue[int]{}, QueueFrom[int](nil))
	})

	t.Run("empty", func(t *testing.T) {
		assert.Equal(t, Queue[int]{}, QueueFrom([]int{}))
	})

	t.Run("single", func(t *testing.T) {
		n := &node[int]{value: 7}
		n.next = n
		n.prev = n

		assert.Equal(t, Queue[int]{
			head:        n,
			repeatStart: n,
			len:         1,
		}, QueueFrom([]int{7}))
	})

	t.Run("many", func(t *testing.T) {
		// a -> b -> c -> a ...
		a := &node[uint8]{value: 9}
		b := &node[uint8]{value: 3, prev: a}
		a.next = b
		c := &node[uint8]{value: 73, prev: b, next: a}
		b.next = c
		a.prev = c

		assert.Equal(t, Queue[uint8]{
			head:        a,
			repeatStart: a,
			len:         3,
		}, QueueFrom([]uint8{9, 3, 73}))
	})
}

func TestQueue_Empty(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		assert.True(t, Queue[complex64]{}.Empty())
	})

	t.Run("non-empty", func(t *testing.T) {
		assert.False(t, QueueFrom([]complex128{complex(9, 34)}).Empty())
	})
}

func TestQueue_Head(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		actual, actualOk := Queue[complex64]{}.Head()
		assert.False(t, actualOk)
		assert.Zero(t, actual)
	})

	t.Run("non-empty", func(t *testing.T) {
		actual, actualOk := QueueFrom([]complex128{complex(9, 34)}).Head()
		assert.True(t, actualOk)
		assert.Equal(t, complex(9, 34), actual)
	})
}

func TestQueue_Skip(t *testing.T) {
	t.Run("skip 0", func(t *testing.T) {
		q := QueueFrom([]int{8, 3, 9})
		q.Skip(0)
		assert.Equal(t, QueueFrom([]int{8, 3, 9}), q)
	})

	t.Run("empty", func(t *testing.T) {
		q := Queue[int]{}
		q.Skip(3)
		assert.Equal(t, Queue[int]{}, q)
	})

	t.Run("repeat none, until empty", func(t *testing.T) {
		q := QueueFrom([]int{3, 7, 5})
		q.Skip(3)
		assert.Equal(t, []int{}, q.To())
		q.Skip(-3)
		q.history = nil

		expected := QueueFrom([]int{3, 7, 5})
		// This happens because the repeatStart gets set to the 5 node since it
		// is the first to be re-inserted into the empty queue.
		expected.repeatStart = expected.head.prev
		assert.Equal(t, expected, q)
	})

	t.Run("repeat none, until empty", func(t *testing.T) {
		q := QueueFrom([]int{3, 7, 5})
		q.Skip(2)
		assert.Equal(t, []int{5}, q.To())
		q.Skip(-7)
		q.history = nil
		assert.Equal(t, QueueFrom([]int{3, 7, 5}), q)
	})

	t.Run("repeat track", func(t *testing.T) {
		q := QueueFrom([]int{3, 7, 5})
		q.Repeat = protocol.RepeatStateTrack
		q.Skip(3)
		q.Repeat = protocol.RepeatStateNone
		assert.Equal(t, QueueFrom([]int{3, 7, 5}), q)
	})

	t.Run("repeat queue", func(t *testing.T) {
		q := QueueFrom([]int{3, 7, 5})
		q.Repeat = protocol.RepeatStateQueue
		q.Skip(3)
		assert.Equal(t, []int{3, 7, 5}, q.To())
		q.Skip(-2)
		assert.Equal(t, []int{7, 5, 3}, q.To())
	})

	t.Run("repeat queue, shuffle", func(t *testing.T) {
		q := QueueFrom([]int{3, 7, 5})
		q.Repeat = protocol.RepeatStateQueue
		q.Shuffle = true
		q.Skip(9)
		// We can't assert in the middle here because the state at this point is
		// intentionally non-deterministic, but we can assert that after the
		// skip back, the result should be identical to the start value
		q.Skip(-9)
		assert.Equal(t, []int{3, 7, 5}, q.To())
	})

	t.Run("repeat queue, removal in between", func(t *testing.T) {
		q := QueueFrom([]int{1, 2, 3})
		q.Repeat = protocol.RepeatStateQueue
		q.Skip(2)
		assert.Equal(t, []int{3, 1, 2}, q.To())
		_, ok := q.Remove(2)
		assert.True(t, ok)
		q.Skip(-1)
		assert.Equal(t, []int{1, 3}, q.To())
	})
}

func TestQueue_Insert(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		q := Queue[int]{}
		assert.True(t, q.Insert(6, 0))
		assert.Equal(t, QueueFrom([]int{6}), q)
	})

	t.Run("empty, out of range", func(t *testing.T) {
		q := Queue[int]{}
		assert.False(t, q.Insert(6, 1))
		assert.Equal(t, Queue[int]{}, q)
	})

	t.Run("front", func(t *testing.T) {
		q := QueueFrom([]int{5, 9, 3})
		assert.True(t, q.Insert(6, 0))

		expected := QueueFrom([]int{6, 5, 9, 3})
		// This happens because inserting at 0 doesn't change the repeatStart.
		expected.repeatStart = expected.head.next
		assert.Equal(t, expected, q)
	})

	t.Run("middle", func(t *testing.T) {
		q := QueueFrom([]int{5, 9, 3})
		assert.True(t, q.Insert(6, 2))
		assert.Equal(t, QueueFrom([]int{5, 9, 6, 3}), q)
	})

	t.Run("end", func(t *testing.T) {
		q := QueueFrom([]int{5, 9, 3})
		assert.True(t, q.Insert(6, 3))
		assert.Equal(t, QueueFrom([]int{5, 9, 3, 6}), q)
	})
}

func TestQueue_Remove(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		q := Queue[int]{}
		actualValue, actualOk := q.Remove(0)
		assert.False(t, actualOk)
		assert.Zero(t, actualValue)
		assert.Equal(t, Queue[int]{}, q)
	})

	t.Run("one", func(t *testing.T) {
		q := QueueFrom([]int{5})
		actualValue, actualOk := q.Remove(0)
		assert.True(t, actualOk)
		assert.Equal(t, 5, actualValue)
		assert.Equal(t, Queue[int]{}, q)
	})

	t.Run("front", func(t *testing.T) {
		q := QueueFrom([]int{5, 9, 3})
		actualValue, actualOk := q.Remove(0)
		assert.True(t, actualOk)
		assert.Equal(t, 5, actualValue)
		assert.Equal(t, QueueFrom([]int{9, 3}), q)
	})

	t.Run("middle", func(t *testing.T) {
		q := QueueFrom([]int{5, 9, 3})
		actualValue, actualOk := q.Remove(1)
		assert.True(t, actualOk)
		assert.Equal(t, 9, actualValue)
		assert.Equal(t, QueueFrom([]int{5, 3}), q)
	})

	t.Run("end", func(t *testing.T) {
		q := QueueFrom([]int{5, 9, 3})
		actualValue, actualOk := q.Remove(2)
		assert.True(t, actualOk)
		assert.Equal(t, 3, actualValue)
		assert.Equal(t, QueueFrom([]int{5, 9}), q)
	})
}

func TestQueue_Clear(t *testing.T) {
	t.Run("already empty", func(t *testing.T) {
		q := Queue[int]{}
		q.Clear()
		assert.Equal(t, Queue[int]{}, q)
	})

	t.Run("not empty", func(t *testing.T) {
		q := QueueFrom([]int{5, 9, 3})
		q.Clear()
		assert.Equal(t, Queue[int]{}, q)
	})
}

func TestQueue_Len(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		q := Queue[int]{}
		assert.Zero(t, q)
	})

	t.Run("not empty", func(t *testing.T) {
		q := QueueFrom([]int{5, 9, 3})
		assert.Equal(t, uint(3), q.Len())
	})
}

func TestQueue_To(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		assert.Equal(t, []int{}, Queue[int]{}.To())
	})

	t.Run("not empty", func(t *testing.T) {
		expectedSlice := []int{5, 9, 3}
		assert.Equal(t, expectedSlice, QueueFrom(expectedSlice).To())
	})
}

func TestHard(t *testing.T) {
	t.SkipNow() // TODO: remove this
}
