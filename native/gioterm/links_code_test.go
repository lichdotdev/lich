package gioterm

import (
	"image"
	"testing"
)

func TestCodeRefAroundFindsThePathUnderTheCell(t *testing.T) {
	cases := []struct {
		name, line, under string
		want              CodeRef
		ok                bool
	}{
		{"relative path", "see src/a.go:12 now", "a.go", CodeRef{Path: "src/a.go", Line: 12}, true},
		{"column", "src/a.go:12:7: unused", "a.go", CodeRef{Path: "src/a.go", Line: 12}, true},
		{"absolute", "/home/me/a.go:3", "me", CodeRef{Path: "/home/me/a.go", Line: 3}, true},
		{"no extension after a slash", "src/Makefile:9", "Make", CodeRef{Path: "src/Makefile", Line: 9}, true},
		{"windows", `C:\work\a.cs:4`, "work", CodeRef{Path: `C:\work\a.cs`, Line: 4}, true},
		{"accented", "src/ação.go:2", "ação", CodeRef{Path: "src/ação.go", Line: 2}, true},
		{"java frame", "    at com.acme.OrderService.process(OrderService.java:142)", "Service.java", CodeRef{Path: "OrderService.java", Line: 142, Package: "com/acme"}, true},
		{"kotlin frame with a nested class", "at a.b.C$D.run(C.kt:1)", "C.kt", CodeRef{Path: "C.kt", Line: 1, Package: "a/b"}, true},
		{"frame with no package", "at Main.run(Main.java:1)", "Main.java", CodeRef{Path: "Main.java", Line: 1}, true},
		{"host and port", "localhost:3000", "host", CodeRef{}, false},
		{"time", "at 12:30", "12", CodeRef{}, false},
		{"ip and port", "127.0.0.1:8080", "0.0", CodeRef{}, false},
		{"version and port", "v1.2:80", "1.2", CodeRef{}, false},
		{"line zero", "a.go:0", "a.go", CodeRef{}, false},
		{"no line", "see a.go now", "a.go", CodeRef{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			line := []rune(c.line)
			at := runeIndex(t, line, c.under)
			_, _, got, ok := codeRefAround(line, at)
			if ok != c.ok || got != c.want {
				t.Fatalf("got %+v, %v; want %+v, %v", got, ok, c.want, c.ok)
			}
		})
	}
}

func runeIndex(t *testing.T, line []rune, under string) int {
	t.Helper()
	sub := []rune(under)
	for i := 0; i+len(sub) <= len(line); i++ {
		if string(line[i:i+len(sub)]) == under {
			return i
		}
	}
	t.Fatalf("%q is not in %q", under, string(line))
	return 0
}

func TestCodeRefAroundOnlyAnswersInsideThePath(t *testing.T) {
	line := []rune("see a.go:12 now")
	if _, _, _, ok := codeRefAround(line, 1); ok {
		t.Fatal("a cell before the path found it")
	}
	start, end, _, ok := codeRefAround(line, 4)
	if !ok || start != 4 || end != 11 {
		t.Fatalf("span %d..%d, %v; want 4..11", start, end, ok)
	}
}

func TestLinkAtReadsAPathWrappedOverRows(t *testing.T) {
	v, err := New(20, 4, 10, 20, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	v.Write([]byte("in src/main/java/OrderService.java:142"))
	got, ok := v.LinkAt(2*10+5, 20+5)
	if !ok || got.Code == nil || got.URL != "" {
		t.Fatalf("LinkAt = %+v, %v", got, ok)
	}
	if got.Code.Path != "src/main/java/OrderService.java" || got.Code.Line != 142 {
		t.Errorf("code = %+v", *got.Code)
	}
	if got.From != image.Pt(3, 0) || got.To != image.Pt(17, 1) {
		t.Errorf("span %v..%v", got.From, got.To)
	}
}

func TestLinkAtPrefersTheURLAroundAPath(t *testing.T) {
	v, err := New(60, 4, 10, 20, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	v.Write([]byte("https://example.com/a.go:12"))
	got, ok := v.LinkAt(22*10+5, 5)
	if !ok || got.Code != nil || got.URL != "https://example.com/a.go:12" {
		t.Fatalf("LinkAt = %+v, %v", got, ok)
	}
}
