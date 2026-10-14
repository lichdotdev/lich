package gioterm

/*
#include <ghostty/vt.h>

static bool link_ref(GhosttyTerminal t, uint16_t x, uint32_t y, GhosttyGridRef* ref) {
	GhosttyPoint p = {0};
	p.tag = GHOSTTY_POINT_TAG_VIEWPORT;
	p.value.coordinate.x = x;
	p.value.coordinate.y = y;
	*ref = (GhosttyGridRef)GHOSTTY_INIT_SIZED(GhosttyGridRef);
	return ghostty_terminal_grid_ref(t, p, ref) == GHOSTTY_SUCCESS;
}

static GhosttyResult cell_uri(GhosttyTerminal t, uint16_t x, uint32_t y, uint8_t* buf, size_t cap, size_t* n) {
	GhosttyGridRef ref;
	*n = 0;
	if (!link_ref(t, x, y, &ref)) return GHOSTTY_INVALID_VALUE;
	return ghostty_grid_ref_hyperlink_uri(&ref, buf, cap, n);
}

static GhosttyResult cell_text(GhosttyTerminal t, uint16_t x, uint32_t y, uint32_t* buf, size_t cap, size_t* n) {
	GhosttyGridRef ref;
	*n = 0;
	if (!link_ref(t, x, y, &ref)) return GHOSTTY_INVALID_VALUE;
	return ghostty_grid_ref_graphemes(&ref, buf, cap, n);
}

static bool row_wraps(GhosttyTerminal t, uint32_t y) {
	GhosttyGridRef ref;
	GhosttyRow row;
	bool wrap = false;
	if (!link_ref(t, 0, y, &ref) || ghostty_grid_ref_row(&ref, &row) != GHOSTTY_SUCCESS) return false;
	ghostty_row_get(row, GHOSTTY_ROW_DATA_WRAP, &wrap);
	return wrap;
}
*/
import "C"

import (
	"image"
	"regexp"
	"strings"
	"unicode/utf8"
	"unsafe"
)

// linkURIMax bounds an OSC 8 URI read off a cell, in bytes: a longer one is
// read again at the size the terminal reports.
const linkURIMax = 2048

// linkGraphemeMax is the codepoints one cell holds before a retry: a base
// and a few combining marks.
const linkGraphemeMax = 8

// Link is a link on the grid: where it goes and the viewport cells it covers,
// From to To inclusive, which may run over soft-wrapped rows.
type Link struct {
	URL      string
	From, To image.Point
}

// LinkAt is the link under pixel (x, y): the cell's OSC 8 hyperlink, else a
// URL printed in the line there, soft-wrapped rows joined; ok false when
// there is none.
func (v *Terminal) LinkAt(x, y float32) (l Link, ok bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	col, row := v.cellAt(x, y)
	if uri := v.hyperlink(col, row); uri != "" {
		from, to := col, col
		for from > 0 && v.hyperlink(from-1, row) == uri {
			from--
		}
		for to < v.cols-1 && v.hyperlink(to+1, row) == uri {
			to++
		}
		return Link{URL: uri, From: image.Pt(from, row), To: image.Pt(to, row)}, true
	}
	first, last := row, row
	for first > 0 && bool(C.row_wraps(v.t, C.uint32_t(first-1))) {
		first--
	}
	for last < v.rows-1 && bool(C.row_wraps(v.t, C.uint32_t(last))) {
		last++
	}
	var line []rune
	for r := first; r <= last; r++ {
		for c := 0; c < v.cols; c++ {
			line = append(line, v.cellRune(c, r))
		}
	}
	at := (row-first)*v.cols + col
	start, end, ok := urlAround(line, at)
	if !ok {
		return Link{}, false
	}
	cell := func(i int) image.Point { return image.Pt(i%v.cols, first+i/v.cols) }
	return Link{URL: string(line[start:end]), From: cell(start), To: cell(end - 1)}, true
}

func (v *Terminal) hyperlink(col, row int) string {
	buf := make([]byte, linkURIMax)
	var n C.size_t
	r := C.cell_uri(v.t, C.uint16_t(col), C.uint32_t(row), (*C.uint8_t)(unsafe.Pointer(&buf[0])), C.size_t(len(buf)), &n)
	if r == C.GHOSTTY_OUT_OF_SPACE {
		buf = make([]byte, int(n))
		r = C.cell_uri(v.t, C.uint16_t(col), C.uint32_t(row), (*C.uint8_t)(unsafe.Pointer(&buf[0])), C.size_t(len(buf)), &n)
	}
	if r != C.GHOSTTY_SUCCESS {
		return ""
	}
	return string(buf[:int(n)])
}

// cellRune is the cell's first codepoint, a space for an empty cell, so
// columns keep their place in the line.
func (v *Terminal) cellRune(col, row int) rune {
	var buf [linkGraphemeMax]C.uint32_t
	var n C.size_t
	r := C.cell_text(v.t, C.uint16_t(col), C.uint32_t(row), &buf[0], C.size_t(len(buf)), &n)
	if (r != C.GHOSTTY_SUCCESS && r != C.GHOSTTY_OUT_OF_SPACE) || n == 0 {
		return ' '
	}
	return rune(buf[0])
}

// linkPattern is a URL as text shows one, the shape xterm's web-links addon
// finds in the web window.
var linkPattern = regexp.MustCompile(`(?i)\bhttps?://[^\s<>"'` + "`" + `]+`)

// linkTrailing is punctuation that ends a sentence around a URL rather than
// the URL itself.
const linkTrailing = `.,;:!?'"`

// urlAround is the rune span [start, end) of the URL in line covering index
// at; ok false when none does.
func urlAround(line []rune, at int) (start, end int, ok bool) {
	text := string(line)
	offset := len(string(line[:at]))
	for _, m := range linkPattern.FindAllStringIndex(text, -1) {
		url := trimURL(text[m[0]:m[1]])
		if offset >= m[0] && offset < m[0]+len(url) {
			start = utf8.RuneCountInString(text[:m[0]])
			return start, start + utf8.RuneCountInString(url), true
		}
	}
	return 0, 0, false
}

// trimURL drops trailing punctuation and closing brackets the URL never
// opened: "(see https://x.dev/a)." keeps https://x.dev/a.
func trimURL(url string) string {
	for url != "" {
		last := url[len(url)-1]
		switch {
		case strings.IndexByte(linkTrailing, last) >= 0:
			url = url[:len(url)-1]
		case last == ')' && strings.Count(url, "(") < strings.Count(url, ")"),
			last == ']' && strings.Count(url, "[") < strings.Count(url, "]"),
			last == '}' && strings.Count(url, "{") < strings.Count(url, "}"):
			url = url[:len(url)-1]
		default:
			return url
		}
	}
	return url
}
