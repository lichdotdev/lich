package main

import (
	"fmt"
	"log"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// ponytail: English until the native window has i18n (settings.providerBinary).
const binPickFailed = "Could not open the file picker: %v"

var (
	icBinBrowse = icons.Lucide("folder-open")
	icBinOK     = icons.Lucide("check")
	icBinFailed = icons.Lucide("x")
	icBinWarn   = icons.Lucide("triangle-alert")
)

// binGlyphSize is the verdict's size-3.5.
var binGlyphSize = ui.Space(3.5)

// binInput is the Binary block's path field. The field and the stored path
// are one value: what is typed is written on every keystroke, and a stored
// value that moved under it (Use Global, another provider) is put in the
// field.
type binInput struct {
	editor  widget.Editor
	browse  widget.Clickable
	started bool
	// provider, scope and text are what the field was last made equal to.
	provider, scope, text string
}

// sync reconciles the field with stored, the path of provider's layer in
// scope. It reports the text the user just typed, when they did.
func (b *binInput) sync(provider, scope, stored string) (typed string, edited bool) {
	if !b.started {
		b.editor.SingleLine, b.started = true, true
	}
	if b.provider != provider || b.scope != scope {
		b.provider, b.scope, b.text = provider, scope, stored
		b.editor.SetText(stored)
		return "", false
	}
	if cur := b.editor.Text(); cur != b.text {
		b.text = cur
		return cur, true
	}
	if stored != b.text {
		b.text = stored
		b.editor.SetText(stored)
	}
	return "", false
}

// projectOffOf is the project's stored switch parking its binary, read once
// per visit: it is not a setting the pane lists, so loadScopedValues does not
// read it.
func (p *providerSettings) projectOffOf(s *settingsScreen, scope, kind string) bool {
	if scope == "" {
		return false
	}
	key := scope + "\x00" + kind
	p.mu.Lock()
	defer p.mu.Unlock()
	if v, ok := p.projectOff[key]; ok {
		return v == "true"
	}
	if !p.askedOff[key] {
		if p.askedOff == nil {
			p.askedOff = map[string]bool{}
		}
		p.askedOff[key] = true
		go func() {
			v, err := s.client.GetSetting(s.ctx, providerBinOffKey(kind), scope)
			if err != nil {
				log.Printf("settings: read %s: %v", providerBinOffKey(kind), err)
				return
			}
			p.mu.Lock()
			if p.projectOff == nil {
				p.projectOff = map[string]string{}
			}
			p.projectOff[key] = v
			p.mu.Unlock()
			s.invalidate()
		}()
	}
	return false
}

// check is what bin resolves to as a spawn would, ok false until the backend
// answered. One question per distinct string, so a path typed one letter at a
// time is asked once per letter and never twice.
func (p *providerSettings) check(s *settingsScreen, bin string) (lichclient.BinaryCheck, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if c, ok := p.checks[bin]; ok {
		return c, true
	}
	if !p.asked[bin] {
		if p.asked == nil {
			p.asked = map[string]bool{}
		}
		p.asked[bin] = true
		go func() {
			c, err := s.client.Verify(s.ctx, bin)
			if err != nil {
				log.Printf("settings: verify %s: %v", bin, err)
				return
			}
			p.mu.Lock()
			if p.checks == nil {
				p.checks = map[string]lichclient.BinaryCheck{}
			}
			p.checks[bin] = c
			p.mu.Unlock()
			s.invalidate()
		}()
	}
	return lichclient.BinaryCheck{}, false
}

// writeBin stores the path of provider's layer in scope. A path typed or
// picked is meant to be used, so a layer parked by the web window is put back
// with it: this window has no switch to do that with.
func (s *settingsScreen) writeBin(m providersMachine, kind, scope, value string) {
	s.writeProviderKey(providerBinKey(kind), scope, value, s.providersChanged)
	if value == "" {
		return
	}
	offKey := providerBinOffKey(kind)
	p := &s.providers
	switch {
	case scope == "" && m.globals[offKey] == "true":
		s.writeProviderKey(offKey, "", "", s.providersChanged)
	case scope != "" && p.projectOffOf(s, scope, kind):
		s.writeProviderKey(offKey, scope, "", s.providersChanged)
		p.mu.Lock()
		p.projectOff[scope+"\x00"+kind] = ""
		p.mu.Unlock()
	}
}

func (s *settingsScreen) pickBin(m providersMachine, p providerInfo, scope string) {
	file, err := s.client.PickFile(s.ctx, fmt.Sprintf(binPickTitle, p.name))
	if err != nil {
		log.Printf("settings: pick %s binary: %v", p.id, err)
		s.theme.toaster.Show(failure(binPickFailed, err))
		return
	}
	if file != "" {
		s.writeBin(m, p.id, scope, file)
		s.invalidate()
	}
}

// binBlock is ProviderBinary, simplified to one row: the path of the open
// scope's layer, a picker, and what a session would spawn. The project's
// layer and Global's are the scope selector's, not two fields.
func (s *settingsScreen) binBlock(gtx C, th *ui.Theme, values scopedValues, m providersMachine, p providerInfo, first bool) D {
	pr := &s.providers
	e := settingEntry{title: binTitle, key: providerBinKey(p.id), scope: scopeOverridable}
	r := s.providerRow(gtx, e, values)
	name := s.scopeName()
	r.description = binDesc
	if s.scope != "" {
		r.description = fmt.Sprintf(binDescProject, name)
	}
	r.globalValue = m.globals[providerBinKey(p.id)]
	if r.globalValue == "" {
		r.globalValue = binGlobalPath
	}

	global := binLayer{bin: m.globals[providerBinKey(p.id)], off: m.globals[providerBinOffKey(p.id)] == "true"}
	var project binLayer
	stored := global.bin
	if s.scope != "" {
		stored, _ = values.set(providerBinKey(p.id), s.scope)
		project = binLayer{bin: stored, off: pr.projectOffOf(s, s.scope, p.id)}
	}
	if typed, edited := pr.bin.sync(p.id, s.scope, stored); edited {
		s.writeBin(m, p.id, s.scope, typed)
		if s.scope == "" {
			global.bin = typed
		} else {
			project.bin = typed
		}
	}
	if pr.bin.browse.Clicked(gtx) {
		go s.pickBin(m, p, s.scope)
	}

	scope := winningBinScope(project, global)
	configured := configuredBin(scope, project, global)
	asked := configured
	if asked == "" {
		asked = p.binary
	}
	check, known := pr.check(s, asked)
	source := parkedLabel(scope, project, global, name)
	return r.layoutBlock(gtx, th, first, func(gtx C, th *ui.Theme) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return s.binField(gtx, th, p) }),
			ui.VGap(2),
			layout.Rigid(func(gtx C) D {
				return binVerdict(gtx, th, check, known, asked, configured, binSource(scope, name)+source)
			}),
			layout.Rigid(func(gtx C) D { return binProblem(gtx, th, check, known, asked) }),
		)
	})
}

