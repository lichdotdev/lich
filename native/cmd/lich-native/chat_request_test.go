package main

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

// fixtureRequest is a request case of the contract's fixture, its sketch, if
// any, pointed at image.
func fixtureRequest(t *testing.T, name, image string) string {
	t.Helper()
	raw, err := os.ReadFile("../../../docs/hooks/fixtures/notification.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct{ Name, Envelope string }
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, c := range fixture.Cases {
		if c.Name == name {
			return strings.ReplaceAll(c.Envelope, "/home/me/.config/lich/drops/sketch.png", image)
		}
	}
	t.Fatalf("no fixture case %q", name)
	return ""
}

// sketchFile is a small picture standing in for a sketch.
func sketchFile(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 160, 100))
	for x := 20; x < 140; x++ {
		img.Set(x, 50, color.RGBA{R: 230, G: 80, B: 80, A: 255})
	}
	path := filepath.Join(t.TempDir(), "sketch.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return path
}

func requestItems(t *testing.T, names ...string) []backend.ChatItem {
	t.Helper()
	sketch := sketchFile(t)
	var items []backend.ChatItem
	for i, name := range names {
		turn := i + 1
		items = append(items,
			backend.ChatItem{ID: "u" + name, Kind: api.ItemUser, Turn: turn, Text: fixtureRequest(t, name, sketch)},
			backend.ChatItem{ID: "a" + name, Kind: api.ItemAgent, Turn: turn, Text: "On it."})
	}
	return items
}

// A request is drawn as the person's box: the action's title, its facts and
// its rows, never its XML or the text the agent read until it is unfolded.
func TestARequestIsDrawnAsThePersonsBox(t *testing.T) {
	for _, shot := range []struct {
		file  string
		names []string
		shows []string
	}{
		{"requests-conflicts-checks.png", []string{"request: resolve conflicts", "request: fix the failing checks"},
			[]string{"Resolve the conflicts", "#1210 · fix-412-handshake → main", "internal/relay/conn.go", "internal/relay/relay.go",
				"Fix the failing checks", "test (ubuntu-latest)", "lint"}},
		{"requests-threads-comments.png", []string{"request: review threads", "request: review comments"},
			[]string{"Address 2 review threads", "internal/relay/conn.go:88-94", "outdated · \"Name the flag.\"",
				"2 review comments", "internal/relay/envelope.go:42", "\"Name this constant.\""}},
		{"request-notes.png", []string{"request: browser notes with a sketch"},
			[]string{"2 notes on the page", "http://localhost:5173/settings", "button \"Save\"", "\"Too close to the edge.\""}},
		{"request-issue.png", []string{"request: start on an issue"},
			[]string{"Issue #1173", "Relay handshake races on reconnect", "https://github.com/lichdotdev/lich/issues/1173"}},
	} {
		u, r := envelopeRig(t, requestItems(t, shot.names...))
		awaitThumbs(u, r)
		if !r.showsAll(shot.shows...) {
			t.Fatalf("%s: the requests are not drawn as boxes:\n%s", shot.file, r.drawn())
		}
		if r.shows("lich-notification") || strings.Contains(r.drawn(), "Resolve them:") {
			t.Errorf("%s: a request drew its XML or its folded text:\n%s", shot.file, r.drawn())
		}
		writeShot(t, r, shot.file)
	}
}

// awaitThumbs draws again once a sketch's thumbnail, read in the background,
// is in.
func awaitThumbs(u *window, r *frameRig) {
	for range 100 {
		ready := true
		for _, th := range u.chats.thumbs {
			if img, _, _ := th.get(); img == nil {
				ready = false
			}
		}
		if ready {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	r.frame()
}

// The chevron unfolds the whole text the agent read under the box's rows.
func TestARequestUnfoldsItsText(t *testing.T) {
	_, r := envelopeRig(t, requestItems(t, "request: resolve conflicts"))
	r.clickLabel(t, "Resolve the conflicts")
	r.frame()
	if !strings.Contains(r.drawn(), "Pull request #1210 (fix-412-handshake) has merge conflicts with main in these files. Resolve them:") {
		t.Fatalf("the text did not unfold:\n%s", r.drawn())
	}
	writeShot(t, r, "request-open.png")
}

// Copying a request copies the text the agent read; the composer's history
// leaves it out, since the person did not type it.
func TestARequestCopiesItsTextAndStaysOutOfHistory(t *testing.T) {
	items := requestItems(t, "request: resolve conflicts")
	envs, _ := parseEnvelopes(items[0].Text)
	if got := envelopesText(envs); !strings.HasPrefix(got, "Pull request #1210") || strings.Contains(got, "<") {
		t.Errorf("copied %q, want the prompt as text", got)
	}
	if sent := sentHistory(items); len(sent) != 0 {
		t.Errorf("history = %q, want no request in it", sent)
	}
}
