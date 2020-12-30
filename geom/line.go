package geom

import (
	"fmt"
	"math"
)

// line represents a line segment between two XY locations. It's an invariant
// that a and b are distinct XY values. Do not create a line that has the same
// a and b value.
type line struct {
	a, b XY
}

func (ln line) envelope() Envelope {
	e := Envelope{
		min: ln.a,
		max: ln.b,
	}
	if e.min.X > e.max.X {
		e.min.X, e.max.X = e.max.X, e.min.X
	}
	if e.min.Y > e.max.Y {
		e.min.Y, e.max.Y = e.max.Y, e.min.Y
	}
	return e
}

func (ln line) reverse() line {
	return line{ln.b, ln.a}
}

func (ln line) length() float64 {
	dx := ln.b.X - ln.a.X
	dy := ln.b.Y - ln.a.Y
	return math.Sqrt(dx*dx + dy*dy)
}

func (ln line) centroid() XY {
	return XY{
		0.5 * (ln.a.X + ln.b.X),
		0.5 * (ln.a.Y + ln.b.Y),
	}
}

func (ln line) minX() float64 {
	return math.Min(ln.a.X, ln.b.X)
}

func (ln line) maxX() float64 {
	return math.Max(ln.a.X, ln.b.X)
}

func (ln line) asLineString() LineString {
	ls, err := NewLineString(NewSequence([]float64{
		ln.a.X, ln.a.Y,
		ln.b.X, ln.b.Y,
	}, DimXY))
	if err != nil {
		// Should not occur, because we know that a and b are distinct.
		panic(fmt.Sprintf("could not create line string: %v", err))
	}
	return ls
}

func (ln line) intersectsXY(xy XY) bool {
	// Speed is O(1) using a bounding box check then a point-on-line check.
	env := ln.envelope()
	if !env.Contains(xy) {
		return false
	}
	lhs := (xy.X - ln.a.X) * (ln.b.Y - ln.a.Y)
	rhs := (xy.Y - ln.a.Y) * (ln.b.X - ln.a.X)
	return lhs == rhs
}

func (ln line) hasEndpoint(xy XY) bool {
	return ln.a == xy || ln.b == xy
}

// lineWithLineIntersection represents the result of intersecting two line
// segments together. It can either be empty (flag set), a single point (both
// points set the same), or a line segment (defined by the two points).
type lineWithLineIntersection struct {
	empty    bool
	ptA, ptB XY
}

// intersectLine calculates the intersection between two line
// segments without performing any heap allocations.
func (ln line) intersectLine(other line) lineWithLineIntersection {
	a := ln.a
	b := ln.b
	c := other.a
	d := other.b

	o1 := orientation(a, b, c)
	o2 := orientation(a, b, d)
	o3 := orientation(c, d, a)
	o4 := orientation(c, d, b)

	if o1 != o2 && o3 != o4 {
		if o1 == collinear {
			return lineWithLineIntersection{false, c, c}
		}
		if o2 == collinear {
			return lineWithLineIntersection{false, d, d}
		}
		if o3 == collinear {
			return lineWithLineIntersection{false, a, a}
		}
		if o4 == collinear {
			return lineWithLineIntersection{false, b, b}
		}

		e := (c.Y-d.Y)*(a.X-c.X) + (d.X-c.X)*(a.Y-c.Y)
		f := (d.X-c.X)*(a.Y-b.Y) - (a.X-b.X)*(d.Y-c.Y)
		// Division by zero is not possible, since the lines are not parallel.
		p := e / f

		pt := b.Sub(a).Scale(p).Add(a)
		return lineWithLineIntersection{false, pt, pt}
	}

	if o1 == collinear && o2 == collinear {
		if (!onSegment(a, b, c) && !onSegment(a, b, d)) && (!onSegment(c, d, a) && !onSegment(c, d, b)) {
			return lineWithLineIntersection{empty: true}
		}

		// ---------------------
		// This block is to remove the collinear points in between the two endpoints
		pts := make([]XY, 0, 4)
		pts = append(pts, a, b, c, d)
		rth := rightmostThenHighestIndex(pts)
		pts = append(pts[:rth], pts[rth+1:]...)
		ltl := leftmostThenLowestIndex(pts)
		pts = append(pts[:ltl], pts[ltl+1:]...)
		// pts[0] and pts[1] _may_ be coincident, but that's ok.
		return lineWithLineIntersection{false, pts[0], pts[1]}
		//----------------------
	}

	return lineWithLineIntersection{empty: true}
}

// onSegement checks if point r on the segment formed by p and q.
// p, q and r should be collinear
func onSegment(p XY, q XY, r XY) bool {
	return r.X <= math.Max(p.X, q.X) &&
		r.X >= math.Min(p.X, q.X) &&
		r.Y <= math.Max(p.Y, q.Y) &&
		r.Y >= math.Min(p.Y, q.Y)
}

// rightmostThenHighestIndex finds the rightmost-then-highest point
func rightmostThenHighestIndex(ps []XY) int {
	rpi := 0
	for i := 1; i < len(ps); i++ {
		if ps[i].X > ps[rpi].X ||
			(ps[i].X == ps[rpi].X &&
				ps[i].Y > ps[rpi].Y) {
			rpi = i
		}
	}
	return rpi
}

// leftmostThenLowestIndex finds the index of the leftmost-then-lowest point.
func leftmostThenLowestIndex(ps []XY) int {
	rpi := 0
	for i := 1; i < len(ps); i++ {
		if ps[i].X < ps[rpi].X ||
			(ps[i].X == ps[rpi].X &&
				ps[i].Y < ps[rpi].Y) {
			rpi = i
		}
	}
	return rpi
}
