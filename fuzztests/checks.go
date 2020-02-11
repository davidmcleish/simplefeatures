package main

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/peterstace/simplefeatures/geom"
)

func CheckWKTParse(t *testing.T, pg PostGIS, candidates []string) {
	var any bool
	for i, wkt := range candidates {
		any = true
		t.Run(fmt.Sprintf("CheckWKTParse_%d", i), func(t *testing.T) {

			// The simple feature library accepts LINEARRING WKTs. However,
			// postgis doesn't accept them. A workaround for this is to just
			// substitute LINEARRING for LINESTRING. However, this will give a
			// false negative if the corpus contains a LINEARRING WKT that
			// isn't closed (and thus won't be accepted by simple features).
			wkt := strings.ReplaceAll(wkt, "LINEARRING", "LINESTRING")

			_, sfErr := geom.UnmarshalWKT(strings.NewReader(wkt))
			isValid, reason := pg.WKTIsValidWithReason(wkt)
			if (sfErr == nil) != isValid {
				t.Logf("SimpleFeatures err: %v", sfErr)
				t.Logf("PostGIS IsValid: %v", isValid)
				t.Logf("PostGIS Reason: %v", reason)
				t.Errorf("mismatch")
			}
		})
	}
	if !any {
		// We know there are some some valid WKT strings, so if this happens
		// then something is wrong with the extraction or conversion logic.
		t.Errorf("could not extract any WKTs")
	}
}

func CheckWKBParse(t *testing.T, pg PostGIS, candidates []string) {
	var any bool
	for i, wkb := range candidates {
		buf, err := hexStringToBytes(wkb)
		if err != nil {
			continue
		}
		any = true
		t.Run(fmt.Sprintf("CheckWKBParse_%d", i), func(t *testing.T) {
			_, sfErr := geom.UnmarshalWKB(bytes.NewReader(buf))
			isValid, reason := pg.WKBIsValidWithReason(t, wkb)
			if (sfErr == nil) != isValid {
				t.Logf("WKB: %v", wkb)
				t.Logf("SimpleFeatures err: %v", sfErr)
				t.Logf("PostGIS IsValid: %v", isValid)
				t.Logf("PostGIS Reason: %v", reason)
				t.Errorf("mismatch")
			}
		})
	}
	if !any {
		// We know there are some some valid hex strings, so if this happens
		// then something is wrong with the extraction or conversion logic.
		t.Errorf("could not extract any WKBs")
	}
}

func hexStringToBytes(s string) ([]byte, error) {
	if len(s)%2 != 0 {
		return nil, errors.New("hex string must have even length")
	}
	var buf []byte
	for i := 0; i < len(s); i += 2 {
		x, err := strconv.ParseUint(s[i:i+2], 16, 8)
		if err != nil {
			return nil, err
		}
		buf = append(buf, byte(x))
	}
	return buf, nil
}

func CheckGeoJSONParse(t *testing.T, pg PostGIS, candidates []string) {
	var any bool
	for i, geojson := range candidates {
		if geojson == `{"type":"Point","coordinates":[]}` {
			// From https://tools.ietf.org/html/rfc7946#section-3.1:
			//
			// > GeoJSON processors MAY interpret Geometry objects with
			// > empty "coordinates" arrays as null objects.
			//
			// Simplefeatures chooses to accept this as an empty point, but
			// Postgres rejects it.
			continue
		}
		any = true
		t.Run(fmt.Sprintf("CheckGeoJSONParse_%d", i), func(t *testing.T) {
			_, sfErr := geom.UnmarshalGeoJSON([]byte(geojson))
			isValid, reason := pg.GeoJSONIsValidWithReason(t, geojson)
			if (sfErr == nil) != isValid {
				t.Logf("GeoJSON: %v", geojson)
				t.Logf("SimpleFeatures err: %v", sfErr)
				t.Logf("PostGIS IsValid: %v", isValid)
				t.Logf("PostGIS Reason: %v", reason)
				t.Errorf("mismatch")
			}
		})
	}
	if !any {
		// We know there are some some valid geojson strings, so if this happens
		// then something is wrong with the extraction or conversion logic.
		t.Errorf("could not extract any geojsons")
	}
}

