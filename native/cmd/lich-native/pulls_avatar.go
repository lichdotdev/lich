package main

import (
	"bytes"
	"image"

	"gioui.org/layout"

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
func decodeAuthorAvatar(data []byte) (*ui.AvatarImage, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return ui.NewAvatarImage(img), nil
}

// authorAvatar is the author's picture, or their initial on a neutral disc
// while it loads, when it fails, or for an account deleted since: color in the
// Pulls screen only ever means state.
func authorAvatar(gtx C, th *ui.Theme, img *ui.AvatarImage, login string) D {
	if login == "" {
		login = i18n.T("pulls.byline.someone")
	}
	return th.Avatar(img, profileInitial(login), authorAvatarSize).Layout(gtx)
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
