package extrender

import (
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/ui"
)

func init() {
	register(map[string]drawFunc{
		extension.Input.Name:      drawInput,
		extension.InputGroup.Name: drawInputGroup,
		extension.Textarea.Name:   drawTextarea,
		extension.InlineEdit.Name: drawInlineEdit,
	})
}

// passwordMask is the glyph a password field draws for each rune, the one
// browsers draw for type=password.
const passwordMask = '•'

// editorState is a field's editor and what the extension knows of its text.
type editorState struct {
	editor widget.Editor
	sent   synced[string]
	// text is the text the extension last heard of, sent by it or fired to
	// it: a change event that leaves the text there is the editor echoing
	// SetText, not the person typing.
	text string
}

// take puts the extension's value in the editor when it is news and differs
// from what the field holds, so an echo of the person's own typing keeps the
// caret where it is.
func (s *editorState) take(v string) {
	if !s.sent.take(v) {
		return
	}
	if v != s.editor.Text() {
		s.editor.SetText(v)
	}
	s.text = s.editor.Text()
}

// events fires each change of the text and each submit since the last frame.
// Submit turns Enter into a submit only when the extension handles one.
func (s *editorState) events(r *Renderer, gtx C, onChange, onSubmit extension.Handler) {
	s.editor.Submit = onSubmit != ""
	for {
		ev, ok := s.editor.Update(gtx)
		if !ok {
			return
		}
		switch ev.(type) {
		case widget.ChangeEvent:
			if t := s.editor.Text(); t != s.text {
				s.text = t
				r.fire(onChange, extension.EventData{Text: t})
			}
		case widget.SubmitEvent:
			r.fire(onSubmit, extension.EventData{Text: s.editor.Text()})
		}
	}
}

type inputState struct {
	editorState
	clear widget.Clickable
}

func drawInput(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.InputProps](n)
	s := stateOf[inputState](r, id)
	s.editor.SingleLine = true
	s.editor.Mask = 0
	if p.Password {
		s.editor.Mask = passwordMask
	}
	s.take(p.Value)
	s.events(r, gtx, p.OnChange, p.OnSubmit)
	in := th.Input(&s.editor, p.Placeholder)
	in.Leading, in.LeadingSize = r.icon(p.Leading), unit.Dp(p.LeadingSize)
	in.Height, in.Width, in.TextSize = unit.Dp(p.Height), unit.Dp(p.Width), unit.Sp(p.TextSize)
	in.Mono, in.Borderless, in.Bare = p.Mono, p.Borderless, p.Bare
	in.Disabled, in.Invalid = p.Disabled, p.Invalid
	if p.Clearable {
		in.Clear = &s.clear
	}
	return in.Layout(gtx)
}

func drawInputGroup(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.InputGroupProps](n)
	s := stateOf[editorState](r, id)
	s.editor.SingleLine = !p.Multiline
	s.take(p.Value)
	s.events(r, gtx, p.OnChange, p.OnSubmit)
	g := th.InputGroup(&s.editor, p.Placeholder)
	if p.Multiline {
		g = th.InputGroupTextarea(&s.editor, p.Placeholder)
	}
	g.Width, g.Disabled, g.Invalid = unit.Dp(p.Width), p.Disabled, p.Invalid
	inner := g.Inner()
	g.Start = r.addon(inner, p.Start, id+"/start")
	g.End = r.addon(inner, p.End, id+"/end")
	return g.Layout(gtx)
}

// addon is a side of an input group; one holding a button sits closer to the
// edge, as the kit's has-[>button].
func (r *Renderer) addon(th *ui.Theme, ns []extension.Node, parent string) ui.InputGroupAddon {
	a := ui.InputGroupAddon{Items: r.nodes(th, ns, parent)}
	for _, c := range ns {
		a.Button = a.Button || c.Type == extension.Button.Name
	}
	return a
}

type textareaState struct {
	editorState
	resize ui.TextareaResize
	bar    widget.Scrollbar
}

func drawTextarea(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.TextareaProps](n)
	s := stateOf[textareaState](r, id)
	s.editor.SingleLine = false
	s.take(p.Value)
	s.events(r, gtx, p.OnChange, "")
	ta := th.Textarea(&s.editor, p.Placeholder)
	if p.Resizable {
		ta.Resize = &s.resize
	}
	if p.MaxHeight > 0 {
		ta.MaxHeight, ta.Scrollbar = unit.Dp(p.MaxHeight), &s.bar
	}
	ta.Width, ta.Disabled, ta.Invalid = unit.Dp(p.Width), p.Disabled, p.Invalid
	return ta.Layout(gtx)
}

type inlineEditState struct {
	edit ui.InlineEdit
	sent synced[string]
}

// drawInlineEdit edits Value from the frame it arrives, and again whenever
// the extension sends another; once the edit ends it draws Value as a label
// until the extension swaps the node out.
func drawInlineEdit(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.InlineEditProps](n)
	s := stateOf[inlineEditState](r, id)
	if s.sent.take(p.Value) {
		s.edit.Start(p.Value)
	}
	size := textSize(p.Size, ui.TextSM)
	if !s.edit.Active() {
		return inlineLabel(th, p, size).Layout(gtx)
	}
	st := th.InlineEdit(&s.edit, size)
	st.PadRight = unit.Dp(p.PadRight)
	if p.Mono {
		st.Font.Typeface = th.Mono
	}
	d, res := st.Layout(gtx)
	switch res.Outcome {
	case ui.InlineEditCommitted:
		r.fire(p.OnCommit, extension.EventData{Text: res.Value})
	case ui.InlineEditCancelled:
		r.fire(p.OnCancel, extension.EventData{})
	}
	return d
}

func inlineLabel(th *ui.Theme, p extension.InlineEditProps, size unit.Sp) ui.LabelStyle {
	l := th.Text(size, p.Value)
	if p.Mono {
		l = l.Mono()
	}
	return l
}
