package main

import (
	"context"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// A file dropped on a chat, or picked with its attach "+", stands over the
// composer's text as a card, an image as its thumbnail and any other file as
// its name and type, as Claude's web composer draws them, with an × to take it
// off and a click that shows it whole. It goes with the message, which the
// backend hands the agent as an image or as the file's path
// (api.ChatPromptOptions), and the person's message keeps its cards.

var (
	// chatCardSize is the side of a card, Claude web's 120px, and
	// chatCardGap the room between two.
	chatCardSize = ui.Space(30)
	chatCardGap  = ui.Space(2)
	// chatCardPad is the room a file's name and type keep from the card's
	// edge, and chatCardRemoveInset the × from its corner.
	chatCardPad         = ui.Space(2.5)
	chatCardRemoveInset = ui.Space(1.5)
	// chatCardNameLines is how many lines a file's name wraps to.
	chatCardNameLines = 3
)

// chatMessage is what the composer sends: its text and the files attached.
type chatMessage struct {
	text  string
	files []backend.Attachment
}

func (m chatMessage) empty() bool { return m.text == "" && len(m.files) == 0 }

// line is m as its queued line: the text on one line, then the files' names.
func (m chatMessage) line() string {
	parts := []string{strings.ReplaceAll(m.text, "\n", " ")}
	if m.text == "" {
		parts = nil
	}
	for _, f := range m.files {
		parts = append(parts, filepath.Base(f.Path))
	}
	return strings.Join(parts, " · ")
}

// joinMessages is msgs as the one message the turn's end sends: their texts
// joined, their files in order.
func joinMessages(msgs []chatMessage) chatMessage {
	var j chatMessage
	var texts []string
	for _, m := range msgs {
		if m.text != "" {
			texts = append(texts, m.text)
		}
		j.files = append(j.files, m.files...)
	}
	j.text = strings.Join(texts, chatQueueJoin)
	return j
}

// addFiles is list with each of more not already in it, by path: one file
// attached twice is one card.
func addFiles(list, more []backend.Attachment) []backend.Attachment {
	for _, f := range more {
		if !slices.ContainsFunc(list, func(g backend.Attachment) bool { return g.Path == f.Path }) {
			list = append(list, f)
		}
	}
	return list
}

// filePaths are files' paths, as the person's message shows them.
func filePaths(files []backend.Attachment) []string {
	if len(files) == 0 {
		return nil
	}
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.Path
	}
	return paths
}

// chatFileCard is one card's controls: the card, which shows the file, and
// its ×.
type chatFileCard struct {
	open, remove ui.Clickable
	removeTip    ui.Tooltip
}

// composed is what the composer holds as a message.
func (p *chatPane) composed() chatMessage {
	return chatMessage{text: strings.TrimSpace(p.editor.Text()), files: p.attached}
}

// clearComposer empties the text and the cards of a message that went.
func (p *chatPane) clearComposer() {
	p.editor.SetText("")
	p.undo.Reset()
	p.attached = nil
}

// restore puts m back in the composer, ahead of what was written since.
func (p *chatPane) restore(m chatMessage) {
	text := m.text
	if typed := strings.TrimSpace(p.editor.Text()); typed != "" {
		text = joinMessages([]chatMessage{{text: text}, {text: typed}}).text
	}
	p.editor.SetText(text)
	p.editor.SetCaret(len([]rune(text)), len([]rune(text)))
	p.attached = addFiles(addFiles(nil, m.files), p.attached)
}

func (p *chatPane) card(key string) *chatFileCard {
	c, ok := p.fileCards[key]
	if !ok {
		if p.fileCards == nil {
			p.fileCards = map[string]*chatFileCard{}
		}
		c = new(chatFileCard)
		p.fileCards[key] = c
	}
	return c
}

// attachIn hands files to session id's composer, from any goroutine, for the
// next frame to draw.
func (c *chatScreens) attachIn(id string, files []backend.Attachment) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.attached == nil {
		c.attached = map[string][]backend.Attachment{}
	}
	c.attached[id] = append(c.attached[id], files...)
}

// takeAttached is what was handed session id's composer, now taken.
func (c *chatScreens) takeAttached(id string) []backend.Attachment {
	c.mu.Lock()
	defer c.mu.Unlock()
	files := c.attached[id]
	delete(c.attached, id)
	return files
}

// thumb is the decoded image at path, read off the frame once; nil while it
// reads, and for a file that is no image the window decodes.
// shortcut: a thumb is kept at maxPreviewEdge, the preview's size, for the
// window's life; decode a card-sized one if many large images show up.
func (u *window) thumb(path string) *previewImage {
	if previewKindOf(path) != blobImage {
		return nil
	}
	r, ok := u.chats.thumbs[path]
	if !ok {
		if u.chats.thumbs == nil {
			u.chats.thumbs = map[string]*remote[*previewImage]{}
		}
		r = &remote[*previewImage]{invalidate: u.model.invalidate}
		u.chats.thumbs[path] = r
		r.want(path, "", func(context.Context) (*previewImage, error) { return readPreview(path) })
	}
	img, _, _ := r.get()
	return img
}

