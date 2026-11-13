package main

import (
	"image"
	"io"
	"math"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var (
	icDockArtifacts  = icons.Lucide("layers")
	icArtifactPage   = icons.Lucide("file-code")
	icArtifactImage  = icons.Lucide("image")
	icArtifactFile   = icons.Lucide("file")
	icArtifactAttach = icons.Lucide("clipboard-paste")
)

// The tab's rows, from the approved mockup: a file row is a sidebar row's
// 34dp, a session's line under Other sessions a shorter 30dp, and the unread
// dot 6dp across.
var (
	artifactRowHeight     = unit.Dp(34)
	artifactSessionHeight = unit.Dp(30)
	artifactDotRadius     = unit.Dp(3)
)

// artifactsTab is the dock's Artifacts tab: the pages the active session's
// agent left for the person, then those of the other open sessions of its
// project. The window hands it what it reads (store, peers) and what its rows
// do (open, attach, reveal).
type artifactsTab struct {
	store  *artifactStore
	peers  func(id string) []backend.Session
	open   func(session, from, name string)
	attach func(target, from, name string)
	reveal func(dir string)

	list     widget.List
	rows     map[string]*artifactRowState
	presence map[string]*ui.Presence
	copyText string
}

type artifactRowState struct {
	click, more   widget.Clickable
	context, menu ui.MenuState
	tip           ui.Tooltip
}

func newArtifactsTab(store *artifactStore) *artifactsTab {
	t := &artifactsTab{store: store, rows: map[string]*artifactRowState{}, presence: map[string]*ui.Presence{}}
	t.list.Axis = layout.Vertical
	return t
}

// holdsKeys is whether a row's menu is open, which takes the keys.
func (t *artifactsTab) holdsKeys() bool {
	for _, r := range t.rows {
		if r.menu.Opened() || r.context.Opened() {
			return true
		}
	}
	return false
}

// count is the tab's label count for session active.
func (t *artifactsTab) count(active string) int {
	return artifactCount(t.groups(active))
}

func (t *artifactsTab) groups(active string) ([]backend.Artifact, []artifactGroup) {
	if active == "" {
		return nil, nil
	}
	return artifactGroups(active, t.peers(active), t.store.of)
}

func (t *artifactsTab) row(key string) *artifactRowState {
	r, ok := t.rows[key]
	if !ok {
		r = new(artifactRowState)
		t.rows[key] = r
	}
	return r
}

func (t *artifactsTab) motion(owner string) *ui.Presence {
	p, ok := t.presence[owner]
	if !ok {
		p = new(ui.Presence)
		t.presence[owner] = p
	}
	return p
}

func (t *artifactsTab) layout(gtx C, th *ui.Theme, root *ui.Root, active string) D {
	own, others := t.groups(active)
	if len(own) == 0 && len(others) == 0 {
		return th.Notice(i18n.T("dock.artifacts.empty")).Layout(gtx)
	}
	var items []layout.Widget
	add := func(w layout.Widget) { items = append(items, w) }
	if len(own) > 0 {
		add(func(gtx C) D { return artifactGroupLabel(gtx, th, i18n.T("dock.artifacts.thisSession")) })
		t.files(gtx, th, root, active, active, own, add)
	}
	if len(others) > 0 {
		add(func(gtx C) D { return artifactGroupLabel(gtx, th, i18n.T("dock.artifacts.otherSessions")) })
		for _, g := range others {
			add(func(gtx C) D { return artifactSessionLine(gtx, th, g.session) })
			t.files(gtx, th, root, active, g.session.ID, g.files, add)
		}
	}
	rows := th.ScrollArea(&t.list)
	rows.AnchorStrategy = material.Overlay
	d := layout.UniformInset(ui.Space(2)).Layout(gtx, func(gtx C) D {
		return rows.Layout(gtx, len(items), func(gtx C, i int) D { return items[i](gtx) })
	})
	if t.copyText != "" {
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(t.copyText))})
		t.copyText = ""
	}
	return d
}

// files adds owner's rows, each entering in its Presence; a row leaving
// is dropped at once, its data gone with the list.
func (t *artifactsTab) files(gtx C, th *ui.Theme, root *ui.Root, active, owner string, files []backend.Artifact, add func(layout.Widget)) {
	byName := make(map[string]backend.Artifact, len(files))
	names := make([]string, len(files))
	for i, f := range files {
		byName[f.Name], names[i] = f, f.Name
	}
	p := t.motion(owner)
	for _, name := range p.Update(gtx.Now, names) {
		f, ok := byName[name]
		if !ok {
			continue
		}
		add(func(gtx C) D {
			return p.Layout(gtx, name, func(gtx C) D { return t.fileRow(gtx, th, root, active, owner, f) })
		})
	}
}

