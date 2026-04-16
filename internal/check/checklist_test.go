package check

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newCheck(_ int) *Check {
	// Replace with actual Check struct if needed
	return &Check{}
}

func TestChecksIterator_Fetch(t *testing.T) {
	checks := Checks{newCheck(1), newCheck(2), newCheck(3)}
	it := NewChecksIterator(checks)

	for i := range checks {
		got := it.Fetch()
		assert.NotNil(t, got, "Fetch() = nil at index %d, want check", i)
	}

	assert.Nil(t, it.Fetch(), "Fetch() after end should return nil")
}

func TestChecksIterator_ShuffleIfNeeded(t *testing.T) {
	orig := Checks{newCheck(1), newCheck(2), newCheck(3), newCheck(4)}
	checks := make(Checks, len(orig))
	copy(checks, orig)
	it := NewChecksIterator(checks)

	// Should shuffle since index == 0
	it.ShuffleIfNeeded()
	// Can't guarantee shuffle, but can check that it's still a permutation
	assert.True(t, sameElements(checks, orig), "Shuffled checks lost elements")

	// Should NOT shuffle since index > 0
	it.index = 1
	before := make(Checks, len(checks))
	copy(before, checks)
	it.ShuffleIfNeeded()
	assert.True(t, reflect.DeepEqual(before, checks), "ShuffleIfNeeded shuffled when index > 0")
}

func TestChecks_Shuffle(t *testing.T) {
	orig := Checks{newCheck(1), newCheck(2), newCheck(3), newCheck(4)}
	checks := make(Checks, len(orig))
	copy(checks, orig)
	checks.Shuffle()
	assert.True(t, sameElements(checks, orig), "Shuffle lost elements")
	// Not guaranteed to change order, but likely
}

func TestListIterator_Fetch(t *testing.T) {
	ordered := Checks{newCheck(1), newCheck(2)}
	shuffled := Checks{newCheck(3), newCheck(4)}
	cl := &List{Ordered: ordered, Shuffled: shuffled}
	it := cl.GetIterator()

	got := []*Check{}

	for {
		c := it.Fetch()
		if c == nil {
			break
		}

		got = append(got, c)
	}

	assert.Len(t, got, 4, "Fetch() got %d checks, want 4", len(got))

	assert.Same(t, got[0], ordered[0], "first check should be from ordered")
	assert.Same(t, got[1], ordered[1], "second check should be from ordered")
}

func TestListIterator_Empty(t *testing.T) {
	cl := &List{}
	it := cl.GetIterator()
	assert.Nil(t, it.Fetch(), "Fetch() on empty iterator should return nil")
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

	return reflect.DeepEqual(ma, mb)
}
