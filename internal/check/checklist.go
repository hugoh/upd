package check

import (
	"iter"
	"math/rand/v2"
)

// Checks is a collection of check definitions.
type Checks []*Check

// List contains ordered and shuffled check collections.
type List struct {
	Ordered  Checks
	Shuffled Checks
}

// All returns an iterator over all checks: the ordered ones first, then the
// shuffled ones in a fresh random order. The permutation is only computed if
// iteration reaches the shuffled section.
func (cl *List) All() iter.Seq[*Check] {
	return func(yield func(*Check) bool) {
		for _, c := range cl.Ordered {
			if !yield(c) {
				return
			}
		}

		for _, i := range rand.Perm(len(cl.Shuffled)) {
			if !yield(cl.Shuffled[i]) {
				return
			}
		}
	}
}
