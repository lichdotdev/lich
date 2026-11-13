package extensions

import (
	"bytes"
	"fmt"
	"image/png"
	"io"
	"os"

	"github.com/lichdotdev/lich/extension"
)

// maxIconFileBytes and maxIconSide bound a surface's iconFile: every status
// change carries its bytes to the window, which draws it at 16 dp.
const (
	maxIconFileBytes = 256 << 10
	maxIconSide      = 512
)

// readIcons reads each surface's iconFile, by surface id.
func readIcons(root string, m extension.Manifest) (map[string][]byte, error) {
	var out map[string][]byte
	for _, s := range m.Surfaces {
		if s.IconFile == "" {
			continue
		}
		data, err := readIcon(root, s.IconFile)
		if err != nil {
			return nil, err
		}
		if out == nil {
			out = map[string][]byte{}
		}
		out[s.ID] = data
	}
	return out, nil
}

func readIcon(root, relative string) ([]byte, error) {
	path, err := resolveInside(root, "iconFile", relative)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("iconFile %q: %w", relative, err)
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxIconFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("iconFile %q: %w", relative, err)
	}
	if len(data) > maxIconFileBytes {
		return nil, fmt.Errorf("iconFile %q can be at most %d KB", relative, maxIconFileBytes>>10)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("iconFile %q is not a PNG", relative)
	}
	if cfg.Width > maxIconSide || cfg.Height > maxIconSide {
		return nil, fmt.Errorf("iconFile %q is %d × %d; it can be at most %d × %d", relative, cfg.Width, cfg.Height, maxIconSide, maxIconSide)
	}
	if _, err := png.Decode(bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("iconFile %q: %w", relative, err)
	}
	return data, nil
}
