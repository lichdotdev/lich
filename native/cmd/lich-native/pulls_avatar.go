package main

import (
	"bytes"
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// An author's avatar sits in a gutter beside the entry, GitHub's way: the
// column of faces is what the eye scans a conversation by.
var (
	authorAvatarSize = ui.Space(6)
	authorAvatarGap  = ui.Space(2.5)
)

// author is who wrote an entry: their login and the address of their picture,
// both "" for an account deleted since.
type author struct{ login, avatar string }

// decodeAuthorAvatar is a picture GitHub serves, PNG or JPEG, ready to draw.
func decodeAuthorAvatar(data []byte) (*avatarImage, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return &avatarImage{src: img, sized: map[int]paint.ImageOp{}}, nil
}

// authorAvatar is the author's picture, or their initial on a neutral disc
// while it loads, when it fails, or for an account deleted since: color in the
// Pulls screen only ever means state.
func authorAvatar(gtx C, th *ui.Theme, img *avatarImage, login string) D {
	size := authorAvatarSize
	px := gtx.Dp(size)
	gtx.Constraints = layout.Exact(image.Pt(px, px))
	ui.Fill(gtx, th.Accent, size/2)
	if img == nil {
		if login == "" {
			login = i18n.T("pulls.byline.someone")
		}
		label := th.Text(ui.TextXS, profileInitial(login)).Weight(font.SemiBold).Muted()
		return layout.Center.Layout(gtx, label.Layout)
	}
	defer clip.Ellipse{Max: image.Pt(px, px)}.Push(gtx.Ops).Pop()
	img.at(px).Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	return D{Size: image.Pt(px, px)}
}

// withAvatar lays entry out beside its author's avatar.
func (p *pullsScreen) withAvatar(gtx C, th *ui.Theme, who author, entry layout.Widget) D {
	img := p.store.avatar(p.project.Path, who.avatar)
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return authorAvatar(gtx, th, img, who.login) }),
		layout.Rigid(layout.Spacer{Width: authorAvatarGap}.Layout),
		layout.Flexed(1, entry),
	)
}
