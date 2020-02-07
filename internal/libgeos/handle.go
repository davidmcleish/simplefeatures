package libgeos

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/peterstace/simplefeatures/geom"
)

/*
#cgo linux CFLAGS: -I/usr/include
#cgo linux LDFLAGS: -L/usr/lib -lgeos_c
#include "geos_c.h"
*/
import "C"

type Handle struct {
	context C.GEOSContextHandle_t
}

func NewHandle() *Handle {
	return &Handle{C.GEOS_init_r()}
}

func (h *Handle) Close() {
	C.GEOS_finish_r(h.context)
}

func (h *Handle) createGeomHandle(g geom.Geometry) (*C.GEOSGeometry, func()) {
	var wkb bytes.Buffer
	if err := g.AsBinary(&wkb); err != nil {
		panic(err) // can't fail writing to a buffer
	}

	reader := C.GEOSWKBReader_create_r(h.context)
	defer C.GEOSWKBReader_destroy_r(h.context, reader)
	gh := C.GEOSWKBReader_read_r(
		h.context,
		reader,
		(*C.uchar)(&wkb.Bytes()[0]),
		C.ulong(wkb.Len()),
	)
	return gh, func() { C.GEOSGeom_destroy(gh) }
}

func (h *Handle) AsText(g geom.Geometry) (string, error) {
	gh, destroy := h.createGeomHandle(g)
	defer destroy()

	writer := C.GEOSWKTWriter_create_r(h.context)
	defer C.GEOSWKTWriter_destroy_r(h.context, writer)
	wkt := C.GEOSWKTWriter_write_r(h.context, writer, gh)
	return C.GoString(wkt), nil
}

func (h *Handle) IsSimple(g geom.Geometry) (bool, error) {
	gh, destroy := h.createGeomHandle(g)
	defer destroy()
	return h.boolErr(C.GEOSisSimple_r(h.context, gh))
}

func (h *Handle) boolErr(c C.char) (bool, error) {
	switch c {
	case 0:
		return false, nil
	case 1:
		return true, nil
	case 2:
		return false, errors.New("an exception occurred")
	default:
		return false, fmt.Errorf("illegal result from libgeos: %v", c)
	}
}
