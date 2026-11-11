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
	"strconv"
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
// From to To inclusive, which may run over soft-wrapped rows. URL is empty on a
// link to code, which Code names instead.
type Link struct {
	URL      string
	Code     *CodeRef
	From, To image.Point
}

// CodeRef is a file and line as a program printed them. Package is the
// directory a stack frame's package names, "" when the line holds none.
type CodeRef struct {
	Path    string
	Line    int
	Package string
}

// LinkAt is the link under pixel (x, y): the cell's OSC 8 hyperlink, else a
// URL or a path:line printed in the line there, soft-wrapped rows joined; ok
// false when there is none.
func (v *Terminal) LinkAt(x, y float32) (l Link, ok bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.closed {
		return Link{}, false
	}
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
	// cellOf is the line index of the cell each rune of line came from: a
	// grapheme of several codepoints is one cell.
	var line []rune
	var cellOf []int
	at := 0
	for r := first; r <= last; r++ {
		for c := 0; c < v.cols; c++ {
			i := (r-first)*v.cols + c
			if r == row && c == col {
				at = len(line)
			}
			runes := len(line)
			line = v.appendCellRunes(line, c, r)
			for range len(line) - runes {
				cellOf = append(cellOf, i)
			}
		}
	}
	return v.printedLink(line, cellOf, at, first)
}

// printedLink is the URL or path:line in line covering index at, placed back
// on the grid through cellOf; first is the row line starts on.
func (v *Terminal) printedLink(line []rune, cellOf []int, at, first int) (Link, bool) {
	cell := func(i int) image.Point { return image.Pt(i%v.cols, first+i/v.cols) }
	if start, end, ok := urlAround(line, at); ok {
		return Link{URL: string(line[start:end]), From: cell(cellOf[start]), To: cell(cellOf[end-1])}, true
	}
	if start, end, ref, ok := codeRefAround(line, at); ok {
		return Link{Code: &ref, From: cell(cellOf[start]), To: cell(cellOf[end-1])}, true
	}
	return Link{}, false
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

// appendCellRunes adds the cell's grapheme to line, a space for an empty cell,
// so columns keep their place in it.
func (v *Terminal) appendCellRunes(line []rune, col, row int) []rune {
	var local [linkGraphemeMax]C.uint32_t
	buf := local[:]
	var n C.size_t
	r := C.cell_text(v.t, C.uint16_t(col), C.uint32_t(row), &buf[0], C.size_t(len(buf)), &n)
	if r == C.GHOSTTY_OUT_OF_SPACE {
		buf = make([]C.uint32_t, int(n))
		r = C.cell_text(v.t, C.uint16_t(col), C.uint32_t(row), &buf[0], C.size_t(len(buf)), &n)
	}
	if r != C.GHOSTTY_SUCCESS || n == 0 {
		return append(line, ' ')
	}
	for _, cp := range buf[:int(n)] {
		line = append(line, rune(cp))
	}
	return line
}

// linkPattern is a URL as text shows one.
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

// codeRefPattern is a path:line, with an optional :column. The name has to end
// in an extension starting with a letter or follow a slash, which keeps
// localhost:3000, 12:30 and 127.0.0.1:80 out.
var codeRefPattern = regexp.MustCompile(`(?:[A-Za-z]:)?[\p{L}\p{N}_.@+~/\\-]*(?:\.\p{L}[\p{L}\p{N}]*|[/\\][\p{L}\p{N}_.@+~-]+):(\d+)(?::\d+)?`)

// stackFrame is the method a Java or Kotlin frame names just before its
// (File.java:12).
var stackFrame = regexp.MustCompile(`\bat\s+([\p{L}\p{N}_$.]+)\($`)

// codeRefAround is the rune span [start, end) of the path:line in line
// covering index at, and what it names; ok false when none does.
func codeRefAround(line []rune, at int) (start, end int, ref CodeRef, ok bool) {
	text := string(line)
	offset := len(string(line[:at]))
	for _, m := range codeRefPattern.FindAllStringSubmatchIndex(text, -1) {
		if offset < m[0] || offset >= m[1] {
			continue
		}
		n, err := strconv.Atoi(text[m[2]:m[3]])
		if err != nil || n == 0 {
			return 0, 0, CodeRef{}, false
		}
		ref = CodeRef{Path: text[m[0] : m[2]-1], Line: n, Package: framePackage(text[:m[0]])}
		start = utf8.RuneCountInString(text[:m[0]])
		return start, start + utf8.RuneCountInString(text[m[0]:m[1]]), ref, true
	}
	return 0, 0, CodeRef{}, false
}

// framePackage is the directory of the frame ending before: com.acme.Foo.run
// is class Foo's method in com/acme.
func framePackage(before string) string {
	m := stackFrame.FindStringSubmatch(before)
	if m == nil {
		return ""
	}
	parts := strings.Split(m[1], ".")
	if len(parts) < 3 {
		return ""
	}
	return strings.Join(parts[:len(parts)-2], "/")
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
