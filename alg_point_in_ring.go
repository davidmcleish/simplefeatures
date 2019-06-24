package simplefeatures

func isPointInsideOrOnRing(pt XY, ring LinearRing) bool {
	ptg, err := NewPointFromCoords(Coordinates{pt})
	if err != nil {
		panic(err)
	}
	// find max x coordinate
	// TODO: should be able to use envelope for this
	maxX := ring.ls.lines[0].a.X
	for _, ln := range ring.ls.lines {
		maxX = maxX.Max(ln.b.X)
		if !ln.Intersection(ptg).IsEmpty() {
			return true
		}
	}
	if pt.X.GT(maxX) {
		return false
	}

	ray, err := NewLine(Coordinates{pt}, Coordinates{XY{maxX.Add(one), pt.Y}})
	if err != nil {
		panic(err)
	}
	var count int
	for _, seg := range ring.ls.lines {
		inter := seg.Intersection(ray)
		if inter.IsEmpty() {
			continue
		}
		if inter.Dimension() == 1 {
			continue
		}
		ep1, err := NewPointFromCoords(seg.a)
		if err != nil {
			panic(err)
		}
		ep2, err := NewPointFromCoords(seg.b)
		if err != nil {
			panic(err)
		}
		if inter.Equals(ep1) || inter.Equals(ep2) {
			otherY := ep1.coords.Y
			if inter.Equals(ep1) {
				otherY = ep2.coords.Y
			}
			if otherY.LT(pt.Y) {
				count++
			}
		} else {
			count++
		}
	}
	return count%2 == 1
}
