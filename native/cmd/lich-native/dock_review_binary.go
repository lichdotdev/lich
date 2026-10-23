package main

// A binary file's card body (BinaryDiffBody): the image on each side of the
// change, or the line saying it is binary with the way out to the desktop's
// application, or to the file on GitHub in a pull request.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"strings"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// binaryPreview is a binary card's state: its sides' reads and its buttons.
type binaryPreview struct {
	before, after *blobView
	exits         fileExits
	github        widget.Clickable
}

// previewsImage reports whether the card draws its file's image: a format
// this window decodes.
func (c *fileCard) previewsImage() bool {
	return c.git.binary && previewKindOf(c.git.path) == blobImage
}

func (c *fileCard) binaryBody(gtx C, p *reviewPanel) D {
	if c.bin == nil {
		c.bin = &binaryPreview{before: newBlobView(p.invalidate), after: newBlobView(p.invalidate)}
	}
	switch {
	case c.previewsImage():
		return c.imageSides(gtx, p)
	case p.pr != nil:
		return c.prBinary(gtx, p)
	}
	c.bin.exits.update(gtx, p.client, p.notify, p.target, c.git.path)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return bodyLine(gtx, p.th, i18n.T("diff.binaryPreview.binary")) }),
		layout.Rigid(func(gtx C) D {
			if c.git.status == statusDeleted {
				return D{}
			}
			return layout.Inset{Left: ui.Space(9), Bottom: ui.Space(3)}.Layout(gtx, func(gtx C) D { return c.bin.exits.layout(gtx, p.th) })
		}),
	)
}

func (c *fileCard) prBinary(gtx C, p *reviewPanel) D {
	if c.bin.github.Clicked(gtx) {
		url := prFileURL(p.pr.url, c.git.path)
		_, name := splitPath(c.git.path)
		go func() {
			if err := p.client.OpenExternal(context.Background(), url); err != nil {
				p.notify(ui.Toast{Variant: ui.ToastError, Title: i18n.T("diff.binaryState.openFailed", i18n.Args{"name": name}), Description: backend.ErrorText(err)})
			}
		}()
	}
	b := p.th.Button(&c.bin.github, i18n.T("diff.dockReviewBinary.viewOnGitHub"))
	b.Size, b.Variant = ui.SizeXS, ui.VariantOutline
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return bodyLine(gtx, p.th, i18n.T("diff.binaryPreview.binary")) }),
		layout.Rigid(b.Layout),
	)
}

// prFileURL is the file's anchor in the pull request's Files changed tab,
// which GitHub names after the SHA-256 of its path.
func prFileURL(prURL, path string) string {
	sum := sha256.Sum256([]byte(path))
	return strings.TrimSuffix(prURL, "/") + "/files#diff-" + hex.EncodeToString(sum[:])
}

// imageSides is the grid of BinaryDiffBody, px-3 pt-2.5 pb-3.5 gap-2.5: both
// sides of a change, or the one an added or deleted file has. A working tree
// change stands on HEAD; a pull request's on its merge base and its head.
func (c *fileCard) imageSides(gtx C, p *reviewPanel) D {
	old := c.git.oldPath
	if old == "" {
		old = c.git.path
	}
	wantBefore := func() { c.bin.before.want(p.client, p.target, old, "HEAD", c.git.blobs) }
	afterRef := ""
	if f := p.pr; f != nil {
		wantBefore = func() { c.bin.before.wantMergeBase(p.client, p.target, old, f, c.git.blobs) }
		afterRef = f.head
	}
	var sides []layout.FlexChild
	side := func(v *blobView, label string, tone color.NRGBA) layout.FlexChild {
		return layout.Flexed(1, func(gtx C) D { return sideColumn(gtx, p.th, v, label, tone) })
	}
	switch c.git.status {
	case statusAdded:
		c.bin.after.want(p.client, p.target, c.git.path, afterRef, c.git.blobs)
		sides = append(sides, side(c.bin.after, i18n.T("diff.binaryPreview.added"), p.th.TonePass))
	case statusDeleted:
		wantBefore()
		sides = append(sides, side(c.bin.before, i18n.T("diff.binaryPreview.deleted"), p.th.Destructive))
	default:
		wantBefore()
		c.bin.after.want(p.client, p.target, c.git.path, afterRef, c.git.blobs)
		sides = append(sides, side(c.bin.before, i18n.T("diff.binaryPreview.before"), p.th.Destructive), ui.Gap(2.5), side(c.bin.after, i18n.T("diff.binaryPreview.after"), p.th.TonePass))
	}
	return layout.Inset{Left: ui.Space(3), Right: ui.Space(3), Top: ui.Space(2.5), Bottom: ui.Space(3.5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{}.Layout(gtx, sides...)
	})
}

// sideColumn is one side under its SideLabel: a toned dot and the side's
// name, uppercase.
func sideColumn(gtx C, th *ui.Theme, v *blobView, label string, tone color.NRGBA) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					side := gtx.Dp(ui.Space(1.5))
					gtx.Constraints = layout.Exact(image.Pt(side, side))
					return ui.Fill(gtx, tone, ui.Space(0.75))
				}),
				ui.Gap(1.5),
				layout.Rigid(th.Text(text2XS, strings.ToUpper(label)).Leading(text2XSLeading).Muted().Layout),
			)
		}),
		ui.VGap(1),
		layout.Rigid(func(gtx C) D { return v.layout(gtx, th) }),
	)
}