func CheckWKT(t *testing.T, want UnaryResult, g geom.Geometry) {
	t.Run("CheckWKT", func(t *testing.T) {
		got := g.AsText()
		if strings.Contains(got, "MULTIPOINT") {
			// Skip Multipoints. This is because Postgis doesn't follow the SFA
			// spec by not including parenthesis around each individual point.
			// The simplefeatures library follows the spec correctly.
			return
		}
		want := want.AsText
		if want != got {
			t.Logf("got:  %v", got)
			t.Logf("want: %v", want)
			t.Error("mismatch")
		}
	})
}

func CheckWKB(t *testing.T, want UnaryResult, g geom.Geometry) {
	t.Run("CheckWKB", func(t *testing.T) {
		if containsEmptyPoint(g) {
			// Empty point WKB use NaN as part of their representation.
			// Go's math.NaN() and Postgis use slightly different (but
			// compatible) representations of NaN.
			return
		}
		if g.IsGeometryCollection() && g.IsEmpty() {
			// The behaviour for GeometryCollections in Postgis is to just
			// give 'GEOMETRYCOLLECTION EMPTY' whenever the contents of a
			// geometry collection are all empty geometries. This doesn't
			// seem like correct behaviour, so these cases are skipped.
			return
		}
		var got bytes.Buffer
		if err := g.AsBinary(&got); err != nil {
			t.Fatalf("writing wkb: %v", err)
		}
		want := want.AsBinary
		if !bytes.Equal(got.Bytes(), want) {
			t.Logf("got:  %v", got.Bytes())
			t.Logf("want: %v", want)
			t.Error("mismatch")
		}
	})
}

func containsEmptyPoint(g geom.Geometry) bool {
	if g.IsEmptySet() && g.AsText() == "POINT EMPTY" {
		return true
	}
	if !g.IsGeometryCollection() {
		return false
	}
	gc := g.AsGeometryCollection()
	for i := 0; i < gc.NumGeometries(); i++ {
		return containsEmptyPoint(gc.GeometryN(i))
	}
	return false
}

func CheckGeoJSON(t *testing.T, want UnaryResult, g geom.Geometry) {
	t.Run("CheckGeoJSON", func(t *testing.T) {
		got, err := g.MarshalJSON()
		if err != nil {
			t.Fatalf("could not convert to geojson: %v", err)
		}
		want := want.AsGeoJSON
		if want.Valid && string(got) != want.String {
			t.Logf("got:  %v", string(got))
			t.Logf("want: %v", want)
			t.Error("mismatch")
		}
	})
}

func CheckIsEmpty(t *testing.T, want UnaryResult, g geom.Geometry) {
	t.Run("CheckIsEmpty", func(t *testing.T) {
		got := g.IsEmpty()
		want := want.IsEmpty
		if got != want {
			t.Logf("got:  %v", got)
			t.Logf("want: %v", want)
			t.Error("mismatch")
		}
	})
}

func CheckDimension(t *testing.T, want UnaryResult, g geom.Geometry) {
	t.Run("CheckDimension", func(t *testing.T) {
		got := g.Dimension()
		want := want.Dimension
		if got != want {
			t.Logf("got:  %v", got)
			t.Logf("want: %v", want)
			t.Error("mismatch")
		}
	})
}

func CheckEnvelope(t *testing.T, want UnaryResult, g geom.Geometry) {
	t.Run("CheckEnvelope", func(t *testing.T) {
		if g.IsEmpty() {
			// PostGIS allows envelopes on empty geometries, but they are empty
			// envelopes. In simplefeatures, an envelope is never empty, so we
			// skip testing that case.
			return
		}
		env, ok := g.Envelope()
		if !ok {
			// We just checked IsEmpty, so this should never happen.
			panic("could not get envelope")
		}
		got := env.AsGeometry()
		want := want.Envelope

		if !got.EqualsExact(want) {
			t.Logf("got:  %v", got.AsText())
			t.Logf("want: %v", want.AsText())
			t.Error("mismatch")
		}
	})
}

