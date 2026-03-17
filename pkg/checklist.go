package pkg

import "math/rand/v2"

// Checks is a collection of check definitions.
type Checks []*Check

// ChecksIterator provides sequential access to a Checks collection.
type ChecksIterator interface {
	Fetch() *Check
	ShuffleIfNeeded()
}

// ChecksIteratorImpl implements ChecksIterator with index-based iteration.
type ChecksIteratorImpl struct {
	checks Checks
	index  int
	limit  int
}

// CheckList contains ordered and shuffled check collections.
type CheckList struct {
	Ordered  Checks
	Shuffled Checks
}

// CheckListIterator provides sequential access to both ordered and shuffled checks.
type CheckListIterator interface {
	Fetch() *Check
}

// CheckListIteratorImpl implements CheckListIterator for ordered then shuffled access.
type CheckListIteratorImpl struct {
	orderedIterator  ChecksIterator
	shuffledIterator ChecksIterator
}

// NewChecksIterator creates a new iterator for the given checks.
func NewChecksIterator(checks Checks) *ChecksIteratorImpl {
	return &ChecksIteratorImpl{
		checks: checks,
		index:  0,
		limit:  len(checks),
	}
}

// Shuffle randomizes the order of checks in place.
func (checks Checks) Shuffle() {
	rand.Shuffle(len(checks), func(i, j int) {
		checks[i], checks[j] = checks[j], checks[i]
	})
}

// ShuffleIfNeeded shuffles the checks on first iteration only.
func (it *ChecksIteratorImpl) ShuffleIfNeeded() {
	if it.index > 0 {
		return
	}
	it.checks.Shuffle()
}

// Fetch returns the next check or nil if exhausted.
func (it *ChecksIteratorImpl) Fetch() *Check {
	if it.index < it.limit {
		check := it.checks[it.index]
		it.index++

		return check
	}

	return nil
}

// GetIterator creates a new iterator over both ordered and shuffled checks.
func (cl *CheckList) GetIterator() *CheckListIteratorImpl {
	return &CheckListIteratorImpl{
		orderedIterator:  NewChecksIterator(cl.Ordered),
		shuffledIterator: NewChecksIterator(cl.Shuffled),
	}
}

// Fetch returns the next check from ordered then shuffled lists.
func (it *CheckListIteratorImpl) Fetch() *Check {
	var check *Check
	check = it.orderedIterator.Fetch()
	if check != nil {
		return check
	}
	it.shuffledIterator.ShuffleIfNeeded()
	check = it.shuffledIterator.Fetch()
	if check != nil {
		return check
	}

	return nil
}