func (s *settingsScreen) binField(gtx C, th *ui.Theme, p providerInfo) D {
	b := &s.providers.bin
	field := th.Input(&b.editor, p.binary)
	field.Mono, field.Height, field.TextSize = true, ui.Space(8), ui.TextXS
	browse := th.Button(&b.browse, binBrowse)
	browse.Variant, browse.Size, browse.Icon = ui.VariantGhost, ui.SizeSM, icBinBrowse
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, field.Layout),
		ui.Gap(2),
		layout.Rigid(browse.Layout),
	)
}

// binSource names where the winning path came from, in the line under the
// field where the layers themselves are not on screen.
func binSource(scope, projectName string) string {
	switch scope {
	case binScopeProject:
		if projectName == "" {
			return binSourceThis
		}
		return fmt.Sprintf(binSourceName, projectName)
	case binScopeGlobal:
		return binSourceGlobal
	}
	return binSourcePath
}

// binVerdict is the glyph of what the path resolves to, the executable it
// resolved to and where it came from. Nothing is drawn for the glyph while a
// check is in flight, so a field being typed into does not blink.
func binVerdict(gtx C, th *ui.Theme, check lichclient.BinaryCheck, known bool, asked, configured, source string) D {
	path := check.Path
	if path == "" {
		path = configured
	}
	if path == "" {
		path = asked
	}
	children := []layout.FlexChild{}
	switch {
	case known && check.Failed():
		children = append(children, layout.Rigid(func(gtx C) D { return icBinFailed.Layout(gtx, binGlyphSize, th.Destructive) }), ui.Gap(1.5))
	case known && check.Status == lichclient.CheckOK:
		children = append(children, layout.Rigid(func(gtx C) D { return icBinOK.Layout(gtx, binGlyphSize, th.TonePass) }), ui.Gap(1.5))
	}
	children = append(children,
		layout.Rigid(th.Text(ui.TextXS, path).Mono().Layout),
		ui.Gap(2),
		layout.Flexed(1, th.Text(ui.TextXS, "· "+source).Muted().Layout),
	)
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

// binProblem is the sentence under a binary a session cannot start from: what
// is wrong and what will happen.
func binProblem(gtx C, th *ui.Theme, check lichclient.BinaryCheck, known bool, asked string) D {
	if !known || !check.Failed() {
		return D{}
	}
	text := binCheckLabel(check, asked) + " — " + binCheckDetail(check)
	return layout.Inset{Top: ui.Space(3)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: unit.Dp(2)}.Layout(gtx, func(gtx C) D { return icBinWarn.Layout(gtx, binGlyphSize, th.Destructive) })
			}),
			ui.Gap(2),
			layout.Flexed(1, func(gtx C) D { return wrapped(gtx, th.Text(ui.TextXS, text).In(th.Destructive)) }),
		)
	})
}
