package gioterm

/*
#include <stdlib.h>
#include <ghostty/vt.h>

extern bool goDecodePng(GhosttyAllocator*, uint8_t*, size_t, GhosttySysImage*);

static bool on_decode_png(void* ud, const GhosttyAllocator* a, const uint8_t* data, size_t len, GhosttySysImage* out) {
	return goDecodePng((GhosttyAllocator*)a, (uint8_t*)data, len, out);
}

static GhosttyResult install_png_decoder(void) {
	return ghostty_sys_set(GHOSTTY_SYS_OPT_DECODE_PNG, (const void*)on_decode_png);
}

static GhosttyResult set_image_limit(GhosttyTerminal t, uint64_t limit) {
	return ghostty_terminal_set(t, GHOSTTY_TERMINAL_OPT_KITTY_IMAGE_STORAGE_LIMIT, &limit);
}

typedef struct {
	uint32_t image_id;
	int32_t z;
	int32_t col, row;
	uint32_t x_off, y_off;
	uint32_t w, h;
	uint32_t sx, sy, sw, sh;
} placed_t;

static GhosttyKittyGraphics graphics_of(GhosttyTerminal t) {
	GhosttyKittyGraphics g = NULL;
	if (ghostty_terminal_get(t, GHOSTTY_TERMINAL_DATA_KITTY_GRAPHICS, &g) != GHOSTTY_SUCCESS) return NULL;
	return g;
}

// Copies the placements on screen into out, at most max of them, and returns
// how many; -1 when the iterator cannot be read. A storage never written to
// returns 0 before touching the iterator, the cost every plain terminal pays.
static int collect_placements(GhosttyTerminal t, GhosttyKittyGraphicsPlacementIterator it, placed_t* out, int max) {
	GhosttyKittyGraphics g = graphics_of(t);
	if (g == NULL) return 0;
	uint64_t gen = 0;
	ghostty_kitty_graphics_get(g, GHOSTTY_KITTY_GRAPHICS_DATA_GENERATION, &gen);
	if (gen == 0) return 0;
	if (ghostty_kitty_graphics_get(g, GHOSTTY_KITTY_GRAPHICS_DATA_PLACEMENT_ITERATOR, &it) != GHOSTTY_SUCCESS) return -1;
	int n = 0;
	while (n < max && ghostty_kitty_graphics_placement_next(it)) {
		placed_t* p = &out[n];
		ghostty_kitty_graphics_placement_get(it, GHOSTTY_KITTY_GRAPHICS_PLACEMENT_DATA_IMAGE_ID, &p->image_id);
		GhosttyKittyGraphicsImage img = ghostty_kitty_graphics_image(g, p->image_id);
		if (img == NULL) continue;
		GhosttyKittyGraphicsPlacementRenderInfo ri = GHOSTTY_INIT_SIZED(GhosttyKittyGraphicsPlacementRenderInfo);
		if (ghostty_kitty_graphics_placement_render_info(it, img, t, &ri) != GHOSTTY_SUCCESS || !ri.viewport_visible) continue;
		p->z = 0;
		p->x_off = p->y_off = 0;
		ghostty_kitty_graphics_placement_get(it, GHOSTTY_KITTY_GRAPHICS_PLACEMENT_DATA_Z, &p->z);
		ghostty_kitty_graphics_placement_get(it, GHOSTTY_KITTY_GRAPHICS_PLACEMENT_DATA_X_OFFSET, &p->x_off);
		ghostty_kitty_graphics_placement_get(it, GHOSTTY_KITTY_GRAPHICS_PLACEMENT_DATA_Y_OFFSET, &p->y_off);
		p->col = ri.viewport_col;
		p->row = ri.viewport_row;
		p->w = ri.pixel_width;
		p->h = ri.pixel_height;
		p->sx = ri.source_x;
		p->sy = ri.source_y;
		p->sw = ri.source_width;
		p->sh = ri.source_height;
		n++;
	}
	return n;
}

typedef struct {
	uint32_t w, h;
	GhosttyKittyImageFormat format;
	const uint8_t* data;
	size_t len;
	uint64_t gen;
} image_t;

// False for an unknown image or one still arriving in chunks, which has no
// pixels yet.
static bool image_data(GhosttyTerminal t, uint32_t id, image_t* o) {
	GhosttyKittyGraphics g = graphics_of(t);
	if (g == NULL) return false;
	GhosttyKittyGraphicsImage img = ghostty_kitty_graphics_image(g, id);
	if (img == NULL) return false;
	o->data = NULL;
	if (ghostty_kitty_graphics_image_get(img, GHOSTTY_KITTY_IMAGE_DATA_WIDTH, &o->w) != GHOSTTY_SUCCESS) return false;
	if (ghostty_kitty_graphics_image_get(img, GHOSTTY_KITTY_IMAGE_DATA_HEIGHT, &o->h) != GHOSTTY_SUCCESS) return false;
	if (ghostty_kitty_graphics_image_get(img, GHOSTTY_KITTY_IMAGE_DATA_FORMAT, &o->format) != GHOSTTY_SUCCESS) return false;
	if (ghostty_kitty_graphics_image_get(img, GHOSTTY_KITTY_IMAGE_DATA_DATA_LEN, &o->len) != GHOSTTY_SUCCESS) return false;
	if (ghostty_kitty_graphics_image_get(img, GHOSTTY_KITTY_IMAGE_DATA_GENERATION, &o->gen) != GHOSTTY_SUCCESS) return false;
	if (ghostty_kitty_graphics_image_get(img, GHOSTTY_KITTY_IMAGE_DATA_DATA_PTR, &o->data) != GHOSTTY_SUCCESS) return false;
	return o->data != NULL;
}
*/
import "C"

