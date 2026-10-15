package main

import (
	"fmt"
	"image"
	"log"
	"os/exec"
	"slices"
	"strings"
	"sync"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// ponytail: English until the native window has i18n (settings.fontSetting).
const (
	fontTitle  = "Terminal font"
	fontAlso   = "typeface monospace"
	fontSample = "the quick brown fox 0O1lI"
	fontSearch = "Search fonts"
	fontEmpty  = "No font matches."
	fontSystem = "System"
)

var (
	fontTriggerWidth = ui.Space(64) // w-64
	fontListHeight   = ui.Space(72) // ComboboxList's max-h-72
	fontRowHeight    = ui.Space(8)  // the item's py-1.5 text-sm
	icCheck          = icons.Lucide("check")
)

// fontPicker is FontSetting's combobox: System, then the families list
// finds, a search over them, and a sample of the chosen one drawn in it. A
// chosen family is stored by name, System as "".
type fontPicker struct {
	// families finds the ones offered; faces loads one, system being the
	// fontconfig alias System draws in.
	find   func() ([]string, error)
	faces  func(family string) ([]font.FontFace, error)
	system string

	panel  ui.DropdownPanelState
	search widget.Editor
	list   widget.List
	rows   map[string]*widget.Clickable

	mu       sync.Mutex
	families []string
	asked    bool
	// shapers draw the sample in a family, built the first time it is shown.
	shapers map[string]*familyShaper
}

type familyShaper struct {
	shaper *text.Shaper
	face   font.Font
}

// monoFamilies lists the installed families fontconfig marks monospace or
// dual-width (the Nerd Font families that are not "Mono"), which a terminal
// can draw on its grid. The native window draws terminals through
// fontconfig too (gioterm), so this is the roster it can render.
func monoFamilies() ([]string, error) {
	seen := map[string]bool{}
	for _, spacing := range []string{":spacing=mono", ":spacing=dual"} {
		out, err := exec.Command("fc-list", spacing, "family").Output()
		if err != nil {
			return nil, fmt.Errorf("fc-list %s: %w", spacing, err)
		}
		for _, name := range parseFamilyLines(string(out)) {
			seen[name] = true
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	slices.Sort(names)
	return names, nil
}

// parseFamilyLines reads fc-list's family lines: a line may hold
// comma-separated localized aliases, and the first is the family's name.
func parseFamilyLines(out string) []string {
	var names []string
	for line := range strings.SplitSeq(out, "\n") {
		if name := strings.TrimSpace(strings.SplitN(line, ",", 2)[0]); name != "" {
			names = append(names, name)
		}
	}
	return names
}

// uiFamilies lists the bundled Onest and every installed family, for the
// interface's font.
func uiFamilies() ([]string, error) {
	out, err := exec.Command("fc-list", ":", "family").Output()
	if err != nil {
		return nil, fmt.Errorf("fc-list: %w", err)
	}
	names := parseFamilyLines(string(out))
	slices.Sort(names)
	return append([]string{onestFamily}, slices.Compact(names)...), nil
}

// fontOptions are the choices offered: System, the current family even when
// fontconfig does not list it, then the rest, those matching query.
func fontOptions(current string, families []string, query string) []string {
	needle := strings.ToLower(strings.TrimSpace(query))
	var out []string
	for _, f := range append([]string{"", current}, families...) {
		if slices.Contains(out, f) || !strings.Contains(strings.ToLower(fontLabel(f)), needle) {
			continue
		}
		out = append(out, f)
	}
	return out
}

// fontLabel is how a stored family reads: "" is System.
func fontLabel(family string) string {
	if family == "" {
		return fontSystem
	}
	return family
}

// load asks fontconfig for the families once, the first time the picker is
// drawn, and keeps the answer for the next visit as the web does.
func (f *fontPicker) load(invalidate func()) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.asked {
		f.asked = true
		go func() {
			families, err := f.find()
			if err != nil {
				log.Printf("settings: list fonts: %v", err)
			}
			f.mu.Lock()
			f.families = families
			f.mu.Unlock()
			invalidate()
		}()
	}
	return f.families
}

// sample is the row's description: the sample line in family itself, the
// answer to "which one is this" being the shape of the glyphs.
func (f *fontPicker) sample(th *ui.Theme, family string) layout.Widget {
	return func(gtx C) D {
		fs, err := f.shaperFor(family)
		if err != nil {
			return th.Text(rowDescSize, fontSample).Muted().Layout(gtx)
		}
		macro := op.Record(gtx.Ops)
		paint.ColorOp{Color: th.MutedForeground}.Add(gtx.Ops)
		ink := macro.Stop()
		return widget.Label{MaxLines: 1}.Layout(gtx, fs.shaper, fs.face, rowDescSize, fontSample, ink)
	}
}

// shaperFor loads family's faces the first time its sample is drawn.
// ponytail: on the frame that first shows it, a fontconfig lookup and a
// parse; a background load waits for a family slow enough to be felt.
func (f *fontPicker) shaperFor(family string) (*familyShaper, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if fs, ok := f.shapers[family]; ok {
		return fs, nil
	}
	faces, err := f.faces(systemOr(family, f.system))
	if err != nil || len(faces) == 0 {
		return nil, fmt.Errorf("load %s: %v", family, err)
	}
	if f.shapers == nil {
		f.shapers = map[string]*familyShaper{}
	}
	fs := &familyShaper{shaper: text.NewShaper(text.NoSystemFonts(), text.WithCollection(faces)), face: faces[0].Font}
	f.shapers[family] = fs
	return fs, nil
}

// fontControl is the combobox: the outline trigger naming the family, and
// the panel with the search and the list under it; choose takes the family
// picked.
func (s *settingsScreen) fontControl(gtx C, th *ui.Theme, f *fontPicker, current string, choose func(string)) D {
	families := f.load(s.invalidate)
	pop := th.DropdownPanel(s.root, &f.panel, fontTriggerWidth, func(gtx C, th *ui.Theme) D {
		return s.fontPanel(gtx, th, f, fontOptions(current, families, f.search.Text()), current, choose)
	})
	return pop.Layout(gtx, func(gtx C) D { return fontTrigger(gtx, th, fontLabel(current)) })
}

// fontTrigger is the Button variant="outline" w-64 justify-between
// font-normal the combobox opens from.
func fontTrigger(gtx C, th *ui.Theme, family string) D {
	size := image.Pt(gtx.Dp(fontTriggerWidth), gtx.Dp(ui.Space(9)))
	gtx.Constraints = layout.Exact(size)
	bg := th.Over(ui.Alpha(th.Palette.Input, 0.3))
	ui.Fill(gtx, bg, ui.RadiusMD)
	insetBorder(gtx, size, gtx.Dp(ui.RadiusMD), th.Over(th.Palette.Input))
	on := th.On(bg)
	return layout.Inset{Left: ui.Space(2.5), Right: ui.Space(2.5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, on.Text(ui.TextSM, family).Layout),
			ui.Gap(2),
			layout.Rigid(func(gtx C) D { return icChevronDown.Layout(gtx, ui.Space(4), on.MutedForeground) }),
		)
	})
}

