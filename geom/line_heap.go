package geom

// heap is a binary heap data structure that contains Lines. The advantage
// of this implementation of a heap over the the standard container/heap
// package is that it doesn't use interface{} (and therefore doesn't allocate
// memory on each heap operation). The obvious disadvantage is that it is a
// non-trivial implementation of something that already exists. The trade off
// is worth it because the heap is used within tight loops.
type heap []int

func (h *heap) push(element int, less func(i, j int) bool) {
	*h = append(*h, element)
	i := len(*h) - 1
	for i > 0 {
		parent := (i - 1) / 2
		if less(parent, i) {
			break
		}
		(*h)[parent], (*h)[i] = (*h)[i], (*h)[parent]
		i = parent
	}
}

func (h *heap) pop(less func(i, j int) bool) {
	(*h)[0] = (*h)[len((*h))-1]
	(*h) = (*h)[:len((*h))-1]
	i := 0
	for {
		swapWith := -1
		childA := 2*i + 1
		childB := 2*i + 2
		switch {
		case childA < len((*h)) && childB < len((*h)):
			if less(i, childA) {
				if less(childB, i) {
					swapWith = childB
				}
			} else {
				swapWith = childA
				if less(childB, childA) {
					swapWith = childB
				}
			}
		case childA < len((*h)):
			if less(childA, i) {
				swapWith = childA
			}
		case childB < len((*h)):
			if less(childB, i) {
				swapWith = childB
			}
		}
		if swapWith == -1 {
			break
		}
		(*h)[swapWith], (*h)[i] = (*h)[i], (*h)[swapWith]
		i = swapWith
	}
}
