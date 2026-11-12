package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/prompt"
)

// scheduleClock is how a scheduled time reads back to the agent that set it.
const scheduleClock = "2006-01-02 15:04"

// scheduleRequest is `lich schedule` and schedule_prompt's arguments.
type scheduleRequest struct {
	when, on, prompt string
	clear            bool
}

func (c *client) schedule(args []string) error {
	flags := newFlagSet("schedule")
	when := flags.String("when", "", "when to type the prompt: 45m, 2h30, 14:30 or \"2026-11-12 14:30\"")
	on := flags.String("on", "", "what to wait for: checks-failed, checks-passed or review")
	clear := flags.Bool("clear", false, "remove what this session has scheduled")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	if flags.NArg() > 1 {
		return usageError("schedule")
	}
	text, err := c.runSchedule(context.Background(), scheduleRequest{when: *when, on: *on, prompt: flags.Arg(0), clear: *clear})
	if err != nil {
		return err
	}
	fmt.Fprintln(c.stdout, text)
	return nil
}

func (c *client) runSchedule(ctx context.Context, req scheduleRequest) (string, error) {
	opts, err := scheduleOptions(req, c.sessionID(), time.Now())
	if err != nil {
		return "", err
	}
	if _, err := callMethod(ctx, c, api.RelayScheduleSelf, opts, shortCall); err != nil {
		return "", err
	}
	return scheduledText(c.lang(), opts), nil
}

func scheduleOptions(req scheduleRequest, from string, now time.Time) (api.ScheduleSelfOptions, error) {
	opts := api.ScheduleSelfOptions{From: from, On: req.on, Prompt: req.prompt}
	if req.clear {
		if req.when != "" || req.on != "" {
			return opts, errors.New("--clear takes no --when or --on")
		}
		opts.Prompt = ""
		return opts, nil
	}
	if (req.when == "") == (req.on == "") {
		return opts, errors.New("give one of --when or --on, or --clear")
	}
	if req.when != "" {
		at, ok := api.ParseWhen(req.when, now)
		if !ok {
			return opts, fmt.Errorf("cannot read the time %q: try 45m, 2h30, 14:30 or \"2026-11-12 14:30\"", req.when)
		}
		opts.At = at.Unix()
	}
	return opts, nil
}

func scheduledText(lang prompt.Lang, opts api.ScheduleSelfOptions) string {
	text := prompt.For(lang)
	switch {
	case opts.On != "":
		return fmt.Sprintf(text.ScheduledOnResult, opts.On, api.WatchRunLimit)
	case opts.At != 0:
		return fmt.Sprintf(text.ScheduledAtResult, time.Unix(opts.At, 0).Format(scheduleClock))
	}
	return text.ScheduleCleared
}

// scheduleTools is the schedule as an MCP tool, appended to mcpTools.
var scheduleTools = []mcpTool{
	{
		Name: "schedule_prompt",
		Description: "Have a prompt typed into the session you are running in, later: at a time, or " +
			"when something happens on this branch's pull request. Use it instead of polling for CI " +
			"or for a review. It only acts on your own session, and something the person set there " +
			"stays until they change it. checks-failed stays on until the pull request closes, at " +
			"most 3 runs on it; checks-passed and review run once. When checks-failed fires, " +
			"read_check_log reads why.",
		Schema: schema(map[string]any{
			"when":   property("string", "When to type it: 45m, 2h30, 14:30 or 2026-11-12 14:30. Give this or on."),
			"on":     property("string", "What to wait for: checks-failed, checks-passed or review. Give this or when."),
			"prompt": property("string", "What to type. Capped at 8 KB."),
			"clear":  property("boolean", "Remove what this session has scheduled, instead of setting it."),
		}),
		Run: func(ctx context.Context, c *client, args mcpArgs) (string, error) {
			return c.runSchedule(ctx, scheduleRequest{
				when: args.text("when"), on: args.text("on"), prompt: args.text("prompt"), clear: args.flag("clear"),
			})
		},
	},
}
