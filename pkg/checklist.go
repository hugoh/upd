package pkg

import "math/rand/v2"

type Checks []*Check

type ChecksIterator struct {
	checks Checks
	index  int
	limit  int
}

type CheckList struct {
	Ordered  Checks
	Shuffled Checks
}

type CheckListIterator struct {
	orderedIterator  *ChecksIterator
	shuffledIterator *ChecksIterator
}

func NewChecksIterator(checks Checks) *ChecksIterator {
	return &ChecksIterator{
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

func (cl *ChecksIterator) ShuffleIfNeeded() {
	if cl.index > 0 {
		return
	}
	cl.checks.Shuffle()
}

func (it *ChecksIterator) Fetch() *Check {
	if it.index < it.limit {
		check := it.checks[it.index]
		it.index++
		return check
	}
	return nil
}

func (cl *CheckList) GetIterator() *CheckListIterator {
	return &CheckListIterator{
		orderedIterator:  NewChecksIterator(cl.Ordered),
		shuffledIterator: NewChecksIterator(cl.Shuffled),
	}
}

func (it *CheckListIterator) Fetch() *Check {
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
