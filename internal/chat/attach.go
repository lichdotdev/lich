package chat

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/shquote"
)

// message is what the person sent, as the provider is handed it: the text,
// with every attached file that goes as text under it, and the images that
// go as images.
type message struct {
	text   string
	images []attachedImage
}

type attachedImage struct {
	path, mime string
	data       []byte
}

// maxImageBytes is the largest file sent as an image, Claude Code's own
// targetRawSize: the 3/4 of its 5 MB maxBase64Size that base64 fits in
// (read from the 2.1.330 binary). A larger one goes as its path.
const maxImageBytes = 5 << 20 / 4 * 3

// sniffBytes is what http.DetectContentType reads.
const sniffBytes = 512

// imageTypes are the formats sent as an image, the Anthropic API's four,
// told by the file's bytes: a name can lie, and a wrong media type fails the
// whole turn.
var imageTypes = []string{"image/png", "image/jpeg", "image/gif", "image/webp"}

// compose reads the attachments of a message: an image goes as one, every
// other file as its path under the text, with its notice. A file that cannot
// be read refuses the message.
func compose(text string, files []api.DropAttachment) (message, error) {
	m := message{text: text}
	var asText []api.DropAttachment
	for _, f := range files {
		img, ok, err := readImage(f.Path)
		if err != nil {
			return message{}, fmt.Errorf("cannot attach %s: %w", filepath.Base(f.Path), err)
		}
		if ok {
			m.images = append(m.images, img)
		} else {
			asText = append(asText, f)
		}
	}
	m.text = underText(text, asText)
	return m, nil
}

// readImage reads path as an image, false for a file sent as its path.
func readImage(path string) (attachedImage, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return attachedImage{}, false, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return attachedImage{}, false, err
	}
	if info.IsDir() || info.Size() > maxImageBytes {
		return attachedImage{}, false, nil
	}
	head := make([]byte, sniffBytes)
	n, err := io.ReadFull(f, head)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return attachedImage{}, false, err
	}
	mime := http.DetectContentType(head[:n])
	if !slices.Contains(imageTypes, mime) {
		return attachedImage{}, false, nil
	}
	rest, err := io.ReadAll(f)
	if err != nil {
		return attachedImage{}, false, err
	}
	return attachedImage{path: path, mime: mime, data: append(head[:n], rest...)}, true, nil
}

// imagesAsText is m for a provider that takes no image: each goes as its path.
func (m message) imagesAsText() message {
	if len(m.images) == 0 {
		return m
	}
	files := make([]api.DropAttachment, len(m.images))
	for i, img := range m.images {
		files[i] = api.DropAttachment{Path: img.path}
	}
	return message{text: underText(m.text, files)}
}

// underText is text with files' paths under it as a drop pastes them at the
// terminal: on one line, quoted where they need it, the notices under them.
func underText(text string, files []api.DropAttachment) string {
	if len(files) == 0 {
		return text
	}
	paths := make([]string, len(files))
	lines := []string{""}
	for i, f := range files {
		paths[i] = quotePath(f.Path)
		if f.Notice != "" {
			lines = append(lines, f.Notice)
		}
	}
	lines[0] = strings.Join(paths, " ")
	block := strings.Join(lines, "\n")
	if text == "" {
		return block
	}
	return text + "\n\n" + block
}

var plainPath = regexp.MustCompile(`^[\w@%+=:,./\\-]+$`)

// quotePath keeps a path with a space one piece. The message goes to a model,
// not a shell, so one rule serves every system.
func quotePath(path string) string {
	if plainPath.MatchString(path) {
		return path
	}
	return shquote.Quote(path)
}

// filePaths are the attachments' paths, as the person's message shows them.
func filePaths(files []api.DropAttachment) []string {
	if len(files) == 0 {
		return nil
	}
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.Path
	}
	return paths
}
