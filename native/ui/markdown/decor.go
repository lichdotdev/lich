package markdown

import (
	"image"
	"image/color"
	"net/url"
	"regexp"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/x/styledtext"

	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// iconTone is the color a span's mark is drawn in.
type iconTone uint8

const (
	iconMuted iconTone = iota // a file or a folder: MutedForeground
	iconLink                  // a link's mark: the link's color
)

// decoration is what a span is drawn with besides its text: an underline, for a
// link, and a mark in room the text leaves before it. path is the path the
// span's code spells, "" for any other span.
type decoration struct {
	underline bool
	icon      *icons.Icon
	tone      iconTone
	path      string
}

const (
	// codeIconRoom is the room two cells of the mono face leave for the mark
	// before a path in code; linkIconRoom, an em and a thin space, the room before a link.
	codeIconRoom = "\u00a0\u00a0"
	linkIconRoom = "\u2003\u2009"
	// markSize is the mark's side.
	markSize = 13
)

var (
	icFile   = icons.Lucide("file")
	icFolder = icons.Lucide("folder")
	icGitHub = icons.Brand("github")
)

// linkMark is the mark a link to url wears, nil for most: GitHub's for a link to
// github.com.
func linkMark(raw string) *icons.Icon {
	u, err := url.Parse(raw)
	if err != nil {
		return nil
	}
	if host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www."); host == "github.com" {
		return icGitHub
	}
	return nil
}

// pathLike is what a path written in code looks like: no spaces, made of what
// a path is made of, and either with a slash or ending in a short extension.
var pathLike = regexp.MustCompile(`^[~.]{0,2}/?[\w@.+-]+(/[\w@.+-]+)*/?$`)

var fileExtension = regexp.MustCompile(`\.[A-Za-z][A-Za-z0-9]{0,7}$`)

// pathIcon is the folder or file mark for code that spells one path, nil for
// any other code.
func pathIcon(t string) *icons.Icon {
	switch {
	case t == "" || !pathLike.MatchString(t) || strings.Contains(t, "://"):
		return nil
	case strings.HasSuffix(t, "/"):
		return icFolder
	case strings.HasPrefix(t, "/") && !strings.Contains(t[1:], "/") && !fileExtension.MatchString(t):
		// a slash and one bare word is how a slash command is written: /compact
		return nil
	case strings.Contains(t, "/") || fileExtension.MatchString(t):
		return icFile
	}
	return nil
}

// decorate draws what is under or beside the text of each span: the ground of
// inline code, a link's underline and the marks of paths and of GitHub links,
// and gives a path that paths links the pointer. The text is laid out
// invisibly to learn where each span stands.
func (c *markdownCell) decorate(gtx C, th *ui.Theme, paths *Paths) {
	drawn := false
	for i, code := range c.code {
		drawn = drawn || code || (i < len(c.decor) && (c.decor[i].underline || c.decor[i].icon != nil))
	}
	if !drawn {
		return
	}
	spans := make([]styledtext.SpanStyle, len(c.spans))
	for i, span := range c.spans {
		spans[i] = styledtext.SpanStyle{Font: span.Font, Size: span.Size, Color: color.NRGBA{}, Content: span.Content}
	}
	style := styledtext.Text(th.Shaper, spans...)
	style.LineHeightScale = c.lineHeightScale()
	style.Layout(gtx, func(gtx layout.Context, i int, dims layout.Dimensions) {
		if dims.Size.X == 0 || dims.Size.Y == 0 {
			return
		}
		if c.code[i] {
			rect := image.Rectangle{Max: dims.Size}.Inset(gtx.Dp(1))
			paint.FillShape(gtx.Ops, codeFill(th), clip.UniformRRect(rect, gtx.Dp(ui.RadiusSM)).Op(gtx.Ops))
		}
		if i >= len(c.decor) {
			return
		}
		d, link := c.decor[i], c.spans[i].Color
		if d.path != "" && paths != nil && paths.Link(d.path) && c.followPath(gtx, i, dims.Size, paths) {
			d.underline = true
		}
		if d.underline {
			y := dims.Size.Y - gtx.Dp(3)
			paint.FillShape(gtx.Ops, link, clip.Rect{Min: image.Pt(0, y), Max: image.Pt(dims.Size.X, y+gtx.Dp(1))}.Op())
		}
		if d.icon != nil {
			tone := th.MutedForeground
			if d.tone == iconLink {
				tone = link
			}
			side := gtx.Dp(markSize)
			at := op.Offset(image.Pt(gtx.Dp(2), (dims.Size.Y-side)/2)).Push(gtx.Ops)
			d.icon.Layout(gtx, gtx.Metric.PxToDp(side), tone)
			at.Pop()
		}
	})
}
