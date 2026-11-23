package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/spawn"
)

// controlCall bounds `lich control`: the longest spawn.Control takes, plus the
// trip back.
const controlCall = spawn.ControlWaitLimit + callSlack

// control takes the session, the action, and up to two words more: the value
// the action carries, and a slash command's arguments.
func (c *client) control(args []string) error {
	flags := newFlagSet("control")
	project := flags.String(
		"project", "",
		"narrow the target to one project, by name or by directory path, when the label is ambiguous",
	)
	asJSON := flags.Bool("json", false, "print the result as JSON")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	if flags.NArg() < 2 || flags.NArg() > 4 {
		return usageError("control")
	}

	out, err := callMethod(context.Background(), c, api.SpawnControl, spawn.ControlOptions{
		From: c.sessionID(), Target: flags.Arg(0), Project: *project,
		Action: flags.Arg(1), Value: flags.Arg(2), Args: flags.Arg(3),
	}, controlCall)
	if err != nil {
		return err
	}
	if *asJSON {
		if err := c.emit(out); err != nil {
			return err
		}
	} else {
		fmt.Fprintln(c.stdout, controlledText(out))
	}
	return controlOutcome(out.State)
}

func controlOutcome(state string) error {
	switch state {
	case spawn.ControlDone:
		return nil
	case spawn.ControlEnded:
		return exitStatus(ExitNoAnswer)
	default:
		return exitStatus(ExitPending)
	}
}

// controlledText words a control outcome for the command line and for an agent
// alike. A queued prompt says it goes through, so neither reads it as a failure
// and sends it again.
func controlledText(out spawn.Controlled) string {
	what := "the prompt"
	if out.Action == spawn.ControlCommand {
		what = "/" + strings.TrimLeft(strings.TrimSpace(out.Value), "/")
	}
	switch out.State {
	case spawn.ControlDone:
		return doneText(out, what)
	case spawn.ControlEnded:
		return fmt.Sprintf("%q ended before its turn on %s started.", out.Label, what)
	default:
		return fmt.Sprintf("%q took %s, which waits behind the turn it is running. It still goes through.",
			out.Label, what)
	}
}

func doneText(out spawn.Controlled, what string) string {
	switch out.Action {
	case spawn.ControlAbort:
		return fmt.Sprintf("%q stopped its turn.", out.Label)
	case spawn.ControlModel:
		return fmt.Sprintf("%q uses %s from its next request on.", out.Label, out.Value)
	case spawn.ControlEffort:
		return fmt.Sprintf("%q runs at %s effort from its next request on.", out.Label, out.Value)
	}
	return fmt.Sprintf("%q started a turn on %s.", out.Label, what)
}

// controlTools is `lich control` as an MCP tool, appended to mcpTools.
var controlTools = []mcpTool{
	{
		Name: "control_session",
		Description: "Drive another lich session drawn as a chat, on any agent with one: send it a " +
			"prompt, stop the turn it is running, set the model or reasoning effort its chat offers, or " +
			"run one of its slash commands, such as compact. A prompt or a command waits up " +
			"to 10 seconds for its turn to start; a result saying it waits behind the running turn is " +
			"not a failure, and it still goes through. A session in its terminal is refused, and so " +
			"is Crush, which has no chat. Your own session takes model and effort, never the other " +
			"actions. The slash commands /model and /effort are refused because Claude Code would " +
			"save them as the user's default for every new session; use the model and effort actions. " +
			"Experimental: may change outside semver.",
		Schema: schema(map[string]any{
			"session": property("string",
				"The session to drive, by the label on its card, the name it answers to or its lich id."),
			"action": property("string", "One of prompt, abort, model, effort, command."),
			"value": property("string",
				"prompt: the text. model: a model the session's chat offers, by its id. effort: "+
					"one of the effort levels its chat offers. command: the slash command's name, "+
					"with or without the slash. abort: omit."),
			"args": property("string", "command only: what follows the command's name, as typed."),
			"project": property("string",
				"Project to narrow to, by name or by directory path, when the same label "+
					"exists in more than one."),
		}, "session", "action"),
		Run: func(ctx context.Context, c *client, args mcpArgs) (string, error) {
			out, err := callMethod(ctx, c, api.SpawnControl, spawn.ControlOptions{
				From: c.sessionID(), Target: args.text("session"), Project: args.text("project"),
				Action: args.text("action"), Value: args.text("value"), Args: args.text("args"),
			}, controlCall)
			if err != nil {
				return "", err
			}
			return controlledText(out), nil
		},
	},
}
