package cli

import (
	"fmt"

	"github.com/lichdotdev/lich/internal/restart"
)

// galleryEnv pins the UI kit gallery to launch, by path, the way
// restart.NativeEnv pins the native window.
const galleryEnv = "LICH_GALLERY"

// galleryBinary is the gallery's executable, looked up beside lich's.
const galleryBinary = "lich-gallery"

// gallery opens the native UI kit gallery. It needs no backend and touches no
// window, so it only starts the gallery and returns.
func (c *client) gallery(args []string) error {
	flags := newFlagSet("gallery")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return usageError("gallery")
	}
	exe, err := restart.Companion(c.env, galleryEnv, galleryBinary, "build it from native/ with "+
		"`go build -o <dir of lich>/"+galleryBinary+" ./cmd/ui-gallery` (cgo and Gio's system packages, no libghostty-vt)")
	if err != nil {
		return err
	}
	if err := c.start(exe, nil, nil); err != nil {
		return fmt.Errorf("launch %s: %w", exe, err)
	}
	fmt.Fprintln(c.stdout, "Opening the native UI gallery.")
	return nil
}
