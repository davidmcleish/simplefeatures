package geom

import (
	"encoding/binary"
	"fmt"
	"math"
)

// MarshalTWKB accepts a geometry and generates the corresponding TWKB byte slice.
func MarshalTWKB(geom Geometry,
	hasZ, hasM bool,
	precXY, precZ, precM int,
	hasSize, hasBBox, closeRings bool,
	idList []int64,
) ([]byte, error) {
	w := newtwkbWriter(hasZ, hasM, precXY, precZ, precM, hasSize, hasBBox, closeRings, idList)
	if err := w.writeGeometry(geom); err != nil {
		return nil, fmt.Errorf("failed to marshal TWKB: %w", err)
	}
	return w.formTWKB(), nil
}

// twkbWriter holds all state information needed for generating TWKB data
// including information such as the last reference point used in coord deltas.
type twkbWriter struct {
	twkbHeaders  []byte
	twkbBBox     []byte
	twkbContents []byte

	kind  twkbGeometryType
	ctype CoordinatesType

	dimensions int
	precXY     int
	hasZ       bool
	hasM       bool
	precZ      int
	precM      int
	scalings   [twkbMaxDimensions]float64

	hasBBox bool
	hasSize bool
	hasIDs  bool
	hasExt  bool
	isEmpty bool

	refpoint [twkbMaxDimensions]int64

	bboxValid bool
	bboxMin   [twkbMaxDimensions]int64
	bboxMax   [twkbMaxDimensions]int64

	idList []int64

	closeRings bool
}

func newtwkbWriter(
	hasZ, hasM bool,
	precXY, precZ, precM int,
	hasSize, hasBBox, closeRings bool,
	idList []int64,
) *twkbWriter {
	w := twkbWriter{
		hasSize:    hasSize,
		hasBBox:    hasBBox,
		closeRings: closeRings,
	}

	w.precXY = precXY
	w.scalings[0] = math.Pow10(precXY)
	w.scalings[1] = w.scalings[0]
	w.dimensions = 2

	if hasZ {
		w.hasZ = true
		w.precZ = precZ
		w.scalings[w.dimensions] = math.Pow10(precZ)
		w.dimensions++
	}

	if hasM {
		w.hasM = true
		w.precM = precM
		w.scalings[w.dimensions] = math.Pow10(precM)
		w.dimensions++
	}

	if hasZ && hasM {
		w.ctype = DimXYZM
	} else if hasZ {
		w.ctype = DimXYZ
	} else if hasM {
		w.ctype = DimXYM
	} else {
		w.ctype = DimXY
	}

	w.hasExt = hasZ || hasM

	if len(idList) > 0 {
		w.hasIDs = true
		w.idList = idList
	}

	return &w
}

func copytwkbWriter(other *twkbWriter) *twkbWriter {
	return newtwkbWriter(
		other.hasZ,       // Assume child has same dimensionality as parent.
		other.hasM,       // Assume child has same dimensionality as parent.
		other.precXY,     // Same precision as in parent.
		other.precZ,      // Same precision as in parent.
		other.precM,      // Same precision as in parent.
		other.hasSize,    // If parent is using a size header, child should too.
		false,            // No bbox in sub-geometries.
		other.closeRings, // If parent requires closed rings, child should too.
		nil,              // No ID list in sub-geometries.
	)
}

func (w *twkbWriter) formTWKB() []byte {
	var data []byte
	data = append(data, w.twkbHeaders...)
	data = append(data, w.twkbBBox...)
	data = append(data, w.twkbContents...)
	return data
}

func (w *twkbWriter) writeGeometry(geom Geometry) error {
	if err := w.writeGeometryByType(geom); err != nil {
		return err
	}
	w.writeAdditionalHeaders()
	return nil
}

