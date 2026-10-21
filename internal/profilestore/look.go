package profilestore

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// ChangedEvent is the event a profile's update emits once it is written: its
// name, color or image changed.
const ChangedEvent = "profile-changed"

// Changed is ChangedEvent's payload. Provider is the Spec's.
type Changed struct {
	Provider string  `json:"provider"`
	Profile  Profile `json:"profile"`
}

// Update is a change to how a profile is shown. A field left nil keeps what the
// profile has; Name keeps it when empty.
type Update struct {
	Name string `json:"name,omitempty"`
	// Color is a card color's name; "" goes back to the color the window
	// derives from the slug. The window draws a name it does not know the same
	// way, so it is stored as given.
	Color *string `json:"color,omitempty"`
	// Image is a file to make the profile's image from; "" removes the image.
	Image *string `json:"image,omitempty"`
}

const (
	// avatarEdge is the side of the square a profile image is stored at: the
	// largest avatar the window draws is 28dp, so this holds it at 4x.
	avatarEdge = 128
	// maxImageFileBytes bounds the file a profile image is made from.
	maxImageFileBytes = 10 << 20
	// maxImagePixels bounds the image by its header before it is decoded, the
	// window's own bound on a preview: a small PNG can unpack to gigabytes.
	maxImagePixels = 40_000_000
)

// Update applies u to a profile, Personal's included, and returns it as List
// shows it. A file that is not an image fails it before anything is written.
func (s Spec) Update(slug string, u Update) (Profile, error) {
	if IsPersonal(slug) {
		slug = Personal
	}
	name := strings.TrimSpace(u.Name)
	if u.Name != "" && name == "" {
		return Profile{}, errors.New("a profile needs a name")
	}
	var avatar []byte
	if u.Image != nil && *u.Image != "" {
		var err error
		if avatar, err = avatarPNG(*u.Image); err != nil {
			return Profile{}, err
		}
	}
	err := s.editMeta(slug, func(m *metaFile) {
		if name != "" {
			m.Name = name
		}
		if u.Color != nil {
			m.Color = *u.Color
		}
	})
	if err != nil {
		return Profile{}, err
	}
	if u.Image != nil {
		if err := s.writeImage(slug, avatar); err != nil {
			return Profile{}, err
		}
	}
	return s.find(slug)
}

// editMeta rewrites a profile's sidecar through edit. Personal's may not exist
// yet; any other profile's must.
func (s Spec) editMeta(slug string, edit func(*metaFile)) error {
	if slug == Personal {
		root, err := s.Root()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(root, 0o755); err != nil {
			return err
		}
	} else if _, err := s.ConfigDir(slug); err != nil {
		return err
	}
	m, err := s.readMeta(slug)
	if err != nil && (slug != Personal || !errors.Is(err, fs.ErrNotExist)) {
		return err
	}
	edit(&m)
	return s.writeMeta(slug, m)
}

func (s Spec) find(slug string) (Profile, error) {
	list, err := s.List()
	if err != nil {
		return Profile{}, err
	}
	for _, p := range list {
		if p.Slug == slug {
			return p, nil
		}
	}
	return Profile{}, fmt.Errorf("%s %w: %q was deleted", s.Provider, ErrGone, slug)
}

func (s Spec) imagePath(slug string) (string, error) {
	root, err := s.Root()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, slug+".png"), nil
}

// readImage is a profile's image, nil when it has none. One that cannot be read
// is shown as none, the way List skips a sidecar that does not parse.
func (s Spec) readImage(slug string) []byte {
	path, err := s.imagePath(slug)
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return data
}

// writeImage stores a profile's image; nil removes it.
func (s Spec) writeImage(slug string, data []byte) error {
	path, err := s.imagePath(slug)
	if err != nil {
		return err
	}
	if data != nil {
		return WriteFileAtomic(path, data, 0o600)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove profile image: %w", err)
	}
	return nil
}

// avatarPNG reads an image file and returns its centered square, scaled to
// avatarEdge, as a PNG.
func avatarPNG(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("read profile image: %w", err)
	}
	if info.Size() > maxImageFileBytes {
		return nil, fmt.Errorf("%s is %.1f MB; a profile image can be at most %d MB", filepath.Base(path), float64(info.Size())/(1<<20), maxImageFileBytes>>20)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read profile image: %w", err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%s is not a PNG, JPEG, GIF or WebP image", filepath.Base(path))
	}
	if cfg.Width*cfg.Height > maxImagePixels {
		return nil, fmt.Errorf("%s is %d × %d; a profile image can be at most %d megapixels", filepath.Base(path), cfg.Width, cfg.Height, maxImagePixels/1_000_000)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", filepath.Base(path), err)
	}
	dst := image.NewNRGBA(image.Rect(0, 0, avatarEdge, avatarEdge))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, centeredSquare(img.Bounds()), xdraw.Src, nil)
	var out bytes.Buffer
	if err := png.Encode(&out, dst); err != nil {
		return nil, fmt.Errorf("encode profile image: %w", err)
	}
	return out.Bytes(), nil
}

func centeredSquare(b image.Rectangle) image.Rectangle {
	side := min(b.Dx(), b.Dy())
	at := b.Min.Add(image.Pt((b.Dx()-side)/2, (b.Dy()-side)/2))
	return image.Rectangle{Min: at, Max: at.Add(image.Pt(side, side))}
}
