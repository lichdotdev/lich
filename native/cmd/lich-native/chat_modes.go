package main

import (
	"image/color"

	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// acpModeNames is lich's name, by provider, for each mode an ACP server builds
// in, which it sends under its own English name, or under its id where a mode
// is an agent (opencode, Kiro). An agent the person made keeps the name they
// gave it. Claude Code and Codex send ids alone, which choiceLabel names.
// Measured on Antigravity 1.5.11, oh-my-pi 18.8.9, opencode 1.19.4 and Kiro
// 2.21.0; Cursor's are read from its 2026.10.19 bundle.
var acpModeNames = map[string]map[string]string{
	// Antigravity's default asks before edits and commands; oh-my-pi's never asks.
	"antigravity": {"default": "chat.choice.manual", "accept-edits": "chat.choice.acceptEdits", "auto": "chat.choice.auto", "plan": "chat.choice.plan"},
	"omp":         {"default": "chat.choice.default", "plan": "chat.choice.plan"},
	cursorKind:    {"agent": "chat.choice.agent", "plan": "chat.choice.plan", "ask": "chat.choice.ask"},
	opencodeKind:  {"build": "chat.choice.build", "plan": "chat.choice.plan"},
	kiroKind:      {"kiro_default": "chat.choice.default", "kiro_planner": "chat.choice.planner", "kiro_guide": "chat.choice.guide"},
}

// modeLabel is the name of mode c of a session of kind.
func modeLabel(kind string, c backend.ChatChoice) string {
	if key, ok := acpModeNames[kind][c.ID]; ok {
		return i18n.T(key)
	}
	return choiceLabel(c)
}

// rainbowContent is optionContent for a dangerous choice: the bolt filled and
// the name, both in the rainbow.
func rainbowContent(text string) footerContent {
	return func(gtx C, th *ui.Theme, fg color.NRGBA) D {
		return chipFace(gtx, fg, func(gtx C) D { return th.RainbowIcon(gtx, icDanger, footerGlyph) }, th.Text(ui.TextXS, text).Rainbow())
	}
}

// rainbowBolt is the menu's lead of a dangerous choice.
func rainbowBolt(th *ui.Theme) func(gtx C, size unit.Dp) D {
	return func(gtx C, size unit.Dp) D { return th.RainbowIcon(gtx, icDanger, size) }
}
