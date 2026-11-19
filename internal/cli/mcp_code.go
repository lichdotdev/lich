package cli

import (
	"context"
	"errors"

	"github.com/lichdotdev/lich/api"
)

// codeTools point the user at code, in the editor that takes the stage.
var codeTools = []mcpTool{
	{
		Name: "show_code",
		Description: "Show the user a file of this session's checkout, in the editor that takes their screen in " +
			"place of this session's terminal, on the given lines. Call it when the user asks to see code, " +
			"never on your own initiative: it takes over what they are looking at, and while they are elsewhere " +
			"it only offers the lines. path is relative to this session's checkout, or absolute inside it. A " +
			"missing file, a path outside the checkout or a line past the end of the file is an error, so " +
			"correct the call. comment puts a short note on the lines: use it only when it adds something your " +
			"answer does not say, such as why these lines matter, and never to repeat that answer.",
		Schema: schema(map[string]any{
			"path":     property("string", "File to show, relative to this session's checkout or absolute inside it."),
			"line":     property("integer", "First line to select, 1-based. Omit to show the top of the file."),
			"end_line": property("integer", "Last line to select, inclusive. Needs line."),
			"comment":  property("string", "Optional plain-text note on the lines, without markdown, up to 360 characters. Needs line."),
		}, "path"),
		Run: func(ctx context.Context, c *client, a mcpArgs) (string, error) {
			id := c.sessionID()
			if id == "" {
				return "", errors.New("show_code runs inside a lich session")
			}
			opts := api.ShowCodeOptions{ID: id, Path: a.text("path"), Line: lineArg(a, "line"), EndLine: lineArg(a, "end_line"), Comment: a.text("comment")}
			shown, err := callMethod(ctx, c, api.ProjectShowCode, opts, shortCall)
			if err != nil {
				return "", err
			}
			return encodeJSON(shown)
		},
	},
}

func lineArg(a mcpArgs, key string) int {
	if n := a.optionalInt(key); n != nil {
		return *n
	}
	return 0
}
