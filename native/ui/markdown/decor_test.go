package markdown

import (
	"image"
	"slices"
	"testing"

	"gioui.org/gpu/headless"
	"gioui.org/op/paint"
	"gioui.org/x/richtext"
)

// iconsOf is, for the first block of src, the mark each span wears: "file",
// "folder", "github" or "" for none.
func iconsOf(t *testing.T, src string) (marks []string, underlined []bool) {
	t.Helper()
	blocks := renderBlocks(markdownTheme(), src)
	if len(blocks) == 0 || blocks[0].prose == nil {
		t.Fatalf("no prose in %q", src)
	}
	c := blocks[0].prose
	for _, d := range c.decor {
		mark := ""
		switch d.icon {
		case icFile:
			mark = "file"
		case icFolder:
			mark = "folder"
		case icGitHub:
			mark = "github"
		}
		marks, underlined = append(marks, mark), append(underlined, d.underline)
	}
	return marks, underlined
}

func has(marks []string, want string) bool {
	return slices.Contains(marks, want)
}

func TestCodeThatSpellsAPathWearsAFileOrFolderMark(t *testing.T) {
	for src, want := range map[string]string{
		"See `src/main.go` now":         "file",
		"Edit `README.md`":              "file",
		"Look in `docs/`":               "folder",
		"Open `./cmd/lich-native`":      "file",
		"Run `total(a, b int)`":         "",
		"The `x` variable":              "",
		"Use `a b/c`":                   "",
		"Fetch `https://example.com/a`": "",
		"Call `foo.bar()`":              "",
		"Type `/compact` in the chat":   "",
		"Run `/model opus`":             "",
		"See `/usr/bin/lich`":           "file",
		"Read `/README.md`":             "file",
	} {
		marks, _ := iconsOf(t, src)
		got := ""
		for _, m := range marks {
			if m == "file" || m == "folder" {
				got = m
			}
		}
		if got != want {
			t.Errorf("%q: mark %q, want %q (%v)", src, got, want, marks)
		}
	}
}

func TestLinksAreUnderlinedAndGitHubOnesWearItsMark(t *testing.T) {
	marks, under := iconsOf(t, "Plain [docs](https://example.com/x) and [repo](https://github.com/lichdotdev/lich) and https://www.github.com/a/b")
	if has(marks, "github") != true {
		t.Fatalf("no GitHub mark: %v", marks)
	}
	githubs, underlined := 0, 0
	for i := range marks {
		if marks[i] == "github" {
			githubs++
		}
		if under[i] {
			underlined++
		}
	}
	if githubs != 2 {
		t.Errorf("%d GitHub marks, want two (the link and the bare address) and none on example.com: %v", githubs, marks)
	}
	if underlined < 3 {
		t.Errorf("%d underlined spans, want every link's", underlined)
	}
}

func TestTheMarkHasRoomBeforeItsText(t *testing.T) {
	blocks := renderBlocks(markdownTheme(), "[repo](https://github.com/a/b) and `x/y.go`")
	var spans []richtext.SpanStyle = blocks[0].prose.spans
	var link, code string
	for i, d := range blocks[0].prose.decor {
		if d.icon == icGitHub {
			link = spans[i].Content
		}
		if d.icon == icFile {
			code = spans[i].Content
		}
	}
	if link != linkIconRoom+"repo" || code != codeIconRoom+"x/y.go\u00a0" {
		t.Errorf("link %q, code %q: want the room before each", link, code)
	}
}

// A file in code wears its language's icon, as the Files tree draws it: a Go
// file paints color where a file of no known language stays gray.
func TestAFileInCodeWearsItsLanguagesIcon(t *testing.T) {
	saturated := func(img *image.RGBA) bool {
		for y := range img.Bounds().Dy() {
			for x := range img.Bounds().Dx() {
				p := img.RGBAAt(x, y)
				hi, lo := max(p.R, p.G, p.B), min(p.R, p.G, p.B)
				if int(hi)-int(lo) > 60 {
					return true
				}
			}
		}
		return false
	}
	gofile, _ := nestedPixels(t, "Open `main.go`")
	if !saturated(gofile) {
		t.Error("main.go drew no colored icon")
	}
	plain, _ := nestedPixels(t, "Open `notes.zzz`")
	if saturated(plain) {
		t.Error("a file of no known language drew a colored icon")
	}
}

// decorRows are the rows of what decorate draws for the first block of src,
// laid out width wide, with the path mark or without it.
func decorRows(t *testing.T, src string, width int, mark bool) *image.RGBA {
	t.Helper()
	size := image.Pt(width, 200)
	win, err := headless.NewWindow(size.X, size.Y)
	if err != nil {
		t.Skipf("no headless GPU: %v", err)
	}
	defer win.Release()
	th := markdownTheme()
	c := renderBlocks(th, src)[0].prose
	if !mark {
		for i := range c.decor {
			c.decor[i].icon = nil
		}
	}
	gtx, ops := markdownFrame(size.Y)
	gtx.Constraints.Max.X = width
	paint.Fill(ops, th.Background)
	c.decorate(gtx, th, nil)
	if err := win.Frame(ops); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rectangle{Max: size})
	if err := win.Screenshot(img); err != nil {
		t.Fatal(err)
	}
	return img
}

func TestAPathThatWrapsWearsItsMarkOnItsFirstLineOnly(t *testing.T) {
	const src = "See `scripts/seed-homologacao-with-a-long-name.sql` now"
	with, without := decorRows(t, src, 160, true), decorRows(t, src, 160, false)
	first, last := -1, -1
	for y := range with.Bounds().Dy() {
		for x := range with.Bounds().Dx() {
			if with.RGBAAt(x, y) != without.RGBAAt(x, y) {
				if first < 0 {
					first = y
				}
				last = y
			}
		}
	}
	if first < 0 {
		t.Fatal("no mark drawn")
	}
	if last-first > int(BodySize) {
		t.Errorf("the mark is drawn from row %d to %d, more than one line", first, last)
	}
}