func CheckIsSimple(t *testing.T, want UnaryResult, g geom.Geometry) {
	t.Run("CheckIsSimple", func(t *testing.T) {
		got, ok := g.IsSimple()
		if ok != want.IsSimple.Valid {
			t.Fatalf("Unexpected IsSimple validity, want "+
				"valid %v, got valid %v", want.IsSimple.Valid, ok)
		}
		if !want.IsSimple.Valid {
			return
		}

		// PostGIS doesn't treat MultiLineStrings containing duplicated
		// LineStrings as non-simple, e.g. MULTILINESTRING((0 0,1 1),(0 0,1
		// 1)). This doesn't seem like correct behaviour to me. It must be
		// deduplicating the LineStrings before checking simplicity. This
		// library doesn't do that, so skip any LineStrings that contain
		// duplicates.
		if g.IsMultiLineString() {
			mls := g.AsMultiLineString()
			n := mls.NumLineStrings()
			for i := 0; i < n; i++ {
				for j := 0; j < n; j++ {
					if mls.LineStringN(i).EqualsExact(mls.LineStringN(j).AsGeometry()) {
						return
					}
				}
			}
		}

		want := want.IsSimple.Bool // want.IsSimple.Valid already checked
		if got != want {
			t.Logf("got:  %v", got)
			t.Logf("want: %v", want)
			t.Error("mismatch")
		}
	})
}

func CheckBoundary(t *testing.T, want UnaryResult, g geom.Geometry) {
	t.Run("CheckBoundary", func(t *testing.T) {
		if g.IsGeometryCollection() == want.Boundary.Valid {
			t.Fatal("Unexpected boundary: "+
				"IsGeometryCollection=%v WantBoundaryValid=%v",
				g.IsGeometryCollection(), want.Boundary.Valid)
		}
		if !want.Boundary.Valid {
			return
		}

		got := g.Boundary()
		want := want.Boundary
		if !got.EqualsExact(want.Geometry, geom.IgnoreOrder) {
			t.Logf("got:  %v", got.AsText())
			t.Logf("want: %v", want.Geometry.AsText())
			t.Error("mismatch")
		}
	})
}

func CheckConvexHull(t *testing.T, want UnaryResult, g geom.Geometry) {
	t.Run("CheckConvexHull", func(t *testing.T) {
		got := g.ConvexHull()
		want := want.ConvexHull
		if !got.EqualsExact(want, geom.IgnoreOrder) {
			t.Logf("got:  %v", got.AsText())
			t.Logf("want: %v", want.AsText())
			t.Error("mismatch")
		}
	})
}

func CheckIsValid(t *testing.T, want UnaryResult, g geom.Geometry) {
	t.Run("CheckIsValid", func(t *testing.T) {
		got := g.IsValid()
		want := want.IsValid
		if got != want {
			t.Logf("got:  %t", got)
			t.Logf("want: %t", want)
			t.Error("mismatch")
		}
	})
}

func CheckIsRing(t *testing.T, want UnaryResult, g geom.Geometry) {
	t.Run("CheckIsRing", func(t *testing.T) {
		isDefined := g.IsLine() ||
			g.IsLineString() ||
			(g.IsEmptySet() && g.AsEmptySet().AsText() == "LINESTRING EMPTY")
		if want.IsRing.Valid != isDefined {
			t.Fatalf("Unexpected IsString definition: "+
				"IsLineString=%v PostGISDefined=%v",
				isDefined, want.IsRing.Valid)
		}
		if !want.IsRing.Valid {
			return
		}
		var got bool
		if g.IsLineString() {
			got = g.AsLineString().IsRing()
		}
		want := want.IsRing.Bool
		if got != want {
			t.Logf("got:  %t", got)
			t.Logf("want: %t", want)
			t.Error("mismatch")
		}
	})
}

func CheckLength(t *testing.T, want UnaryResult, g geom.Geometry) {
	t.Run("CheckLength", func(t *testing.T) {
		got := g.Length()
		if math.Abs(got-want.Length) > 1e-6 {
			t.Logf("got:  %v", got)
			t.Logf("want: %v", want.Length)
			t.Error("mismatch")
		}
	})
}