func (w *twkbWriter) writeGeometryByType(geom Geometry) error {
	switch geom.gtype {
	case TypePoint:
		return w.writePoint(geom.MustAsPoint())
	case TypeLineString:
		return w.writeLineString(geom.MustAsLineString())
	case TypePolygon:
		return w.writePolygon(geom.MustAsPolygon())
	case TypeMultiPoint:
		return w.writeMultiPoint(geom.MustAsMultiPoint())
	case TypeMultiLineString:
		return w.writeMultiLineString(geom.MustAsMultiLineString())
	case TypeMultiPolygon:
		return w.writeMultiPolygon(geom.MustAsMultiPolygon())
	case TypeGeometryCollection:
		return w.writeGeometryCollection(geom.MustAsGeometryCollection())
	default:
		return fmt.Errorf("geometry has unsupported type: %q", geom.gtype)
	}
}

func (w *twkbWriter) writePoint(pt Point) error {
	w.writeTypeAndPrecision(twkbTypePoint)

	if ctype := pt.CoordinatesType(); ctype != w.ctype {
		return fmt.Errorf("mismatched Point coordinate dimensions got %s expected %s", ctype, w.ctype)
	}

	if pt.IsEmpty() {
		w.writeIsEmptyHeader()
		return nil
	}
	w.writeInitialHeaders()

	return w.writePointCoords(pt)
}

func (w *twkbWriter) writePointCoords(pt Point) error {
	switch pt.CoordinatesType() {
	case DimXY:
		w.writePoints(1, pt.coords.XY.X, pt.coords.XY.Y)
	case DimXYZ:
		w.writePoints(1, pt.coords.XY.X, pt.coords.XY.Y, pt.coords.Z)
	case DimXYM:
		w.writePoints(1, pt.coords.XY.X, pt.coords.XY.Y, pt.coords.M)
	case DimXYZM:
		w.writePoints(1, pt.coords.XY.X, pt.coords.XY.Y, pt.coords.Z, pt.coords.M)
	default:
		return fmt.Errorf("point has unsupported type %s", pt.CoordinatesType())
	}
	return nil
}

func (w *twkbWriter) writeLineString(ls LineString) error {
	w.writeTypeAndPrecision(twkbTypeLineString)

	if ctype := ls.CoordinatesType(); ctype != w.ctype {
		return fmt.Errorf("mismatched LineString coordinate dimensions got %s expected %s", ctype, w.ctype)
	}

	if ls.IsEmpty() {
		w.writeIsEmptyHeader()
		return nil
	}
	w.writeInitialHeaders()

	return w.writeLineStringCoords(ls)
}

func (w *twkbWriter) writeLineStringCoords(ls LineString) error {
	coords := ls.Coordinates()
	numPoints := coords.Length()
	w.writeUnsignedVarint(uint64(numPoints))
	w.writePointArray(numPoints, coords.floats)
	return nil
}

func (w *twkbWriter) writeRing(ls LineString) error {
	coords := ls.Coordinates()
	numPoints := coords.Length()
	if !w.closeRings && numPoints >= 2 {
		numPoints-- // Omit the final point in the ring.
	}
	w.writeUnsignedVarint(uint64(numPoints))
	w.writePointArray(numPoints, coords.floats)
	return nil
}

func (w *twkbWriter) writePolygon(poly Polygon) error {
	w.writeTypeAndPrecision(twkbTypePolygon)

	if ctype := poly.CoordinatesType(); ctype != w.ctype {
		return fmt.Errorf("mismatched Polygon coordinate dimensions got %s expected %s", ctype, w.ctype)
	}

	if poly.IsEmpty() {
		w.writeIsEmptyHeader()
		return nil
	}
	w.writeInitialHeaders()

	return w.writePolygonRings(poly)
}

func (w *twkbWriter) writePolygonRings(poly Polygon) error {
	w.writeUnsignedVarint(uint64(poly.NumRings()))

	if poly.NumRings() == 0 {
		return nil
	}

	ls := poly.ExteriorRing()
	w.writeRing(ls)

	numRings := poly.NumInteriorRings()
	for i := 0; i < numRings; i++ {
		ls = poly.InteriorRingN(i)
		w.writeRing(ls)
	}
	return nil
}

