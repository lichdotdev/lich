package chat

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
)

// pngHead is enough of a PNG for its bytes to say what it is.
var pngHead = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")

func writeAttachment(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// An image goes as one, told by its bytes; a file named like one, a file too
// large to send as one and any other file go as their paths under the text,
// quoted where they hold a space, their notices under them.
func TestComposeSendsImagesAsImagesAndTheRestAsPaths(t *testing.T) {
	shot := writeAttachment(t, "shot.png", pngHead)
	fake := writeAttachment(t, "fake.png", []byte("not an image"))
	notes := writeAttachment(t, "my notes.md", []byte("# notes"))
	big := writeAttachment(t, "big.png", append(slices.Clone(pngHead), make([]byte, maxImageBytes)...))
	m, err := compose("look", []api.DropAttachment{{Path: shot}, {Path: fake}, {Path: notes, Notice: "a copy"}, {Path: big}})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.images) != 1 || m.images[0].path != shot || m.images[0].mime != "image/png" || !bytes.Equal(m.images[0].data, pngHead) {
		t.Fatalf("images = %+v, want shot.png alone, whole", m.images)
	}
	want := "look\n\n" + quotePath(fake) + " " + quotePath(notes) + " " + quotePath(big) + "\na copy"
	if m.text != want {
		t.Errorf("text = %q, want %q", m.text, want)
	}
}

// A message of files alone is their paths; one with none is its text.
func TestComposeWithoutText(t *testing.T) {
	notes := writeAttachment(t, "notes.md", []byte("x"))
	if m, _ := compose("", []api.DropAttachment{{Path: notes}}); m.text != quotePath(notes) {
		t.Errorf("text = %q, want the path alone", m.text)
	}
	if m, _ := compose("hi", nil); m.text != "hi" || m.images != nil {
		t.Errorf("message = %+v, want hi alone", m)
	}
}

// A file that cannot be read refuses the message, naming it.
func TestComposeRefusesAFileItCannotRead(t *testing.T) {
	_, err := compose("x", []api.DropAttachment{{Path: filepath.Join(t.TempDir(), "gone.png")}})
	if err == nil || !strings.Contains(err.Error(), "gone.png") {
		t.Errorf("err = %v, want one naming gone.png", err)
	}
}

// A path with a space or a quote is one quoted piece; a plain one stays as it
// is.
func TestQuotePath(t *testing.T) {
	for in, want := range map[string]string{
		"/a/b.md":    "/a/b.md",
		"/a/b c.md":  "'/a/b c.md'",
		"/a/it's.md": `'/a/it'\''s.md'`,
	} {
		if got := quotePath(in); got != want {
			t.Errorf("quotePath(%q) = %q, want %q", in, got, want)
		}
	}
}

// An agent that takes no image gets each one as its path.
func TestImagesAsText(t *testing.T) {
	m := message{text: "look", images: []attachedImage{{path: "/a/b.png", mime: "image/png"}}}.imagesAsText()
	if m.text != "look\n\n/a/b.png" || m.images != nil {
		t.Errorf("message = %+v", m)
	}
}

// An ACP prompt carries an image block for each image where the agent takes
// one, and no empty text block for a message of images alone; where it takes
// none, the image is a path in the text block.
func TestACPContent(t *testing.T) {
	img := attachedImage{path: "/a/b.png", mime: "image/png", data: pngHead}
	d := &acpDriver{images: true}
	got := d.content(message{images: []attachedImage{img}})
	if len(got) != 1 || got[0]["type"] != "image" || got[0]["mimeType"] != "image/png" || got[0]["data"] != base64.StdEncoding.EncodeToString(pngHead) {
		t.Errorf("blocks = %v, want one image block", got)
	}
	d.images = false
	got = d.content(message{text: "look", images: []attachedImage{img}})
	if len(got) != 1 || got[0]["type"] != "text" || got[0]["text"] != "look\n\n/a/b.png" {
		t.Errorf("blocks = %v, want the path in the text", got)
	}
}

// Claude Code's content is the text alone without images, and the API's
// blocks with them, no text block for a message of images alone.
func TestClaudeContent(t *testing.T) {
	if got := claudeContent(message{text: "hi"}); got != "hi" {
		t.Errorf("content = %v, want the text", got)
	}
	img := attachedImage{mime: "image/png", data: pngHead}
	blocks, _ := claudeContent(message{images: []attachedImage{img}}).([]map[string]any)
	if len(blocks) != 1 || blocks[0]["type"] != "image" {
		t.Fatalf("content = %v, want one image block", blocks)
	}
	src, _ := blocks[0]["source"].(map[string]any)
	if src["type"] != "base64" || src["media_type"] != "image/png" || src["data"] != base64.StdEncoding.EncodeToString(pngHead) {
		t.Errorf("source = %v", src)
	}
}

// Claude Code reads an @path only from a text block after the images: with the
// text first it leaves the mention unread (measured on 2.1.330, stream-json).
func TestClaudeContentPutsTheTextAfterTheImages(t *testing.T) {
	img := attachedImage{mime: "image/png", data: pngHead}
	blocks, _ := claudeContent(message{text: "read @a.go", images: []attachedImage{img, img}}).([]map[string]any)
	if len(blocks) != 3 || blocks[0]["type"] != "image" || blocks[1]["type"] != "image" || blocks[2]["text"] != "read @a.go" {
		t.Errorf("content = %v, want the images, then the text", blocks)
	}
}
