// Package turbojpeg decodes a JPEG with libjpeg-turbo, straight into the RGBA
// the window draws: image/jpeg and the copy to RGBA after it took 31 ms on a
// 1920x1080 screencast frame against 9 ms here (measured 2026-11-11).
package turbojpeg

/*
#cgo pkg-config: libturbojpeg
#include <turbojpeg.h>
*/
import "C"

import (
	"errors"
	"fmt"
	"image"
	"unsafe"
)

var errEmpty = errors.New("turbojpeg: no data")

// Decode decodes a JPEG. It uses the TurboJPEG 2 calls, which version 3 keeps:
// a distribution's own libjpeg-turbo builds the window too.
func Decode(data []byte) (*image.RGBA, error) {
	if len(data) == 0 {
		return nil, errEmpty
	}
	h := C.tjInitDecompress()
	if h == nil {
		return nil, errors.New("turbojpeg: " + C.GoString(C.tjGetErrorStr2(nil)))
	}
	defer C.tjDestroy(h)
	src, size := (*C.uchar)(unsafe.Pointer(&data[0])), C.ulong(len(data))
	var width, height, subsampling, colorspace C.int
	if C.tjDecompressHeader3(h, src, size, &width, &height, &subsampling, &colorspace) != 0 {
		return nil, lastError(h)
	}
	img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	dst := (*C.uchar)(unsafe.Pointer(&img.Pix[0]))
	if C.tjDecompress2(h, src, size, dst, width, C.int(img.Stride), height, C.TJPF_RGBA, 0) != 0 {
		return nil, lastError(h)
	}
	return img, nil
}

func lastError(h C.tjhandle) error {
	return fmt.Errorf("turbojpeg: %s", C.GoString(C.tjGetErrorStr2(h)))
}
