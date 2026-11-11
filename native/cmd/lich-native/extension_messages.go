package main

import (
	"context"
	"image/color"
	"strings"
	"sync"
	"time"

	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// extensionSend is one line for one extension's process.
type extensionSend struct {
	id  string
	msg extension.Message
}

// extensionOutbox carries what the window sends extensions off the frame, one
// line at a time in the order it was sent: a field's keystrokes arrive as the
// person typed them.
type extensionOutbox struct {
	mu    sync.Mutex
	queue []extensionSend
	wake  chan struct{}
}

func newExtensionOutbox() *extensionOutbox {
	return &extensionOutbox{wake: make(chan struct{}, 1)}
}

func (o *extensionOutbox) send(id string, msg extension.Message) {
	o.mu.Lock()
	o.queue = append(o.queue, extensionSend{id, msg})
	o.mu.Unlock()
	select {
	case o.wake <- struct{}{}:
	default:
	}
}

// run delivers until ctx ends; a line deliver refuses is reported to failed
// and the next one still goes.
func (o *extensionOutbox) run(ctx context.Context, deliver func(context.Context, string, extension.Message) error, failed func(error)) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-o.wake:
		}
		o.mu.Lock()
		batch := o.queue
		o.queue = nil
		o.mu.Unlock()
		for _, s := range batch {
			if err := deliver(ctx, s.id, s.msg); err != nil {
				failed(err)
			}
		}
	}
}

// fireExtension calls handler h of extension id with nothing but its name.
func fireExtension(send func(string, extension.Message), id string, h extension.Handler) func() {
	if h == "" {
		return nil
	}
	return func() { send(id, extension.FireEvent.New(extension.EventData{Handler: h})) }
}

// extensionToast is an extension's toast as the window's toaster draws it.
// Its ID is kept apart from lich's own toasts' and from other extensions'.
func extensionToast(id string, t extension.ToastData, send func(string, extension.Message)) ui.Toast {
	out := ui.Toast{
		Variant: toastVariants[t.Variant], Title: t.Title, Description: t.Description,
		Action:    toastButton(id, t.Action, send),
		Cancel:    toastButton(id, t.Cancel, send),
		Secondary: toastButton(id, t.Secondary, send),
		Icon:      lookupIcon(t.Icon), Spin: t.Spin,
		Duration:  toastDuration(t.DurationMs),
		OnDismiss: fireExtension(send, id, t.OnDismiss),
	}
	if t.ID != "" {
		out.ID = extensionToastID(id, t.ID)
	}
	if t.IconColor != "" {
		name := t.IconColor
		out.IconColor = func(th *ui.Theme) color.NRGBA { return themeColor(th, name) }
	}
	for _, l := range t.Lines {
		out.Lines = append(out.Lines, ui.ToastLine{Icon: lookupIcon(l.Icon), Text: l.Text})
	}
	if t.Progress != nil {
		out.Progress = &ui.ToastProgress{Percent: t.Progress.Percent}
	}
	return out
}

func extensionToastID(ext, id string) string { return "extension:" + ext + ":" + id }

var toastVariants = map[extension.ToastVariant]ui.ToastVariant{
	extension.ToastDefault: ui.ToastDefault, extension.ToastSuccess: ui.ToastSuccess,
	extension.ToastInfo: ui.ToastInfo, extension.ToastWarning: ui.ToastWarning, extension.ToastError: ui.ToastError,
}

func toastButton(id string, b *extension.ToastButton, send func(string, extension.Message)) *ui.ToastButton {
	if b == nil {
		return nil
	}
	return &ui.ToastButton{Label: b.Label, OnClick: fireExtension(send, id, b.OnPress)}
}

func toastDuration(ms int) time.Duration {
	if ms == extension.ToastForever {
		return ui.ToastForever
	}
	return time.Duration(max(ms, 0)) * time.Millisecond
}

// lookupIcon is the Lucide glyph an extension names, nil for none or one the
// set lacks: the name comes from outside lich.
func lookupIcon(name extension.Icon) *icons.Icon {
	if name == "" {
		return nil
	}
	ic, _ := icons.LookupLucide(string(name))
	return ic
}

// themeColor is a token's color in th or a hex value, the foreground for
// anything else; the names are the ones a tree's colors take (extrender).
func themeColor(th *ui.Theme, name string) color.NRGBA {
	tokens := map[string]color.NRGBA{
		"background": th.Background, "foreground": th.Foreground, "canvas": th.Canvas,
		"card": th.Card, "card-foreground": th.CardForeground,
		"popover": th.Popover, "popover-foreground": th.PopoverForeground,
		"primary": th.Primary, "primary-foreground": th.PrimaryForeground,
		"secondary": th.Secondary, "secondary-foreground": th.SecondaryForeground,
		"muted": th.Muted, "muted-foreground": th.MutedForeground,
		"accent": th.Accent, "accent-foreground": th.AccentForeground,
		"destructive": th.Destructive, "tone-pass": th.TonePass, "tone-wait": th.ToneWait,
		"border": th.Border, "input": th.Palette.Input, "ring": th.Palette.Ring,
		"sidebar": th.Sidebar, "sidebar-foreground": th.SidebarForeground,
		"sidebar-accent": th.SidebarAccent, "sidebar-accent-foreground": th.SidebarAccentForeground,
		"sidebar-border": th.SidebarBorder,
	}
	if c, ok := tokens[name]; ok {
		return c
	}
	if strings.HasPrefix(name, "#") {
		if c, err := ui.ParseColor(name); err == nil {
			return c
		}
	}
	return th.Foreground
}

// extensionMessage carries out what an extension asked the window for.
func (u *window) extensionMessage(id string, msg extension.Message) {
	switch d := msg.Data.(type) {
	case extension.ToastData:
		u.toaster.Show(extensionToast(id, d, u.extOutbox.send))
	case extension.DismissToastData:
		u.toaster.Dismiss(extensionToastID(id, d.ID))
	case extension.OpenExternalData:
		go func() { u.logErr("open an extension's link", u.client.OpenExternal(u.ctx, d.URL)) }()
	case extension.ClipboardData:
		u.extension.copy(d.Text)
	}
	u.model.invalidate()
}
