package main

import (
	"context"
	"errors"
	"os/exec"
)

// macClipScript answers the clipboard's PNG, the «class PNGf» Claude Code
// asks for, and "" when the clipboard holds text, which the editor pastes.
// Reading an image as text fails, hence the try.
const macClipScript = `try
	if (the clipboard as text) is not "" then return ""
end try
return the clipboard as «class PNGf»`

// clipboardImage reads the image with osascript, which fails on a clipboard
// with no PNG.
func clipboardImage(ctx context.Context) (clipImage, error) {
	out, err := execClip(ctx, clipReadTimeout, []string{"osascript", "-e", macClipScript})
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return clipImage{}, nil
	}
	if err != nil {
		return clipImage{}, err
	}
	return parseMacPNG(out)
}
