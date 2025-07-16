//go:build unit

package pkg

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type dummyCheck struct {
	id int
}

func newCheck(id int) *Check {
	// Replace with actual Check struct if needed
	return &Check{}
}

func TestChecksIterator_Fetch(t *testing.T) {
	checks := Checks{newCheck(1), newCheck(2), newCheck(3)}
	it := NewChecksIterator(checks)

	for i := 0; i < len(checks); i++ {
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

func TestCheckListIterator_Fetch(t *testing.T) {
	ordered := Checks{newCheck(1), newCheck(2)}
	shuffled := Checks{newCheck(3), newCheck(4)}
	cl := &CheckList{Ordered: ordered, Shuffled: shuffled}
	it := cl.GetIterator()

	// Should return all ordered, then all shuffled, then nil
	got := []*Check{}
	for {
		c := it.Fetch()
		if c == nil {
			break
		}
		got = append(got, c)
	}
	assert.Equal(t, 4, len(got), "Fetch() got %d checks, want 4", len(got))
	// Check that the first two are the same as ordered, in order
	assert.True(t, reflect.DeepEqual(got[:2], ordered), "First two checks are not the same as ordered: got %v, want %v", got[:2], ordered)
	// Check that the last two are the same as shuffled, in any order
	last := Checks{got[2], got[3]}
	assert.True(t, sameElements(last, shuffled), "Last two checks are not the same as shuffled (any order): got %v, want %v", last, shuffled)
}

func TestCheckListIterator_Empty(t *testing.T) {
	cl := &CheckList{}
	it := cl.GetIterator()
	assert.Nil(t, it.Fetch(), "Fetch() on empty iterator should return nil")
}

// Helper: check if two slices have the same elements (ignoring order)
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
