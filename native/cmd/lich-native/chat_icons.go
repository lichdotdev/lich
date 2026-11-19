package main

import (
	"strings"

	"github.com/lichdotdev/lich/native/ui/icons"
)

var (
	icThink     = icons.Lucide("sparkles")
	icToolOther = icons.Lucide("wrench")
	icToolMCP   = icons.Lucide("plug")
	icToolRead  = icons.Lucide("file-text")
	icToolEdit  = icons.Lucide("pencil")
	icToolFind  = icons.Lucide("search")
	icToolRun   = icons.Lucide("terminal")
	icToolWeb   = icons.Lucide("globe")
	icToolAgent = icons.Lucide("bot")
	icToolPlan  = icons.Lucide("shield-check")
)

// toolIcons is the glyph of each tool the chat knows, by its name lowered:
// Claude Code's own names, the kinds ACP gives a call and Codex's. A tool it
// does not know wears icToolOther.
var toolIcons = map[string]*icons.Icon{
	"read":         icToolRead,
	"notebookread": icToolRead,
	"edit":         icToolEdit,
	"multiedit":    icToolEdit,
	"write":        icToolEdit,
	"notebookedit": icToolEdit,
	"apply_patch":  icToolEdit,
	"delete":       icons.Lucide("trash-2"),
	"move":         icons.Lucide("arrow-right-left"),
	"search":       icToolFind,
	"grep":         icToolFind,
	"glob":         icToolFind,
	"websearch":    icToolFind,
	"find":         icToolFind,
	"execute":      icToolRun,
	"bash":         icToolRun,
	"bashoutput":   icToolRun,
	"killshell":    icToolRun,
	"shell":        icToolRun,
	"fetch":        icToolWeb,
	"webfetch":     icToolWeb,
	"think":        icThink,
	"task":         icToolAgent,
	"agent":        icToolAgent,
	"todowrite":    icons.Lucide("list-checks"),
	"switch_mode":  icToolPlan,
	"exitplanmode": icToolPlan,
}

// toolIcon is the glyph of the tool called name.
func toolIcon(name string) *icons.Icon {
	name = strings.ToLower(name)
	if ic, ok := toolIcons[name]; ok {
		return ic
	}
	if strings.HasPrefix(name, "mcp__") {
		return icToolMCP
	}
	return icToolOther
}
