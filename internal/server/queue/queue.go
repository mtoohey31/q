package queue

import (
	"math/rand"

	"mtoohey.com/q/internal/protocol"
)

// Queue is a queue in the audio player sense, which contains values of type T,
// which will usually be some kind of data structure storing information needed
// for playback of an audio track. The zero value of this type is an empty
// queue, ready for use. This type is not threadsafe. It should be protected
// with a mutex if threadsafety is desired.
type Queue[T any] struct {
	// Repeat indicates the type of repeat behaviour that the queue should
	// exhibit. Can be changed at any point.
	Repeat protocol.RepeatState

	// Shuffle indicates if values should be shuffled when re-inserted into the
	// queue if repeat == protocol.RepeatStateQueue. Can be changed at any
	// point.
	Shuffle protocol.ShuffleState

	// head points to the top element of the queue if len != 0. Otherwise, if
	// len == 0, then head == nil.
	head *node[T]
	// repeatStart is the first song played on the current repeat of the queue
	// if len != 0. Otherwise, if len == 0, then repeatStart == nil.
	repeatStart *node[T]
	// len is the number of elements in the queue.
	len uint

	// history is a list of entries that can be used to restore the queue to a
	// previous state. They are arranged with the last entry being the most
	// recently added.
	history []historyEntry[T]
}

// node is an entry in the queue.
type node[T any] struct {
	// value is the track value stored in this node.
	value T
	// next is the node containing the track that comes after the one stored in
	// this node.
	next *node[T]
	// prev is the node containing the track that comes before the one stored in
	// this node.
	prev *node[T]
	// removed indicates that this node was previously re-inserted manually into
	// the queue but was explicitly removed, and so should not be re-inserted
	// when skipping backwards.
	removed bool
}

// historyEntry is a function that adds a given track back into the queue in an
// implementation-specific manner. It returns true if nothing could be restored,
// in which case the prior history entry should be restored instead. Each value
// should only be called once after creation.
type historyEntry[T any] func(*Queue[T]) bool

// QueueFrom creates a new Queue from the given slice.
func QueueFrom[T any](s []T) Queue[T] {
	q := Queue[T]{len: uint(len(s))}

	if q.Empty() {
		return q
	}
	// In this case we have that len(s) >= 1.

	q.head = &node[T]{value: s[0]}
	q.repeatStart = q.head

	curr := q.head
	for _, v := range s[1:] {
		next := &node[T]{value: v, prev: curr}
		curr.next = next
		curr = next
	}

	curr.next = q.head
	q.head.prev = curr

	return q
}

// Empty returns whether the queue is empty.
func (q Queue[T]) Empty() bool { return q.len == 0 }

// Head returns the track at top of the queue (the "now playing" value) and
// true if the queue is non-empty. If the queue is empty, it returns the zero
// value of T and false.
func (q Queue[T]) Head() (v T, ok bool) {
	if q.Empty() {
		var z T
		return z, false
	}

	return q.head.value, true
}

// Skip moves the queue n tracks forward (or backward).
//
// Note that depending on the prior states of the queue, skipping backwards may
// not result in a simple rotation of the queue. For example, if repeat was
// disabled when the previous "now playing" track was skipped, it will be
// restored as the head of the queue instead of the tail of the queue becoming
// the head.
func (q *Queue[T]) Skip(n int) {
	switch {
	case n > 0:
		q.skipFW(uint(n))

	case n < 0:
		q.skipRV(uint(-n))

	case n == 0:
		// Do nothing: we're moving neither forward or backward, this shouldn't
		// have any effect.
	}
}