// fontPanel is ComboboxContent: the search field over the matching
// families, the chosen one checked.
func (s *settingsScreen) fontPanel(gtx C, th *ui.Theme, f *fontPicker, options []string, current string, choose func(string)) D {
	in := th.Input(&f.search, fontSearch)
	in.Leading, in.Borderless = icSearch, true
	list := func(gtx C) D {
		if len(options) == 0 {
			return layout.UniformInset(ui.Space(2)).Layout(gtx, th.Text(ui.TextSM, fontEmpty).Muted().Layout)
		}
		gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(fontListHeight))
		f.list.Axis = layout.Vertical
		return th.ScrollArea(&f.list).Layout(gtx, len(options), func(gtx C, i int) D {
			return fontRow(gtx, th, f, options[i], options[i] == current, choose)
		})
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, layout.Rigid(in.Layout), ui.VGap(1), layout.Rigid(list))
}

// fontRow is one ComboboxItem: the family, truncated, and the check on the
// chosen one; picking it applies it and closes the panel.
func fontRow(gtx C, th *ui.Theme, f *fontPicker, family string, chosen bool, choose func(string)) D {
	if f.rows == nil {
		f.rows = map[string]*widget.Clickable{}
	}
	click := clickableOf(f.rows, family)
	if click.Clicked(gtx) {
		choose(family)
		f.panel.Close()
		f.search.SetText("")
	}
	return click.Layout(gtx, func(gtx C) D {
		size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(fontRowHeight))
		gtx.Constraints = layout.Exact(size)
		bg := th.Surface
		if click.Hovered() {
			bg = th.Accent
			ui.Fill(gtx, bg, ui.RadiusSM)
		}
		on := th.On(bg)
		return layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, on.Text(ui.TextSM, fontLabel(family)).Layout),
				layout.Rigid(func(gtx C) D {
					if !chosen {
						return D{Size: image.Pt(gtx.Dp(ui.Space(4)), 0)}
					}
					return icCheck.Layout(gtx, ui.Space(4), on.Foreground)
				}),
			)
		})
	})
}
