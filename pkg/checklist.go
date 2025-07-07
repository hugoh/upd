package pkg

import "math/rand/v2"

type Checks []*Check

type ChecksIterator interface {
	Fetch() *Check
	ShuffleIfNeeded()
}

type ChecksIteratorImpl struct {
	checks Checks
	index  int
	limit  int
}

type CheckList struct {
	Ordered  Checks
	Shuffled Checks
}

type CheckListIterator interface {
	Fetch() *Check
}

type CheckListIteratorImpl struct {
	orderedIterator  ChecksIterator
	shuffledIterator ChecksIterator
}

func NewChecksIterator(checks Checks) *ChecksIteratorImpl {
	return &ChecksIteratorImpl{
		checks: checks,
		index:  0,
		limit:  len(checks),
	}
}

func (checks Checks) Shuffle() {
	rand.Shuffle(len(checks), func(i, j int) {
		checks[i], checks[j] = checks[j], checks[i]
	})
}

func (it *ChecksIteratorImpl) ShuffleIfNeeded() {
	if it.index > 0 {
		return
	}
	it.checks.Shuffle()
}

func (it *ChecksIteratorImpl) Fetch() *Check {
	if it.index < it.limit {
		check := it.checks[it.index]
		it.index++
		return check
	}
	return nil
}

func (cl *CheckList) GetIterator() *CheckListIteratorImpl {
	return &CheckListIteratorImpl{
		orderedIterator:  NewChecksIterator(cl.Ordered),
		shuffledIterator: NewChecksIterator(cl.Shuffled),
	}
}

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