// skipFW skips forward. Expects n > 0.
func (q *Queue[T]) skipFW(n uint) {
	if q.Empty() {
		return
	}

	switch q.Repeat {
	case protocol.RepeatStateNone:
		newHistory := make([]historyEntry[T], 0, n)

		curr := q.head
		for i := uint(0); i < n && i < q.len; i, curr = i+1, curr.next {
			restoreValue := curr.value
			newHistory = append(newHistory, func(q *Queue[T]) bool {
				if !q.Insert(restoreValue, 0) {
					// Should be unreachable because Insert can only fail when
					// i > q.len, but q.len >= 0 == i.
					panic("Insert failed")
				}

				return true
			})
		}
		// When the loop exits, curr is the new head when n > q.len.

		q.history = append(q.history, newHistory...)

		if q.len <= n {
			q.Clear()
			return
		}

		q.len -= n
		if q.len == 1 {
			curr.next = curr
		}
		curr.prev = q.head.prev
		q.head = curr

	case protocol.RepeatStateTrack:
		// Don't do anything: we continue playing the same track because its
		// being repeated, and we don't touch the history because building up a
		// pile of entries for the same track isn't very helpful either.

	case protocol.RepeatStateQueue:
		var repeatedPartLen int
		if q.Shuffle {
			// Determine the distance from q.repeatStart to q.head. This loop
			// must exit because q.repeatStart and q.head must both be in the
			// list.
			repeatedPartLen = 0
			curr := q.repeatStart
			for curr != q.head {
				curr = curr.next
				repeatedPartLen++
			}
		}

		for i := uint(0); i < n; i++ {
			restoreNode := q.head
			// restoreRepeatStart is necessary because if we finish something
			// then move it to
			var restoreRepeatStart *node[T]
			q.history = append(q.history, func(q *Queue[T]) bool {
				if restoreNode.removed {
					return false
				}

				if q.Empty() {
					restoreNode.next = restoreNode
					restoreNode.prev = restoreNode

					q.head = restoreNode
					q.repeatStart = restoreNode
					return true
				}

				// Remove from current position.
				restoreNode.next.prev = restoreNode.prev
				restoreNode.prev.next = restoreNode.next

				// Insert as head.
				restoreNode.prev = q.head.prev
				q.head.prev.next = restoreNode
				restoreNode.next = q.head
				q.head.prev = restoreNode

				q.head = restoreNode
				if restoreRepeatStart != nil {
					q.repeatStart = restoreRepeatStart
				}

				return true
			})
			q.head = restoreNode.next

			if !q.Shuffle {
				continue
			}

			// Next we want to remove and re-insert in a random position between
			// q.repeatStart and q.head.

			// Special case: if we just skipped the repeatStart, it always go
			// straight to the end because this was the start of a new repeat,
			// so none of the other tracks in the queue have been played yet in
			// the newly started repeat.
			if restoreNode == q.repeatStart {
				// So in this case we don't have to do anything because it will
				// automatically become the last node now that we've shifted
				// q.head.
				repeatedPartLen = 1
				continue
			}

			// Remove from current position.
			restoreNode.prev.next = restoreNode.next
			restoreNode.next.prev = restoreNode.prev

			// If this is the last value in this repeat of the queue...
			var indexWithinRepeated int
			if restoreNode.next == q.repeatStart {
				// Disallow re-insertion at index 0 in the repeated section,
				// because this would put a value right back where it was which
				// would be non-ideal.
				indexWithinRepeated = (rand.Int() % repeatedPartLen) + 1
			} else {
				indexWithinRepeated = rand.Int() % (repeatedPartLen + 1)
			}

			curr := q.repeatStart
			for i := indexWithinRepeated; i > 0; i-- {
				curr = curr.next
			}
			restoreNode.prev = curr.prev
			restoreNode.next = curr
			curr.prev.next = restoreNode
			curr.prev = restoreNode

			if indexWithinRepeated == 0 {
				restoreRepeatStart = q.repeatStart
				q.repeatStart = restoreNode
			}
			repeatedPartLen++
		}

	default:
		panic("invalid Repeat")
	}
}

// skipRV skips backward.
func (q *Queue[T]) skipRV(n uint) {
	for n > 0 && len(q.history) > 0 {
		if q.history[len(q.history)-1](q) {
			// Only count it as a successful step backwards if something
			// happened.
			n--
		}

		q.history = q.history[:len(q.history)-1]
	}
}

// Insert adds the given track to the queue at the specified index. It returns
// whether the index was in bounds (and the track was therefore successfully
// inserted).
func (q *Queue[T]) Insert(v T, i uint) bool {
	if i > q.len {
		return false
	}

	vn := &node[T]{value: v}

	if q.Empty() {
		vn.next = vn
		vn.prev = vn
		q.head = vn
		q.repeatStart = vn

		q.len += 1

		return true
	}

	curr := q.head
	for j := uint(0); j < i; j++ {
		curr = curr.next
	}

	curr.prev.next = vn
	vn.prev = curr.prev
	curr.prev = vn
	vn.next = curr

	if i == 0 {
		// If we're inserting at 0, this is the new head, but not the new
		// repeatStart.
		q.head = vn
	}

	q.len += 1

	return true
}

// Remove removes the track at the specified index. It returns the track that
// was removed and true, if the index was in bounds. Otherwise, it returns the
// zero value for T and false.
func (q *Queue[T]) Remove(i uint) (v T, ok bool) {
	if i >= q.len {
		var z T
		return z, false
	}

	if q.len == 1 {
		// The queue will be empty after this operation.
		res := q.head
		q.head.removed = true

		q.Clear()

		return res.value, true
	}

	curr := q.head
	for j := uint(0); j < i; j++ {
		curr = curr.next
	}

	curr.prev.next = curr.next
	curr.next.prev = curr.prev
	curr.removed = true
	if curr == q.repeatStart {
		q.repeatStart = curr.next
	}

	if i == 0 {
		// If we're removing the head, set the new head to the track after the
		// current head.
		q.head = curr.next
	}

	q.len -= 1

	return curr.value, true
}

// Clear removes all tracks from the queue.
func (q *Queue[T]) Clear() {
	q.head = nil
	q.len = 0
	q.repeatStart = nil
}

// Len returns the length of the queue.
func (q Queue[T]) Len() uint { return q.len }

// Reshuffle pseudo-randomly re-orders the queue.
func (q *Queue[T]) Reshuffle() {
	if q.Len() <= 2 {
		// We can't do anything (and might accidentally pass rand.Shuffle an
		// invalid input) if we don't have at least three values in the queue.
		return
	}

	s := q.To()

	rand.Shuffle(len(s)-1, func(i, j int) {
		s[i+1], s[j+1] = s[j+1], s[i+1]
	})

	*q = QueueFrom(s)
}

// To returns an ordered slice representation of the queue.
func (q Queue[T]) To() []T {
	res := make([]T, q.len)

	for i, curr := uint(0), q.head; i < q.len; i, curr = i+1, curr.next {
		res[i] = curr.value
	}

	return res
}
