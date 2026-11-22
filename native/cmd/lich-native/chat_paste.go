package main

import (
	"bytes"
	"errors"
	"log"

	"gioui.org/io/clipboard"
	"gioui.org/io/key"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// A terminal agent reads an image off the clipboard at its own prompt; a
// chat's has nobody to, so the composer's paste does: an image with no text
// beside it becomes a card, stored through /drop as a dropped file is, so the
// copy lives under the session's copies and goes when the session does
// (internal/drop).

// pastedImageBase is the name a pasted image's copy takes, numbered by /drop
// on a clash.
const pastedImageBase = "pasted-image"

// pasteToolToast is the toast saying the machine has no tool to read an image.
const pasteToolToast = "paste:no-tool"

// pasteKeys takes the composer's Ctrl+V (Cmd+V on macOS) ahead of its
// editor: the editor still reads the clipboard's text, and the image is read
// beside it.
func (u *window) pasteKeys(gtx C, p *chatPane, id string) {
	for {
		ev, ok := gtx.Event(key.Filter{Focus: &p.editor, Name: "V", Required: key.ModShortcut})
		if !ok {
			return
		}
		if e, ok := ev.(key.Event); !ok || e.State != key.Press {
			continue
		}
		// What the editor's own V runs: its text paste lands as it always did.
		gtx.Execute(clipboard.ReadCmd{Tag: &p.editor})
		if u.chats.readImage != nil {
			go u.pasteImage(id)
		}
	}
}

// pasteImage reads the clipboard's image, off the UI goroutine, into a card
// in session id's composer.
func (u *window) pasteImage(id string) {
	img, err := u.chats.readImage(u.ctx)
	switch {
	case errors.Is(err, errNoClipboardTool):
		// Without the tool nobody knows an image was there, so the first
		// Ctrl+V of a window says so, text or not, and no other.
		if u.chats.toolWarned.CompareAndSwap(false, true) {
			u.toaster.Show(ui.Toast{ID: pasteToolToast, Variant: ui.ToastError, Title: i18n.T("chat.attach.pasteNeedsTool")})
		}
		return
	case err != nil:
		log.Printf("paste image into %s: %v", id, err)
		return
	case img.data == nil:
		return
	}
	name := pastedImageBase + img.ext
	a, err := u.client.UploadDrop(u.ctx, id, name, bytes.NewReader(img.data))
	if err != nil {
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("terminal.drop.notAttached", i18n.Args{"files": skippedDrop(name, backend.ErrorText(err))})})
		return
	}
	// No notice: a pasted image is no copy of a file the session cannot reach.
	u.chats.attachIn(id, []backend.Attachment{{Path: a.Path}})
	u.model.invalidate()
}
