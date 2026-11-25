package main

import (
	"encoding/json"
	"image"
	"log"
	"slices"
	"strings"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// chatPrefKinds is chat.Kinds: the providers a session can be drawn as a
// chat of, the only sessions born on the preferred model and effort.
var chatPrefKinds = []string{"claude", "opencode", "omp", "antigravity", "kiro", "cursor", "codex"}

// chatPrefSearchFrom is how many choices a list holds before it gets a search
// field: opencode lists every model of every configured provider.
const chatPrefSearchFrom = 8

func hasChatPrefs(kind string) bool { return slices.Contains(chatPrefKinds, kind) }

// providerPrefKey is the setting a new chat of kind is born on for category
// (store.ProviderModel, store.ProviderEffort).
func providerPrefKey(kind, category string) string {
	if category == api.OptionEffort {
		return "provider." + kind + ".effort"
	}
	return "provider." + kind + ".model"
}

// providerChoicesKey is what the last chat of kind offered for category
// (store.SetProviderChoices).
func providerChoicesKey(kind, category string) string {
	return "provider." + kind + "." + category + ".choices"
}

func chatPrefKeys(kind string) []string {
	var keys []string
	for _, category := range []string{api.OptionModel, api.OptionEffort} {
		keys = append(keys, providerPrefKey(kind, category), providerChoicesKey(kind, category))
	}
	return keys
}

// chatPrefPicker is one preference's combobox.
type chatPrefPicker struct {
	panel  ui.DropdownPanelState
	search widget.Editor
	list   widget.List
	rows   map[string]*ui.Clickable
}

// chatPrefChoices decodes what the last chat offered. A value it cannot read
// lists nothing rather than failing the screen.
func chatPrefChoices(raw string) []api.ChatChoice {
	var choices []api.ChatChoice
	if raw == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(raw), &choices); err != nil {
		log.Printf("settings: read chat choices: %v", err)
		return nil
	}
	return choices
}

// chatPrefOptions is the list under the trigger: the provider's default, then
// what the last chat offered matching query. A stored value that chat no
// longer offers stays listed, so the screen never hides what a chat starts on.
func chatPrefOptions(choices []api.ChatChoice, current, query string) []api.ChatChoice {
	if current != "" && !slices.ContainsFunc(choices, func(c api.ChatChoice) bool { return c.ID == current }) {
		choices = append(choices, api.ChatChoice{ID: current})
	}
	query = strings.ToLower(strings.TrimSpace(query))
	options := []api.ChatChoice{{}}
	for _, c := range choices {
		if query == "" || strings.Contains(strings.ToLower(chatChoiceLabel(c)), query) || strings.Contains(strings.ToLower(c.ID), query) {
			options = append(options, c)
		}
	}
	return options
}

func chatChoiceLabel(c api.ChatChoice) string {
	if c.Label != "" {
		return c.Label
	}
	return c.ID
}

// chatPrefLabel is what the trigger reads for the stored value.
func chatPrefLabel(choices []api.ChatChoice, current, provider string) string {
	if current == "" {
		return i18n.T("settings.chatPrefSetting.default", i18n.Args{"provider": provider})
	}
	if i := slices.IndexFunc(choices, func(c api.ChatChoice) bool { return c.ID == current }); i >= 0 {
		return chatChoiceLabel(choices[i])
	}
	return current
}

// chatPrefShown reports whether the category's row is drawn. The model's
// always is; the effort's once a chat offered one or one is stored, since
// Kiro offers none over ACP.
func chatPrefShown(category string, choices []api.ChatChoice, current string) bool {
	return category == api.OptionModel || len(choices) > 0 || current != ""
}

// chatPrefRow is the model or the effort a new chat of p starts on, global.
func (s *settingsScreen) chatPrefRow(gtx C, th *ui.Theme, values scopedValues, m providersMachine, p providerInfo, category string, pk *chatPrefPicker, first bool) D {
	key := providerPrefKey(p.id, category)
	titleKey, descKey := "settings.chatPrefSetting.modelTitle", "settings.chatPrefSetting.modelDescription"
	if category == api.OptionEffort {
		titleKey, descKey = "settings.chatPrefSetting.effortTitle", "settings.chatPrefSetting.effortDescription"
	}
	r := s.providerRow(gtx, settingEntry{titleKey: titleKey, key: key}, values)
	r.description = i18n.T(descKey, i18n.Args{"provider": p.name})
	choices := chatPrefChoices(m.globals[providerChoicesKey(p.id, category)])
	if len(choices) == 0 {
		r.detail = i18n.T("settings.chatPrefSetting.noChoices", i18n.Args{"provider": p.name})
	}
	if r.kind == rowGlobalOnly {
		return r.layout(gtx, th, first, nil)
	}
	current := m.globals[key]
	return r.layout(gtx, th, first, func(gtx C, th *ui.Theme) D {
		pop := th.DropdownPanel(s.root, &pk.panel, providerTriggerW, func(gtx C, th *ui.Theme) D {
			return s.chatPrefPanel(gtx, th, pk, choices, current, p.name, key)
		})
		return pop.Layout(gtx, th.SelectTrigger(chatPrefLabel(choices, current, p.name), providerTriggerW).Layout)
	})
}

// chatPrefPanel is the list, with a search field over a long one.
func (s *settingsScreen) chatPrefPanel(gtx C, th *ui.Theme, pk *chatPrefPicker, choices []api.ChatChoice, current, provider, key string) D {
	options := chatPrefOptions(choices, current, pk.search.Text())
	list := func(gtx C) D {
		gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(fontListHeight))
		pk.list.Axis = layout.Vertical
		return th.ScrollArea(&pk.list).Layout(gtx, len(options), func(gtx C, i int) D {
			return s.chatPrefOption(gtx, th, pk, options[i], options[i].ID == current, provider, key)
		})
	}
	if len(choices) < chatPrefSearchFrom {
		return list(gtx)
	}
	in := th.Input(&pk.search, i18n.T("settings.chatPrefSetting.search"))
	in.Leading, in.Borderless = icSearch, true
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, layout.Rigid(in.Layout), ui.VGap(1), layout.Rigid(list))
}

// chatPrefOption is one item, checked when chosen; picking it stores it, the
// default as no value, and closes the list.
func (s *settingsScreen) chatPrefOption(gtx C, th *ui.Theme, pk *chatPrefPicker, c api.ChatChoice, chosen bool, provider, key string) D {
	if pk.rows == nil {
		pk.rows = map[string]*ui.Clickable{}
	}
	click := clickableOf(pk.rows, c.ID)
	if click.Clicked(gtx) {
		s.writeProviderKey(key, "", c.ID, nil)
		pk.panel.Close()
		pk.search.SetText("")
	}
	label := chatChoiceLabel(c)
	if c.ID == "" {
		label = i18n.T("settings.chatPrefSetting.default", i18n.Args{"provider": provider})
	}
	return click.Layout(gtx, func(gtx C) D {
		gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(fontRowHeight)))
		bg := th.Surface
		if click.Hovered() {
			bg = th.Accent
			ui.Fill(gtx, bg, ui.RadiusSM)
		}
		on := th.On(bg)
		return layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, on.Text(ui.TextSM, label).Layout),
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
