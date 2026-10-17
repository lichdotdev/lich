package main

import (
	"bytes"
	"image"
	"image/png"
	"strings"
	"testing"
)

func TestPreviewKindOf(t *testing.T) {
	for rel, want := range map[string]blobKind{
		"docs/shot.PNG":     blobImage,
		"a/photo.jpeg":      blobImage,
		"anim.gif":          blobImage,
		"pic.webp":          blobImage,
		"spec.pdf":          blobPDF,
		"favicon.ico":       blobNone, // served by /blob, not decoded here
		"logo.svg":          blobNone,
		"release.zip":       blobNone,
		"Makefile":          blobNone,
		"dir.png/README.md": blobNone,
	} {
		if got := previewKindOf(rel); got != want {
			t.Errorf("previewKindOf(%q) = %d, want %d", rel, got, want)
		}
	}
}

func encodePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDecodePreviewCaptionsTheFile(t *testing.T) {
	data := encodePNG(t, 3, 2)
	img, err := decodePreview(data)
	if err != nil {
		t.Fatalf("decodePreview: %v", err)
	}
	want := "3 × 2 · PNG · " + formatBlobSize(len(data))
	if got := img.caption(); got != want {
		t.Errorf("caption = %q, want %q", got, want)
	}
}

func TestDecodePreviewScalesTheLongestEdgeDown(t *testing.T) {
	img, err := decodePreview(encodePNG(t, 3000, 10))
	if err != nil {
		t.Fatalf("decodePreview: %v", err)
	}
	if got := img.src.Size(); got.X != maxPreviewEdge {
		t.Errorf("held at %v, want a %d px longest edge", got, maxPreviewEdge)
	}
	if img.width != 3000 || img.height != 10 {
		t.Errorf("caption size %d × %d, want the file's own 3000 × 10", img.width, img.height)
	}
}

// TestDecodePreviewRefusesAHugeImageByItsHeader proves the pixel cap reads the
// header alone: the GIF below claims 10000 × 10000 and holds no image data.
func TestDecodePreviewRefusesAHugeImageByItsHeader(t *testing.T) {
	header := []byte("GIF89a\x10\x27\x10\x27\x00\x00\x00")
	_, err := decodePreview(header)
	if err == nil || !strings.Contains(err.Error(), "10000 × 10000 is above the 40 MP preview limit") {
		t.Fatalf("err = %v, want the pixel limit", err)
	}
}

func TestDecodePreviewSaysWhyItCannotDecode(t *testing.T) {
	_, err := decodePreview([]byte("not an image"))
	if err == nil || !strings.HasPrefix(err.Error(), "this image can't be decoded") {
		t.Fatalf("err = %v", err)
	}
}

func TestFormatBlobSize(t *testing.T) {
	for n, want := range map[int]string{512: "0.5 KB", 1024 * 1024: "1.0 MB", 1536 * 1024: "1.5 MB"} {
		if got := formatBlobSize(n); got != want {
			t.Errorf("formatBlobSize(%d) = %q, want %q", n, got, want)
		}
	}
}
