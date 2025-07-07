//go:build unit

package pkg

import (
	"reflect"
	"testing"
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
		if got == nil {
			t.Fatalf("Fetch() = nil at index %d, want check", i)
		}
	}
	if it.Fetch() != nil {
		t.Fatal("Fetch() after end should return nil")
	}
}

func TestChecksIterator_ShuffleIfNeeded(t *testing.T) {
	orig := Checks{newCheck(1), newCheck(2), newCheck(3), newCheck(4)}
	checks := make(Checks, len(orig))
	copy(checks, orig)
	it := NewChecksIterator(checks)

	// Should shuffle since index == 0
	it.ShuffleIfNeeded()
	// Can't guarantee shuffle, but can check that it's still a permutation
	if !sameElements(checks, orig) {
		t.Fatal("Shuffled checks lost elements")
	}

	// Should NOT shuffle since index > 0
	it.index = 1
	before := make(Checks, len(checks))
	copy(before, checks)
	it.ShuffleIfNeeded()
	if !reflect.DeepEqual(before, checks) {
		t.Fatal("ShuffleIfNeeded shuffled when index > 0")
	}
}

func TestChecks_Shuffle(t *testing.T) {
	orig := Checks{newCheck(1), newCheck(2), newCheck(3), newCheck(4)}
	checks := make(Checks, len(orig))
	copy(checks, orig)
	checks.Shuffle()
	if !sameElements(checks, orig) {
		t.Fatal("Shuffle lost elements")
	}
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
	if len(got) != 4 {
		t.Fatalf("Fetch() got %d checks, want 4", len(got))
	}
}

func TestCheckListIterator_Empty(t *testing.T) {
	cl := &CheckList{}
	it := cl.GetIterator()
	if it.Fetch() != nil {
		t.Fatal("Fetch() on empty iterator should return nil")
	}
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