import (
	"bytes"
	"cmp"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"slices"
	"sync"
	"unsafe"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// imageStorageLimit caps the decoded Kitty images one terminal holds. Kitty's
// own default is 320 MB a terminal; lich keeps many terminals open at once.
const imageStorageLimit = 64 << 20

// maxPlacements is how many image placements one frame draws.
const maxPlacements = 1024

// zBelowText is the Kitty z-index from which a placement draws over the text;
// anything lower draws under it.
const zBelowText = 0

// installPNGDecoder sets libghostty's process-wide PNG decoder, without which
// it rejects f=100, the format most programs send.
var installPNGDecoder = sync.OnceValue(func() C.GhosttyResult { return C.install_png_decoder() })

//export goDecodePng
func goDecodePng(alloc *C.GhosttyAllocator, data *C.uint8_t, n C.size_t, out *C.GhosttySysImage) C.bool {
	pix, w, h, ok := decodePNG(C.GoBytes(unsafe.Pointer(data), C.int(n)))
	if !ok {
		return false
	}
	buf := C.ghostty_alloc(alloc, C.size_t(len(pix)))
	if buf == nil {
		return false
	}
	copy(unsafe.Slice((*byte)(unsafe.Pointer(buf)), len(pix)), pix)
	out.width, out.height = C.uint32_t(w), C.uint32_t(h)
	out.data, out.data_len = buf, C.size_t(len(pix))
	return true
}

// decodePNG returns b's pixels as unpremultiplied RGBA. An image whose header
// claims more than the storage limit is refused before decoding: the program
// on the PTY chooses those bytes.
func decodePNG(b []byte) (pix []byte, w, h int, ok bool) {
	cfg, err := png.DecodeConfig(bytes.NewReader(b))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height*4 > imageStorageLimit {
		return nil, 0, 0, false
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, 0, 0, false
	}
	nrgba := image.NewNRGBA(image.Rect(0, 0, cfg.Width, cfg.Height))
	draw.Draw(nrgba, nrgba.Bounds(), img, img.Bounds().Min, draw.Src)
	return nrgba.Pix, cfg.Width, cfg.Height, true
}

// placedImage is one image placement in the grid's pixels.
type placedImage struct {
	op       paint.ImageOp
	dst, src image.Rectangle
	z        int32
}

type texture struct {
	gen  uint64
	op   paint.ImageOp
	used bool
}

// kittyImages is a terminal's Kitty graphics as the renderers draw them.
type kittyImages struct {
	it       C.GhosttyKittyGraphicsPlacementIterator
	raw      []C.placed_t
	textures map[uint32]*texture
	below    []placedImage
	above    []placedImage
}

func (k *kittyImages) init(t C.GhosttyTerminal) C.GhosttyResult {
	if r := installPNGDecoder(); r != C.GHOSTTY_SUCCESS {
		return r
	}
	if r := C.set_image_limit(t, imageStorageLimit); r != C.GHOSTTY_SUCCESS {
		return r
	}
	k.raw = make([]C.placed_t, maxPlacements)
	return C.ghostty_kitty_graphics_placement_iterator_new(nil, &k.it)
}

func (k *kittyImages) free() {
	C.ghostty_kitty_graphics_placement_iterator_free(k.it)
	k.textures = nil
}

// update reads the placements on screen and the pixels of any image that
// changed since the last frame. It runs with the terminal locked: what
// libghostty lends is only valid until the next write.
func (k *kittyImages) update(t C.GhosttyTerminal, cellW, cellH int) error {
	k.below, k.above = k.below[:0], k.above[:0]
	n := int(C.collect_placements(t, k.it, &k.raw[0], C.int(len(k.raw))))
	if n < 0 {
		return fmt.Errorf("kitty graphics: placement iterator")
	}
	for _, tx := range k.textures {
		tx.used = false
	}
	for _, p := range k.raw[:n] {
		tx := k.texture(t, uint32(p.image_id))
		if tx == nil {
			continue
		}
		tx.used = true
		x := int(p.col)*cellW + int(p.x_off)
		y := int(p.row)*cellH + int(p.y_off)
		pi := placedImage{
			op:  tx.op,
			dst: image.Rect(x, y, x+int(p.w), y+int(p.h)),
			src: image.Rect(int(p.sx), int(p.sy), int(p.sx+p.sw), int(p.sy+p.sh)),
			z:   int32(p.z),
		}
		if pi.z < zBelowText {
			k.below = append(k.below, pi)
		} else {
			k.above = append(k.above, pi)
		}
	}
	for id, tx := range k.textures {
		if !tx.used {
			delete(k.textures, id)
		}
	}
	byZ := func(a, b placedImage) int { return cmp.Compare(a.z, b.z) }
	slices.SortStableFunc(k.below, byZ)
	slices.SortStableFunc(k.above, byZ)
	return nil
}

// texture is image id ready to draw, made again only when libghostty's
// generation for it moves (a retransmission under the same id).
func (k *kittyImages) texture(t C.GhosttyTerminal, id uint32) *texture {
	var d C.image_t
	if !C.image_data(t, C.uint32_t(id), &d) {
		return nil
	}
	if tx := k.textures[id]; tx != nil && tx.gen == uint64(d.gen) {
		return tx
	}
	img := toNRGBA(int(d.w), int(d.h), int(d.format), unsafe.Slice((*byte)(unsafe.Pointer(d.data)), int(d.len)))
	if img == nil {
		return nil
	}
	if k.textures == nil {
		k.textures = map[uint32]*texture{}
	}
	tx := &texture{gen: uint64(d.gen), op: paint.NewImageOp(img)}
	k.textures[id] = tx
	return tx
}

// toNRGBA copies libghostty's pixels, which it frees on its next write, into
// an image in any of the formats it stores; nil when the length does not fit
// the size.
func toNRGBA(w, h, format int, pix []byte) *image.NRGBA {
	bpp := map[int]int{
		C.GHOSTTY_KITTY_IMAGE_FORMAT_RGBA:       4,
		C.GHOSTTY_KITTY_IMAGE_FORMAT_RGB:        3,
		C.GHOSTTY_KITTY_IMAGE_FORMAT_GRAY_ALPHA: 2,
		C.GHOSTTY_KITTY_IMAGE_FORMAT_GRAY:       1,
	}[format]
	if bpp == 0 || w <= 0 || h <= 0 || len(pix) != w*h*bpp {
		return nil
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := range w * h {
		s, d := pix[i*bpp:(i+1)*bpp], img.Pix[i*4:i*4+4]
		switch bpp {
		case 4:
			copy(d, s)
		case 3:
			d[0], d[1], d[2], d[3] = s[0], s[1], s[2], 255
		case 2:
			d[0], d[1], d[2], d[3] = s[0], s[0], s[0], s[1]
		case 1:
			d[0], d[1], d[2], d[3] = s[0], s[0], s[0], 255
		}
	}
	return img
}

// paintImages draws each placement's source rectangle scaled into its place
// on the grid.
func paintImages(ops *op.Ops, images []placedImage) {
	for _, im := range images {
		if im.src.Empty() || im.dst.Empty() {
			continue
		}
		cl := clip.Rect(im.dst).Push(ops)
		scale := f32.Pt(float32(im.dst.Dx())/float32(im.src.Dx()), float32(im.dst.Dy())/float32(im.src.Dy()))
		aff := f32.Affine2D{}.
			Offset(f32.Pt(-float32(im.src.Min.X), -float32(im.src.Min.Y))).
			Scale(f32.Point{}, scale).
			Offset(f32.Pt(float32(im.dst.Min.X), float32(im.dst.Min.Y)))
		t := op.Affine(aff).Push(ops)
		im.op.Add(ops)
		paint.PaintOp{}.Add(ops)
		t.Pop()
		cl.Pop()
	}
}