func (w *twkbWriter) writeMultiPoint(mp MultiPoint) error {
	w.writeTypeAndPrecision(twkbTypeMultiPoint)

	if ctype := mp.CoordinatesType(); ctype != w.ctype {
		return fmt.Errorf("mismatched MultiPoint coordinate dimensions got %s expected %s", ctype, w.ctype)
	}

	if mp.IsEmpty() {
		w.writeIsEmptyHeader()
		return nil
	}
	w.writeInitialHeaders()

	numPoints := mp.NumPoints()
	w.writeUnsignedVarint(uint64(numPoints))

	if err := w.writeIDList(numPoints); err != nil {
		return err
	}

	for i := 0; i < numPoints; i++ {
		pt := mp.PointN(i)
		w.writePointCoords(pt)
	}
	return nil
}

func (w *twkbWriter) writeMultiLineString(ml MultiLineString) error {
	w.writeTypeAndPrecision(twkbTypeMultiLineString)

	if ctype := ml.CoordinatesType(); ctype != w.ctype {
		return fmt.Errorf("mismatched MultiLineString coordinate dimensions got %s expected %s", ctype, w.ctype)
	}

	if ml.IsEmpty() {
		w.writeIsEmptyHeader()
		return nil
	}
	w.writeInitialHeaders()

	numLineStrings := ml.NumLineStrings()
	w.writeUnsignedVarint(uint64(numLineStrings))

	if err := w.writeIDList(numLineStrings); err != nil {
		return err
	}

	for i := 0; i < numLineStrings; i++ {
		ls := ml.LineStringN(i)
		w.writeLineStringCoords(ls)
	}
	return nil
}

func (w *twkbWriter) writeMultiPolygon(mp MultiPolygon) error {
	w.writeTypeAndPrecision(twkbTypeMultiPolygon)

	if ctype := mp.CoordinatesType(); ctype != w.ctype {
		return fmt.Errorf("mismatched MultiPolygon coordinate dimensions got %s expected %s", ctype, w.ctype)
	}

	if mp.IsEmpty() {
		w.writeIsEmptyHeader()
		return nil
	}
	w.writeInitialHeaders()

	numPolygons := mp.NumPolygons()
	w.writeUnsignedVarint(uint64(numPolygons))

	if err := w.writeIDList(numPolygons); err != nil {
		return err
	}

	for i := 0; i < numPolygons; i++ {
		poly := mp.PolygonN(i)
		w.writePolygonRings(poly)
	}
	return nil
}

func (w *twkbWriter) writeGeometryCollection(gc GeometryCollection) error {
	w.writeTypeAndPrecision(twkbTypeGeometryCollection)

	if ctype := gc.CoordinatesType(); ctype != w.ctype {
		return fmt.Errorf("mismatched GeometryCollection coordinate dimensions got %s expected %s", ctype, w.ctype)
	}

	if gc.IsEmpty() {
		w.writeIsEmptyHeader()
		return nil
	}
	w.writeInitialHeaders()

	numGeometries := gc.NumGeometries()
	w.writeUnsignedVarint(uint64(numGeometries))

	if err := w.writeIDList(numGeometries); err != nil {
		return err
	}

	for i := 0; i < numGeometries; i++ {
		subWriter := copytwkbWriter(w)
		geom := gc.GeometryN(i)
		subWriter.writeGeometry(geom)
		subTWKB := subWriter.formTWKB()
		w.twkbContents = append(w.twkbContents, subTWKB...)
	}
	return nil
}

func (w *twkbWriter) writeTypeAndPrecision(kind twkbGeometryType) {
	w.kind = kind
	w.writeHeaderByte(byte(EncodeZigZagInt32(int32(w.precXY))<<4) | byte(w.kind))
}

func (w *twkbWriter) writeIsEmptyHeader() {
	w.isEmpty = true
	// Because this is an empty object, we only need to write the "is empty" bit.
	// In particular, we do not write any extended info, size, bbox, or ids,
	// even if those were available or requested.
	w.writeMetadataHeader(twkbIsEmpty)
}

func (w *twkbWriter) writeInitialHeaders() {
	var metaheader twkbMetadataHeader
	if w.hasExt {
		metaheader |= twkbHasExtPrec
	}
	if w.hasSize {
		metaheader |= twkbHasSize
	}
	if w.hasBBox {
		metaheader |= twkbHasBBox
	}
	if w.hasIDs {
		metaheader |= twkbHasIDs
	}
	w.writeMetadataHeader(metaheader)

	if w.hasExt {
		w.writeExtendedPrecision()
	}
}

