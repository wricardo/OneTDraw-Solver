package solver

type SolutionHandler interface {
	handleNewSolutionFound(*[]uint16)
}

type solutionStorer struct {
	solutions []*Solution
}

func newSolutionStorer() *solutionStorer {
	sc := new(solutionStorer)
	return sc
}
func (this *solutionStorer) handleNewSolutionFound(path *[]uint16) {
	s := make(Solution, len(*path))
	s = *path
	this.solutions = append(this.solutions, &s)
}

type solutionCounter struct {
	count_solutions int
}

func newSolutionCounter() *solutionCounter {
	sc := new(solutionCounter)
	sc.count_solutions = 0
	return sc
}

func (this *solutionCounter) handleNewSolutionFound(path *[]uint16) {
	this.count_solutions = this.count_solutions + 1
}
