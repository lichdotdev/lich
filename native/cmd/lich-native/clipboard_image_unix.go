//go:build !darwin && !windows

package main

import (
	"context"
	"os"
)

// clipboardImage reads the image with wl-paste (wl-clipboard) or xclip.
func clipboardImage(ctx context.Context) (clipImage, error) {
	return readClipImage(ctx, execClip, unixClipTools(os.Getenv("WAYLAND_DISPLAY") != ""))
}