func readPreview(path string) (*previewImage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return decodePreview(data)
}

// cardActs runs the composer's cards' clicks before they are drawn: a ×
// takes its card off.
func (p *chatPane) cardActs(gtx C) {
	p.attached = slices.DeleteFunc(p.attached, func(f backend.Attachment) bool { return p.card(f.Path).remove.Clicked(gtx) })
}

// composerCards is the row of the composer's cards over its text, nothing
// without any.
func (u *window) composerCards(gtx C, th *ui.Theme, p *chatPane, fill color.NRGBA) D {
	if len(p.attached) == 0 {
		return D{}
	}
	files := p.attached
	return layout.Inset{Top: chatComposerPadY, Left: chatComposerPadX, Right: chatComposerPadX}.Layout(gtx, func(gtx C) D {
		return p.cardList.Layout(gtx, len(files), func(gtx C, i int) D {
			return layout.Inset{Right: chatCardGap}.Layout(gtx, func(gtx C) D {
				return u.fileCard(gtx, th, p.card(files[i].Path), files[i].Path, fill, true)
			})
		})
	})
}

// sentCards is the row of cards a person's message attached, at the
// column's right over the message.
func (u *window) sentCards(gtx C, th *ui.Theme, p *chatPane, item string, paths []string) D {
	if len(paths) == 0 {
		return D{}
	}
	return layout.E.Layout(gtx, func(gtx C) D {
		children := make([]layout.FlexChild, 0, 2*len(paths))
		for i, path := range paths {
			if i > 0 {
				children = append(children, layout.Rigid(layout.Spacer{Width: chatCardGap}.Layout))
			}
			children = append(children, layout.Rigid(func(gtx C) D {
				return u.fileCard(gtx, th, p.card(item+"\x00"+path), path, th.Accent, false)
			}))
		}
		return layout.Inset{Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D { return layout.Flex{}.Layout(gtx, children...) })
	})
}

// fileCard is one file's card on fill, and its × while the pointer is on it
// when removable. A click opens the preview, which the window draws before
// its body: the frame after shows it.
func (u *window) fileCard(gtx C, th *ui.Theme, c *chatFileCard, path string, fill color.NRGBA, removable bool) D {
	if c.open.Clicked(gtx) {
		u.attachPreview.show(u, path)
		gtx.Execute(op.InvalidateCmd{})
	}
	side := gtx.Dp(chatCardSize)
	size := image.Pt(side, side)
	gtx.Constraints = layout.Exact(size)
	radius := gtx.Dp(ui.RadiusLG)
	img := u.thumb(path)
	c.open.Layout(gtx, func(gtx C) D {
		defer clip.UniformRRect(image.Rectangle{Max: size}, radius).Push(gtx.Ops).Pop()
		paint.FillShape(gtx.Ops, th.Over(fill), clip.Rect{Max: size}.Op())
		if img != nil {
			pic := th.Image(img.src)
			pic.Fit = widget.Cover
			return pic.Layout(gtx)
		}
		return fileCardText(gtx, th.On(fill), filepath.Base(path))
	})
	ui.InsetBorder(gtx, size, radius, th.Over(th.Border))
	if removable {
		shown := c.open.Hovered() || c.remove.Hovered()
		layout.NE.Layout(gtx, func(gtx C) D {
			return layout.UniformInset(chatCardRemoveInset).Layout(gtx, func(gtx C) D {
				b := th.Button(&c.remove, "")
				b.Icon, b.Variant, b.Size = icQueueDrop, ui.VariantSecondary, ui.SizeIconXS
				return th.Tooltip(&u.root, &c.removeTip, i18n.T("chat.attach.remove")).Layout(gtx, func(gtx C) D { return revealed(gtx, shown, b.Layout) })
			})
		})
	}
	return D{Size: size}
}

// fileCardText is a file's name, wrapped to a few lines, at the card's top
// and its type in capitals at its foot, which the name then leaves out.
func fileCardText(gtx C, th *ui.Theme, name string) D {
	ext := strings.ToUpper(strings.TrimPrefix(filepath.Ext(name), "."))
	name = strings.TrimSuffix(name, filepath.Ext(name))
	return layout.UniformInset(chatCardPad).Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min = gtx.Constraints.Max
		layout.NW.Layout(gtx, func(gtx C) D {
			l := th.Text(ui.TextXS, name)
			l.MaxLines = chatCardNameLines
			return l.Layout(gtx)
		})
		return layout.SW.Layout(gtx, th.Text(ui.Text2XS, ext).Weight(font.SemiBold).Muted().Layout)
	})
}
