package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/spawn"
)

// askCall bounds `lich ask`: the longest wait spawn.Ask holds, plus the trip
// back.
const askCall = spawn.AskWaitLimit + callSlack

// ask takes the session and the question, whose words need no quoting.
func (c *client) ask(args []string) error {
	flags := newFlagSet("ask")
	project := flags.String(
		"project", "",
		"narrow the target to one project, by name or by directory path, when the label is ambiguous",
	)
	asJSON := flags.Bool("json", false, "print the result as JSON")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	if flags.NArg() < 2 {
		return usageError("ask")
	}

	question := strings.Join(flags.Args()[1:], " ")
	opts := spawn.AskOptions{From: c.sessionID(), Target: flags.Arg(0), Project: *project, Question: question}
	out, err := callMethod(context.Background(), c, api.SpawnAsk, opts, askCall)
	if err != nil {
		return err
	}
	if *asJSON {
		return c.emit(out)
	}
	fmt.Fprintln(c.stdout, out.Answer)
	return nil
}

// askTools is `lich ask` as an MCP tool, appended to mcpTools.
var askTools = []mcpTool{
	{
		Name: "ask_session",
		Description: "Ask another running Claude Code session in lich a side question and get its " +
			"answer, without interrupting it: it answers from its own conversation while its turn " +
			"goes on, and neither the question nor the answer enters that conversation. It sees " +
			"the conversation as of its last finished reply, not the step it is taking right now, " +
			"and cannot use tools to find out more. Waits up to 90 seconds; ask for a brief " +
			"answer. Claude Code sessions only, and never your own. " +
			"Experimental: may change outside semver.",
		Schema: schema(map[string]any{
			"session": property("string",
				"The session to ask, by the label on its card, the name it answers to or its lich id."),
			"question": property("string", "The question."),
			"project": property("string",
				"Project to narrow to, by name or by directory path, when the same label "+
					"exists in more than one."),
		}, "session", "question"),
		Run: func(ctx context.Context, c *client, args mcpArgs) (string, error) {
			out, err := callMethod(ctx, c, api.SpawnAsk, spawn.AskOptions{
				From: c.sessionID(), Target: args.text("session"), Project: args.text("project"),
				Question: args.text("question"),
			}, askCall)
			if err != nil {
				return "", err
			}
			return out.Answer, nil
		},
	},
}