// fileRow is one artifact: its glyph in the node column, its name with the
// unread dot, how long ago it was written, and the corner slot's ⋯. A click
// opens it in the active session's browser.
func (t *artifactsTab) fileRow(gtx C, th *ui.Theme, root *ui.Root, active, owner string, f backend.Artifact) D {
	r := t.row(owner + "\x00" + f.Name)
	do := t.actions(active, owner, f)
	if r.click.Clicked(gtx) {
		do.open()
	}
	entries := func() []ui.MenuEntry { return artifactMenu(owner == active, do) }
	more := func(gtx C) D {
		return th.DropdownMenu(root, &r.menu, entries()).Layout(gtx, func(gtx C) D {
			return th.Tooltip(root, &r.tip, i18n.T("dock.artifacts.options", i18n.Args{"name": f.Name})).Layout(gtx, func(gtx C) D {
				return th.CornerButton(&r.more, icGroupMore, r.menu.Opened()).Layout(gtx)
			})
		})
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return th.ContextMenuLazy(root, &r.context, entries).Layout(gtx, func(gtx C) D {
		return r.click.Layout(gtx, func(gtx C) D {
			hovered := r.click.Hovered() || r.menu.Opened() || r.context.Opened()
			h := gtx.Dp(artifactRowHeight)
			gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = h, h
			ui.Fill(gtx, cardLayer(th, "", false, hovered), ui.RadiusMD)
			unread := t.store.isUnread(owner, f.Name)
			return layout.Inset{Left: rowInset, Right: ui.Space(1)}.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.Y = h
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(inNodeColumn(func(gtx C) D { return artifactIcon(f.Name).Layout(gtx, ui.Space(3.5), th.MutedForeground) })),
					ui.Gap(1.5),
					layout.Flexed(1, func(gtx C) D { return artifactName(gtx, th, f.Name, unread) }),
					ui.Gap(1.5),
					layout.Rigid(func(gtx C) D { return artifactAge(gtx, th, f.Modified) }),
					ui.Gap(1),
					layout.Rigid(func(gtx C) D {
						size := gtx.Dp(cardSlotSize)
						gtx.Constraints = layout.Exact(image.Pt(size, size))
						if !hovered {
							return D{Size: gtx.Constraints.Min}
						}
						return layout.Center.Layout(gtx, more)
					}),
				)
			})
		})
	})
}

// artifactActions is what a row's click and menu entries do.
type artifactActions struct {
	open, attach, copy, reveal func()
}

func (t *artifactsTab) actions(active, owner string, f backend.Artifact) artifactActions {
	return artifactActions{
		open: func() {
			t.store.markRead(owner, f.Name)
			t.open(active, owner, f.Name)
		},
		attach: func() { t.attach(active, owner, f.Name) },
		copy:   func() { t.copyText = f.Path },
		reveal: func() { t.reveal(filepath.Dir(f.Path)) },
	}
}

// artifactMenu is a row's ⋯ and right click. The active session's own file
// opens first; another session's is there to be handed over, so Attach to
// prompt leads.
func artifactMenu(own bool, do artifactActions) []ui.MenuEntry {
	open := ui.MenuEntry{Label: i18n.T("dock.artifacts.openInBrowser"), Icon: icBrowser, OnClick: do.open}
	attach := ui.MenuEntry{Label: i18n.T("dock.artifacts.attachToPrompt"), Icon: icArtifactAttach, OnClick: do.attach}
	first, second := open, attach
	if !own {
		first, second = attach, open
	}
	return []ui.MenuEntry{
		first, second,
		{Kind: ui.MenuSeparator},
		{Label: i18n.T("dock.artifacts.copyPath"), Icon: icMenuCopy, OnClick: do.copy},
		{Label: i18n.T("dock.artifacts.showInFileManager"), Icon: icOpenIn, OnClick: do.reveal},
	}
}

func artifactIcon(name string) *icons.Icon {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".html", ".htm", "":
		return icArtifactPage
	case ".svg", ".png", ".jpg", ".jpeg", ".gif", ".webp":
		return icArtifactImage
	}
	return icArtifactFile
}

// artifactName is the name cut to the column, with the unread dot right after
// it: the dot's room is kept first, so a long name never pushes it out.
func artifactName(gtx C, th *ui.Theme, name string, unread bool) D {
	full := gtx.Constraints.Max.X
	dot := 0
	if unread {
		dot = gtx.Dp(ui.Space(2)) + 2*gtx.Dp(artifactDotRadius)
	}
	ngtx := gtx
	ngtx.Constraints = layout.Constraints{Max: image.Pt(max(full-dot, 0), gtx.Constraints.Max.Y)}
	d := th.Text(ui.TextSM, name).Layout(ngtx)
	if unread {
		r := float32(gtx.Dp(artifactDotRadius))
		center := f32.Pt(float32(d.Size.X+gtx.Dp(ui.Space(2)))+r, float32(d.Size.Y)/2)
		ui.StrokeArc(gtx, center, r/2, r, 0, 2*math.Pi, th.TonePass)
	}
	return D{Size: image.Pt(full, d.Size.Y), Baseline: d.Baseline}
}

// artifactAge is how long ago the file was written, asking for the frame
// the readout moves on.
func artifactAge(gtx C, th *ui.Theme, modified int64) D {
	elapsed := gtx.Now.Sub(time.UnixMilli(modified))
	gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(nextAgeChange(elapsed))})
	return th.Text(ui.TextXS, formatAge(elapsed)).Mono().Muted().Layout(gtx)
}

// artifactGroupLabel is a group's uppercase label and the rule to the row's
// end, in the node column's start.
func artifactGroupLabel(gtx C, th *ui.Theme, text string) D {
	heading := th.SectionHeading(text)
	heading.Size, heading.Weight, heading.Leading = ui.Text2XS, font.SemiBold, text2XSLeading
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Inset{Top: ui.Space(2.5), Bottom: ui.Space(1), Left: ui.Space(3.5), Right: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(heading.Layout),
			ui.Gap(2),
			layout.Flexed(1, func(gtx C) D {
				gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(1)))
				return ui.Fill(gtx, th.Over(th.Border), 0)
			}),
		)
	})
}

// artifactSessionLine names the session the rows under it came from: its
// provider mark in the node column and its name.
func artifactSessionLine(gtx C, th *ui.Theme, s backend.Session) D {
	h := gtx.Dp(artifactSessionHeight)
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = h, h
	return layout.Inset{Left: rowInset, Right: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.Y = h
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(inNodeColumn(func(gtx C) D { return providerIcon(s.Kind).Layout(gtx, ui.Space(3.5), th.MutedForeground) })),
			ui.Gap(1.5),
			layout.Flexed(1, th.Text(ui.TextXS, s.Label).Muted().Layout),
		)
	})
}
