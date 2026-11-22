package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

// winClipScript prints the clipboard's image as a base64 PNG, the
// Clipboard.GetImage Claude Code reads, and exits 1 over text, which the
// editor pastes, or no image. The clipboard answers only an STA thread.
const winClipScript = `Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
if ([System.Windows.Forms.Clipboard]::ContainsText()) { exit 1 }
$i = [System.Windows.Forms.Clipboard]::GetImage()
if ($null -eq $i) { exit 1 }
$ms = New-Object System.IO.MemoryStream
$i.Save($ms, [System.Drawing.Imaging.ImageFormat]::Png)
[Convert]::ToBase64String($ms.ToArray())`

// clipboardImage reads the image with PowerShell, its window hidden.
func clipboardImage(ctx context.Context) (clipImage, error) {
	ctx, cancel := context.WithTimeout(ctx, clipReadTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Sta", "-EncodedCommand", encodePowerShell(winClipScript))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	out, err := cmd.Output()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return clipImage{}, nil
	}
	if err != nil {
		return clipImage{}, err
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(out)))
	if err != nil {
		return clipImage{}, fmt.Errorf("powershell PNG: %w", err)
	}
	return clipImage{data: data, ext: ".png"}, nil
}
