package cli

import (
	"context"
	"fmt"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/prompt"
	"github.com/lichdotdev/lich/internal/spawn"
)

// closeProject is `lich close-project`: the tab's close, from outside the
// window. The project is required, because the one an agent would inherit is
// its own, the one project it cannot close.
func (c *client) closeProject(args []string) error {
	flags := newFlagSet("close-project")
	asJSON := flags.Bool("json", false, "print the result as JSON")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return usageError("close-project")
	}

	closed, err := callMethod(context.Background(), c, api.SpawnCloseProject, spawn.CloseProjectOptions{
		From: c.sessionID(), Project: flags.Arg(0),
	}, openCall)
	if err != nil {
		return err
	}
	if *asJSON {
		return c.emit(closed)
	}
	fmt.Fprint(c.stdout, closedProjectText(c.lang(), closed))
	return nil
}

func closedProjectText(lang prompt.Lang, closed spawn.ClosedProject) string {
	return fmt.Sprintf(prompt.For(lang).ClosedProject, closed.Name, closed.Path)
}

// closeProjectTools is the project close as an MCP tool, appended to mcpTools.
var closeProjectTools = []mcpTool{
	{
		Name: "close_project",
		Description: "Close a project open in lich, as its tab's close does: the tab leaves the " +
			"window and every session in it stops. Nothing is deleted: the sessions stay with " +
			"the project, and opening a session on its directory path again reopens it with " +
			"them. The project you are running in cannot be closed from here, and neither can " +
			"the home project.",
		Schema: schema(map[string]any{
			"project": property("string",
				"The project to close, by name or by directory path."),
		}, "project"),
		Run: func(ctx context.Context, c *client, args mcpArgs) (string, error) {
			closed, err := callMethod(ctx, c, api.SpawnCloseProject, spawn.CloseProjectOptions{
				From: c.sessionID(), Project: args.text("project"),
			}, openCall)
			if err != nil {
				return "", err
			}
			return closedProjectText(c.lang(), closed), nil
		},
	},
}