func (w *twkbWriter) writeMetadataHeader(metaheader twkbMetadataHeader) {
	if metaheader&twkbIsEmpty != 0 {
		w.isEmpty = true
	}
	w.writeHeaderByte(byte(metaheader))
}

func (w *twkbWriter) writeExtendedPrecision() {
	var ext byte
	if w.hasZ {
		ext |= 0x01
		ext |= byte(EncodeZigZagInt32(int32(w.precZ)) << 2)
	}
	if w.hasM {
		ext |= 0x02
		ext |= byte(EncodeZigZagInt32(int32(w.precM)) << 5)
	}
	w.writeHeaderByte(ext)
}

func (w *twkbWriter) writePoints(numPoints int, coords ...float64) {
	w.writePointArray(numPoints, coords)
}

// Convert a given number of points from floating point to integer coordinates.
// Utilise and update the running memory of the previous reference point.
// The input coords must contain numPoints * the number of dimensions values.
func (w *twkbWriter) writePointArray(numPoints int, coords []float64) {
	var buf [binary.MaxVarintLen64]byte
	c := 0
	for i := 0; i < numPoints; i++ {
		for d := 0; d < w.dimensions; d++ {
			fval := coords[c]
			ival := int64(fval * w.scalings[d])
			// Compute bounding box.
			if !w.bboxValid {
				w.bboxMin[d] = ival
				w.bboxMax[d] = ival
			} else if ival < w.bboxMin[d] {
				w.bboxMin[d] = ival
			} else if ival > w.bboxMax[d] {
				w.bboxMax[d] = ival
			}
			// Perform coord differencing to find the int value.
			ival -= w.refpoint[d]
			n := binary.PutVarint(buf[:], ival)

			w.twkbContents = append(w.twkbContents, buf[:n]...)
			w.refpoint[d] += ival
			c++
		}
		// After at least one point, the bounding box becomes valid.
		if i == 0 {
			w.bboxValid = true
		}
	}
}

func (w *twkbWriter) writeAdditionalHeaders() {
	// These are written in this order so that the size of the
	// bbox is included in the size computation.
	if w.hasBBox {
		w.writeBBoxHeader()
	}
	if w.hasSize {
		w.writeSizeHeader(len(w.twkbBBox), len(w.twkbContents))
	}
}

func (w *twkbWriter) writeBBoxHeader() {
	// Store bbox min and delta for each dimension.
	var buf [twkbMaxDimensions * 2 * binary.MaxVarintLen64]byte
	n := 0
	for d := 0; d < w.dimensions; d++ {
		n += binary.PutVarint(buf[n:], w.bboxMin[d])
		n += binary.PutVarint(buf[n:], w.bboxMax[d]-w.bboxMin[d])
	}
	// Insert the bbox varints into the appropriate header.
	w.twkbBBox = append(w.twkbBBox, buf[:n]...)
}

func (w *twkbWriter) writeSizeHeader(bboxLength, contentsLength int) {
	// Form size header as a varint covering all data after
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], uint64(bboxLength+contentsLength))

	// Insert the size header after any other headers.
	w.twkbHeaders = append(w.twkbHeaders, buf[:n]...)
}

func (w *twkbWriter) writeIDList(num int) error {
	if !w.hasIDs {
		return nil
	}
	if num != len(w.idList) {
		return fmt.Errorf("unexpected ID list length %d, expected %d", len(w.idList), num)
	}
	for i := 0; i < num; i++ {
		w.writeSignedVarint(w.idList[i])
	}
	return nil
}

func (w *twkbWriter) writeSignedVarint(val int64) int {
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutVarint(buf[:], val)
	w.twkbContents = append(w.twkbContents, buf[:n]...)
	return n
}

func (w *twkbWriter) writeUnsignedVarint(val uint64) int {
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], val)
	w.twkbContents = append(w.twkbContents, buf[:n]...)
	return n
}

func (w *twkbWriter) writeHeaderByte(b byte) {
	w.twkbHeaders = append(w.twkbHeaders, b)
}
