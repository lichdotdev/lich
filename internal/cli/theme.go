package cli

import (
	"context"
	"fmt"

	"github.com/lichdotdev/lich/internal/themes"
)

// themeRow is one installed theme as `lich theme list` prints it: what a
// script needs to pick an id, without the colors.
type themeRow struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Scheme string `json:"scheme"`
	Pair   string `json:"pair,omitempty"`
	InUse  bool   `json:"inUse"`
}

// theme lists the installed themes or switches the window to one, for a
// script that changes a desktop's theme to bring lich along.
func (c *client) theme(args []string) error {
	flags := newFlagSet("theme")
	asJSON := flags.Bool("json", false, "print the result as JSON")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	switch {
	case flags.NArg() == 1 && flags.Arg(0) == "list":
		return c.listThemes(*asJSON)
	case flags.NArg() == 2 && flags.Arg(0) == "switch":
		return c.switchTheme(flags.Arg(1), *asJSON)
	}
	return usageError("theme")
}

func (c *client) listThemes(asJSON bool) error {
	ctx := context.Background()
	var installed []themes.Theme
	if err := c.call(ctx, "themes.List", nil, shortCall, &installed); err != nil {
		return err
	}
	var selection themes.Selection
	if err := c.call(ctx, "themes.Selection", nil, shortCall, &selection); err != nil {
		return err
	}
	rows := []themeRow{{ID: themes.SystemID, Name: "System, the bundled theme the OS asks for", InUse: selection.Theme == themes.SystemID}}
	for _, t := range installed {
		rows = append(rows, themeRow{ID: t.ID, Name: t.Name, Scheme: t.Scheme, Pair: t.Pair, InUse: t.ID == selection.Theme})
	}
	if asJSON {
		return c.emit(rows)
	}
	fmt.Fprintln(c.stdout, "theme\tscheme\tname")
	for _, row := range rows {
		mark := ""
		if row.InUse {
			mark = "\t(in use)"
		}
		fmt.Fprintf(c.stdout, "%s\t%s\t%s%s\n", row.ID, orDash(row.Scheme), row.Name, mark)
	}
	return nil
}

func (c *client) switchTheme(id string, asJSON bool) error {
	call := []any{themes.SelectOptions{ID: id}}
	if err := c.call(context.Background(), "themes.Select", call, shortCall, nil); err != nil {
		return err
	}
	if asJSON {
		return c.emit(themes.SelectOptions{ID: id})
	}
	fmt.Fprintf(c.stdout, "Switched to %q.\n", id)
	return nil
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