func CheckEqualsExact(t *testing.T, pg PostGIS, g1, g2 geom.Geometry) {
	t.Run("CheckEqualsExact", func(t *testing.T) {
		got := g1.EqualsExact(g2)
		want := pg.OrderingEquals(t, g1, g2)
		if got != want {
			t.Logf("got:  %t", got)
			t.Logf("want: %t", want)
			t.Error("mismatch")
		}
	})
}

func CheckEquals(t *testing.T, pg PostGIS, g1, g2 geom.Geometry) {
	t.Run("CheckEquals", func(t *testing.T) {
		if g1.IsGeometryCollection() || g2.IsGeometryCollection() {
			// PostGIS cannot calculate Equals for geometry collections.
			return
		}
		got, err := g1.Equals(g2)
		if err != nil {
			return // operation not implemented
		}
		want := pg.Equals(t, g1, g2)
		if got != want {
			t.Logf("got:  %t", got)
			t.Logf("want: %t", want)
			t.Error("mismatch")
		}
	})
}

func CheckIntersects(t *testing.T, pg PostGIS, g1, g2 geom.Geometry) {
	t.Run("CheckIntersects", func(t *testing.T) {
		got := g1.Intersects(g2)
		want := pg.Intersects(t, g1, g2)
		if got != want {
			t.Logf("got:  %t", got)
			t.Logf("want: %t", want)
			t.Error("mismatch")
		}
	})
}

func CheckIntersection(t *testing.T, pg PostGIS, g1, g2 geom.Geometry) {
	t.Run("CheckIntersection", func(t *testing.T) {
		got, err := g1.Intersection(g2)
		if err != nil {
			return // operation not implemented
		}
		want := pg.Intersection(t, g1, g2)

		if got.IsEmpty() && want.IsEmpty() {
			return // Both empty, so they match.
		}

		if got.IsGeometryCollection() || want.IsGeometryCollection() {
			// GeometryCollections are not supported by ST_Equals. So there's
			// not much that we can do here.
			return
		}

		// PostGIS TolerantEquals (a chain of ST_SnapToGrid and ST_Equals) is
		// used rather than in memory ExactEquals because simplefeatures does
		// not implement intersect in exactly the same way as PostGIS.

		if !pg.TolerantEquals(t, got, want) {
			t.Logf("g1:   %s", g1.AsText())
			t.Logf("g2:   %s", g2.AsText())
			t.Logf("got:  %s", got.AsText())
			t.Logf("want: %s", want.AsText())
			t.Error("mismatch")
		}
	})
}

func CheckArea(t *testing.T, want UnaryResult, g geom.Geometry) {
	t.Run("CheckArea", func(t *testing.T) {
		got := g.Area()
		want := want.Area
		const eps = 0.000000001
		if math.Abs(got-want) > eps {
			t.Logf("got:  %v", got)
			t.Logf("want: %v", want)
			t.Error("mismatch")
		}
	})
}

func CheckCentroid(t *testing.T, want UnaryResult, g geom.Geometry) {
	t.Run("CheckCentroid", func(t *testing.T) {
		if !g.IsPolygon() && !g.IsMultiPolygon() {
			return
		}

		got, ok := g.Centroid()
		want := want.Cetroid

		if !ok {
			if !want.IsEmpty() {
				t.Log("got:  undefined")
				t.Logf("want: %v", want.AsText())
				t.Error("mismatch")
			}
		} else {
			if !got.EqualsExact(want, geom.Tolerance(0.000000001)) {
				t.Logf("got:  %v", got.AsText())
				t.Logf("want: %v", want.AsText())
				t.Error("mismatch")
			}
		}
	})
}

func CheckReverse(t *testing.T, want UnaryResult, g geom.Geometry) {
	t.Run("CheckReverse", func(t *testing.T) {
		got := g.Reverse()
		want := want.Reverse
		if !got.EqualsExact(want) {
			t.Logf("got:  %v", got.AsText())
			t.Logf("want: %v", want.AsText())
			t.Error("mismatch")
		}
	})
}
