package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestPickImageType(t *testing.T) {
	for _, c := range []struct {
		name  string
		types []string
		want  string
	}{
		{"a screenshot", []string{"TARGETS", "image/png"}, "image/png"},
		{"a browser's copied image", []string{"text/html", "image/png"}, "image/png"},
		{"png wins over jpeg in Claude Code's order", []string{"image/jpeg", "image/png"}, "image/png"},
		{"jpeg alone", []string{"image/jpeg"}, "image/jpeg"},
		{"text", []string{"TARGETS", "UTF8_STRING"}, ""},
		{"text beside an image pastes the text", []string{"text/plain", "image/png"}, ""},
		{"an empty clipboard", nil, ""},
		{"no type read", []string{"TARGETS", "application/x-foo"}, ""},
	} {
		if got := pickImageType(c.types); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

// fakeClip answers each argv from outs by its joined words, recording the
// calls; an argv it does not know fails as a missing binary does when its
// tool is in missing, else as a non-zero exit.
type fakeClip struct {
	outs    map[string]string
	missing []string
	calls   []string
}

func (f *fakeClip) run(_ context.Context, _ time.Duration, argv []string) ([]byte, error) {
	line := strings.Join(argv, " ")
	f.calls = append(f.calls, line)
	if slices.Contains(f.missing, argv[0]) {
		return nil, &exec.Error{Name: argv[0], Err: exec.ErrNotFound}
	}
	out, ok := f.outs[line]
	if !ok {
		return nil, fmt.Errorf("%s: exit status 1", argv[0])
	}
	return []byte(out), nil
}

func TestReadClipImageReadsTheFirstToolThatLists(t *testing.T) {
	f := &fakeClip{outs: map[string]string{
		"xclip -selection clipboard -o -t TARGETS":   "TARGETS\nimage/png\n",
		"xclip -selection clipboard -o -t image/png": "\x89PNG",
	}}
	// Wayland first, as the session's display: wl-paste fails to connect and
	// hands over to xclip.
	img, err := readClipImage(t.Context(), f.run, unixClipTools(true))
	if err != nil || string(img.data) != "\x89PNG" || img.ext != ".png" {
		t.Fatalf("read %q %q, %v", img.data, img.ext, err)
	}
	if want := []string{"wl-paste --list-types", "xclip -selection clipboard -o -t TARGETS", "xclip -selection clipboard -o -t image/png"}; !slices.Equal(f.calls, want) {
		t.Errorf("ran %q, want %q", f.calls, want)
	}
}

func TestReadClipImageOverTextReadsNothing(t *testing.T) {
	f := &fakeClip{outs: map[string]string{"wl-paste --list-types": "text/plain;charset=utf-8\ntext/plain\nimage/png\n"}}
	img, err := readClipImage(t.Context(), f.run, unixClipTools(true))
	if err != nil || img.data != nil {
		t.Fatalf("read %q, %v", img.data, err)
	}
	if len(f.calls) != 1 {
		t.Errorf("ran %q after the listing said text", f.calls)
	}
}

func TestReadClipImageWithoutToolsSaysSo(t *testing.T) {
	f := &fakeClip{missing: []string{"wl-paste", "xclip"}}
	if _, err := readClipImage(t.Context(), f.run, unixClipTools(false)); !errors.Is(err, errNoClipboardTool) {
		t.Errorf("no tool installed answered %v", err)
	}
	// One tool there, failing on an empty clipboard, is no missing tool.
	f = &fakeClip{missing: []string{"xclip"}}
	if img, err := readClipImage(t.Context(), f.run, unixClipTools(true)); err != nil || img.data != nil {
		t.Errorf("an empty clipboard answered %q, %v", img.data, err)
	}
}

func TestReadClipImageReportsAFailedRead(t *testing.T) {
	f := &fakeClip{outs: map[string]string{"xclip -selection clipboard -o -t TARGETS": "image/gif"}}
	if _, err := readClipImage(t.Context(), f.run, unixClipTools(false)); err == nil {
		t.Error("a listed image that would not read answered no error")
	}
}

func TestParseMacPNG(t *testing.T) {
	img, err := parseMacPNG([]byte("«data PNGf89504E47»\n"))
	if err != nil || string(img.data) != "\x89PNG" || img.ext != ".png" {
		t.Errorf("parsed %q %q, %v", img.data, img.ext, err)
	}
	if img, err := parseMacPNG([]byte("\n")); err != nil || img.data != nil {
		t.Errorf("an empty answer parsed %q, %v", img.data, err)
	}
	for _, bad := range []string{"hello", "«data PNGfZZ»", "«data PNGf8950"} {
		if _, err := parseMacPNG([]byte(bad)); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
}
