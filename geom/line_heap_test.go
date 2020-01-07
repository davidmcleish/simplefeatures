package geom

import (
	"math/rand"
	"testing"
)

func TestLineHeap(t *testing.T) {
	less := func(i, j int) bool { return i < j }
	var h heap

	seed := int64(1577816310618750611)
	rnd := rand.New(rand.NewSource(seed))
	t.Logf("seed %v", seed)

	check := func() {
		for i := range h {
			childA := 2*i + 1
			childB := 2*i + 2
			le := func(i, j int) bool {
				return i <= j
			}
			if childA < len(h) {
				if !le(i, childA) {
					t.Fatal("h invariant doesn't hold")
				}
			}
			if childB < len(h) {
				if !le(i, childB) {
					t.Fatal("h invariant doesn't hold")
				}
			}
		}
	}
	push := func() {
		h.push(int(rnd.Int63()), less)
	}
	pop := func() {
		h.pop(less)
	}

	const n = 100

	for i := 0; i < n; i++ {
		push()
		check()
		push()
		check()
		pop()
		check()
	}

	for i := 0; i < n; i++ {
		pop()
		check()
	}

	if len(h) != 0 {
		t.Fatalf("not empty: %d", len(h))
	}
}
