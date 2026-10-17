package glyphs

import (
	"bytes"
	"fmt"
	"os"
	"sync"

	gotext "github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
)

// fontFile is a font file read and parsed once per process and shared by every
// Rasterizer: each terminal and diff view has its own, and a fallback such as
// a CJK collection or a color emoji font is tens of megabytes.
type fontFile struct {
	// sf is the file's first face; sfnt keeps reading from data.
	sf   *sfnt.Font
	data []byte
	// color is the first face as go-text reads it, for the color bitmaps
	// sfnt cannot (CBDT, sbix); nil when the file has none.
	color *gotext.Font
}

var fontFiles = struct {
	sync.Mutex
	byPath map[string]*fontFile
}{byPath: map[string]*fontFile{}}

// colorTables are the tables that hold PNG glyphs.
var colorTables = [...]ot.Tag{ot.MustNewTag("CBDT"), ot.MustNewTag("sbix")}

func openFontFile(path string) (*fontFile, error) {
	fontFiles.Lock()
	defer fontFiles.Unlock()
	if f, ok := fontFiles.byPath[path]; ok {
		return f, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	coll, err := opentype.ParseCollection(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	sf, err := coll.Font(0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	color, err := parseColor(data)
	if err != nil {
		return nil, fmt.Errorf("parse color bitmaps of %s: %w", path, err)
	}
	f := &fontFile{sf: sf, data: data, color: color}
	fontFiles.byPath[path] = f
	return f, nil
}

// parseColor parses the first face of data when it carries PNG glyphs, and
// only that face: reading the headers of a collection costs nothing, parsing
// all of its faces is what a CJK collection makes expensive.
func parseColor(data []byte) (*gotext.Font, error) {
	loaders, err := ot.NewLoaders(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	for _, tag := range colorTables {
		if loaders[0].HasTable(tag) {
			return gotext.NewFont(loaders[0])
		}
	}
	return nil, nil
}
