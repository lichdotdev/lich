// Package turbojpeg decodes a JPEG with libjpeg-turbo, straight into the RGBA
// the window draws: image/jpeg and the copy to RGBA after it took 31 ms on a
// 1920x1080 screencast frame against 9 ms here (measured 2026-11-11). It
// encodes lich-browser's frames too: 2 ms for a 1280x720 page where
// image/jpeg took 15 (measured 2026-11-26).
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

// EncodeBGRX encodes the pixels of a cairo image surface, B, G, R and an
// unused byte each in memory (ARGB32 or RGB24 on a little-endian machine),
// at quality 1 to 100.
func EncodeBGRX(pixels []byte, width, height, stride, quality int) ([]byte, error) {
	if len(pixels) < stride*height || width <= 0 || height <= 0 {
		return nil, errEmpty
	}
	h := C.tjInitCompress()
	if h == nil {
		return nil, errors.New("turbojpeg: " + C.GoString(C.tjGetErrorStr2(nil)))
	}
	defer C.tjDestroy(h)
	var out *C.uchar
	var size C.ulong
	src := (*C.uchar)(unsafe.Pointer(&pixels[0]))
	if C.tjCompress2(h, src, C.int(width), C.int(stride), C.int(height), C.TJPF_BGRX, &out, &size, C.TJSAMP_420, C.int(quality), 0) != 0 {
		return nil, lastError(h)
	}
	defer C.tjFree(out)
	return C.GoBytes(unsafe.Pointer(out), C.int(size)), nil
}

func lastError(h C.tjhandle) error {
	return fmt.Errorf("turbojpeg: %s", C.GoString(C.tjGetErrorStr2(h)))
}
