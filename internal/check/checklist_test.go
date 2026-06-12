package check

import (
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newCheck(_ int) *Check {
	return &Check{}
}

func TestListAll_OrderedFirst(t *testing.T) {
	ordered := Checks{newCheck(1), newCheck(2)}
	shuffled := Checks{newCheck(3), newCheck(4)}
	cl := &List{Ordered: ordered, Shuffled: shuffled}

	got := slices.Collect(cl.All())

	assert.Len(t, got, 4)
	assert.Same(t, got[0], ordered[0], "first check should be from ordered")
	assert.Same(t, got[1], ordered[1], "second check should be from ordered")
	assert.True(t, sameElements(got[2:], shuffled), "shuffled checks lost elements")
}

func TestListAll_Empty(t *testing.T) {
	cl := &List{}
	assert.Empty(t, slices.Collect(cl.All()))
}

func TestListAll_EarlyBreak(t *testing.T) {
	cl := &List{Ordered: Checks{newCheck(1), newCheck(2)}}

	count := 0
	for range cl.All() {
		count++

		break
	}

	assert.Equal(t, 1, count)
}

func TestListAll_DoesNotMutateShuffled(t *testing.T) {
	shuffled := Checks{newCheck(1), newCheck(2), newCheck(3), newCheck(4)}
	orig := slices.Clone(shuffled)
	cl := &List{Shuffled: shuffled}

	got := slices.Collect(cl.All())

	assert.Len(t, got, len(orig))
	assert.True(t, slices.Equal(orig, shuffled), "All() should not reorder the underlying slice")
}

// Helper: check if two slices have the same elements (ignoring order).
func sameElements(a, b Checks) bool {
	if len(a) != len(b) {
		return false
	}

	ma := make(map[*Check]int)
	mb := make(map[*Check]int)

	for _, x := range a {
		ma[x]++
	}

	for _, x := range b {
		mb[x]++
	}

	return maps.Equal(ma, mb)
}
