package main

import "image"

// fromARGB32 is a cairo ARGB32 image (premultiplied, native-endian words, so
// B, G, R, A in memory on the little-endian machines the window runs on) as
// an image.RGBA, which is premultiplied too.
func fromARGB32(pixels []byte, width, height, stride int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		row := pixels[y*stride : y*stride+width*4]
		out := img.Pix[y*img.Stride : y*img.Stride+width*4]
		for x := 0; x < width*4; x += 4 {
			out[x], out[x+1], out[x+2], out[x+3] = row[x+2], row[x+1], row[x], row[x+3]
		}
	}
	return img
}
