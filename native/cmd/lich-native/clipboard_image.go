package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"
)

// The Gio fork reads the clipboard as text alone, so an image is read with
// the OS's own tool (clipboard_image_<os>.go), the way Claude Code 2.1.330
// reads one at its prompt: its commands, its type order and its timeouts.

// clipImage is an image read off the clipboard and the extension its type
// takes; no data is a clipboard with no image, or with text.
type clipImage struct {
	data []byte
	ext  string
}

// errNoClipboardTool is a machine with none of the tools that read an image
// off its clipboard.
var errNoClipboardTool = errors.New("no clipboard tool")

const (
	clipListTimeout = 3 * time.Second
	clipReadTimeout = 30 * time.Second
)

// clipImageTypes are the image types read, in Claude Code's order.
var clipImageTypes = []string{"image/png", "image/jpeg", "image/jpg", "image/webp", "image/gif", "image/bmp"}

// clipTextTypes are the types the Gio fork pastes as text
// (app/os_wayland.go clipboardMimeTypes; X11 asks UTF8_STRING). A clipboard
// offering one is text, which the editor pastes, so it makes no card: Claude
// Code takes the image over the text, but its text paste is another key.
var clipTextTypes = []string{"text/plain;charset=utf8", "UTF8_STRING", "text/plain", "TEXT", "STRING"}

// pickImageType is the type an image is read as from a clipboard offering
// types, "" for one with text or no image.
func pickImageType(types []string) string {
	if slices.ContainsFunc(types, func(t string) bool { return slices.Contains(clipTextTypes, t) }) {
		return ""
	}
	for _, want := range clipImageTypes {
		if slices.Contains(types, want) {
			return want
		}
	}
	return ""
}

// clipTool lists a clipboard's types with list, and reads one with read and
// the type after it.
type clipTool struct {
	list, read []string
}

var (
	x11Clip     = clipTool{list: []string{"xclip", "-selection", "clipboard", "-o", "-t", "TARGETS"}, read: []string{"xclip", "-selection", "clipboard", "-o", "-t"}}
	waylandClip = clipTool{list: []string{"wl-paste", "--list-types"}, read: []string{"wl-paste", "--type"}}
)

// unixClipTools are the tools tried in turn, the session's own display's
// first: under Wayland xclip reaches only XWayland's clipboard.
func unixClipTools(wayland bool) []clipTool {
	if wayland {
		return []clipTool{waylandClip, x11Clip}
	}
	return []clipTool{x11Clip, waylandClip}
}

// runClip runs argv within timeout and answers its stdout.
type runClip func(ctx context.Context, timeout time.Duration, argv []string) ([]byte, error)

func execClip(ctx context.Context, timeout time.Duration, argv []string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return exec.CommandContext(ctx, argv[0], argv[1:]...).Output()
}

// readClipImage reads the image off the clipboard with the first of tools
// that lists its types. A tool that fails to list (no display, an empty
// clipboard) hands over to the next; errNoClipboardTool when none is
// installed.
func readClipImage(ctx context.Context, run runClip, tools []clipTool) (clipImage, error) {
	missing := 0
	for _, tool := range tools {
		out, err := run(ctx, clipListTimeout, tool.list)
		if errors.Is(err, exec.ErrNotFound) {
			missing++
			continue
		}
		if err != nil {
			continue
		}
		mime := pickImageType(strings.Fields(string(out)))
		if mime == "" {
			return clipImage{}, nil
		}
		data, err := run(ctx, clipReadTimeout, append(slices.Clone(tool.read), mime))
		if err != nil {
			return clipImage{}, fmt.Errorf("%s %s: %w", tool.read[0], mime, err)
		}
		return clipImage{data: data, ext: "." + strings.TrimPrefix(mime, "image/")}, nil
	}
	if missing == len(tools) {
		return clipImage{}, errNoClipboardTool
	}
	return clipImage{}, nil
}

// macPNGPrefix and macPNGSuffix wrap the hex osascript prints a «class PNGf»
// as.
const (
	macPNGPrefix = "«data PNGf"
	macPNGSuffix = "»"
)

// parseMacPNG reads the PNG out of osascript's answer, nothing for an empty
// one: the script answers "" over text.
func parseMacPNG(out []byte) (clipImage, error) {
	s := strings.TrimSpace(string(out))
	if s == "" {
		return clipImage{}, nil
	}
	h, prefixed := strings.CutPrefix(s, macPNGPrefix)
	h, suffixed := strings.CutSuffix(h, macPNGSuffix)
	if !prefixed || !suffixed {
		return clipImage{}, fmt.Errorf("osascript answered %.40q", s)
	}
	data, err := hex.DecodeString(h)
	if err != nil {
		return clipImage{}, fmt.Errorf("osascript PNG: %w", err)
	}
	return clipImage{data: data, ext: ".png"}, nil
}
