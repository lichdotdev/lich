package markdown

import (
	"bytes"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"github.com/yuin/goldmark/ast"

	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// Alert is the kind of a GitHub alert, a blockquote opening on `[!NOTE]`.
type Alert uint8

const (
	noAlert Alert = iota
	AlertNote
	AlertTip
	AlertImportant
	AlertWarning
	AlertCaution
)

// Labels are the words a body draws of its own, read when each is drawn so a
// change of language shows at once. The window sets Words before its first
// frame.
type Labels struct {
	Alert func(a Alert) string
	More  func(count int) string
	Less  func() string
}

var Words Labels

// alertMarkers are GitHub's five, matched without regard to case as GitHub
// does.
var alertMarkers = map[string]Alert{
	"[!NOTE]": AlertNote, "[!TIP]": AlertTip, "[!IMPORTANT]": AlertImportant,
	"[!WARNING]": AlertWarning, "[!CAUTION]": AlertCaution,
}

var alertIcons = map[Alert]*icons.Icon{
	AlertNote:      icons.Lucide("info"),
	AlertTip:       icons.Lucide("lightbulb"),
	AlertImportant: icons.Lucide("message-square-warning"),
	AlertWarning:   icons.Lucide("triangle-alert"),
	AlertCaution:   icons.Lucide("octagon-alert"),
}

func (a Alert) tone(th *ui.Theme) color.NRGBA {
	switch a {
	case AlertNote:
		return th.ToneNote
	case AlertTip:
		return th.TonePass
	case AlertImportant:
		return th.ToneImportant
	case AlertWarning:
		return th.ToneWait
	case AlertCaution:
		return th.Destructive
	}
	return th.Border
}

// takeAlert is the alert n opens with, its marker line taken out of n. A
// quote holding nothing but the marker is a quote, as on GitHub.
func takeAlert(source []byte, n *ast.Blockquote) Alert {
	p, ok := n.FirstChild().(*ast.Paragraph)
	if !ok || p.Lines().Len() == 0 {
		return noAlert
	}
	first := p.Lines().At(0)
	alert := alertMarkers[string(bytes.ToUpper(bytes.TrimSpace(first.Value(source))))]
	if alert == noAlert || (p.Lines().Len() == 1 && p.NextSibling() == nil) {
		return noAlert
	}
	for c := p.FirstChild(); c != nil; {
		next := c.NextSibling()
		if t, isText := c.(*ast.Text); isText && t.Segment.Start < first.Stop {
			p.RemoveChild(p, c)
		}
		c = next
	}
	if p.FirstChild() == nil {
		n.RemoveChild(n, p)
	}
	return alert
}

// alertHeadGap is the room, in ui.Space steps, between an alert's name and
// its text.
const alertHeadGap = 0.5

// layoutAlertHead is the alert's icon and name in its tone.
func layoutAlertHead(gtx C, th *ui.Theme, a Alert) D {
	tone := a.tone(th)
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return alertIcons[a].Layout(gtx, markSize, tone) }),
		layout.Rigid(layout.Spacer{Width: ui.Space(1.5)}.Layout),
		layout.Rigid(th.Text(ui.TextSM, Words.Alert(a)).In(tone).Weight(font.SemiBold).Layout),
	)
}
