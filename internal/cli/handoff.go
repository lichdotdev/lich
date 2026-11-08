package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/prompt"
	"github.com/lichdotdev/lich/internal/relay"
	"github.com/lichdotdev/lich/internal/spawn"
)

// handoffWait bounds the wait for a successor's agent to reach its prompt.
// Opening (openCall) and this together stay under the 120s at which an MCP
// client detaches the call.
const handoffWait = 45 * time.Second

// handoffPoll is how often that wait asks whether the prompt is up, the pace
// the window's own handoffs ask at.
const handoffPoll = 250 * time.Millisecond

// handoffNote is what a session hands its successor: the five fields of a
// clean break.
type handoffNote struct {
	done, branch, commits, nextSteps, gotchas string
}

// check refuses a note the successor could not start from, before anything is
// opened: a session left behind by a refused call is one nobody asked for.
func (n handoffNote) check() error {
	if strings.TrimSpace(n.done) == "" || strings.TrimSpace(n.nextSteps) == "" {
		return errors.New("a handoff needs what is done and the next steps: the successor starts from nothing else")
	}
	if size := len(n.done) + len(n.branch) + len(n.commits) + len(n.nextSteps) + len(n.gotchas); size > relay.PromptLimit {
		return fmt.Errorf("the handoff note is %d bytes and the limit is %d: name the paths to read instead of pasting their contents",
			size, relay.PromptLimit)
	}
	return nil
}

// text is the note as the successor reads it at its prompt, from the session
// that wrote it.
func (n handoffNote) text(lang prompt.Lang, from string) string {
	t := prompt.For(lang)
	orNone := func(field string) string {
		if strings.TrimSpace(field) == "" {
			return t.HandoffNone
		}
		return field
	}
	return fmt.Sprintf(t.HandoffNote, from, n.done, orNone(n.branch), orNone(n.commits), n.nextSteps, orNone(n.gotchas))
}

// openSuccessor opens the session that takes the caller's work over.
func (c *client) openSuccessor(ctx context.Context, opts api.HandoffSessionOptions, note handoffNote) (spawn.Session, error) {
	if err := note.check(); err != nil {
		return spawn.Session{}, err
	}
	opts.From = c.sessionID()
	return callMethod(ctx, c, api.SpawnHandoff, opts, openCall)
}

// suggestionText tells the caller what became of a suggested profile.
func (c *client) suggestionText(suggested string, opened spawn.Session) string {
	t := prompt.For(c.lang())
	switch {
	case suggested == "":
		return ""
	case opened.ProfileDropped != "":
		return fmt.Sprintf(t.HandoffProfileDropped, suggested, opened.ProfileDropped)
	}
	return fmt.Sprintf(t.HandoffProfileSuggested, suggested)
}

// submitNote types the note at the successor's prompt once there is one, with
// no ticket: nothing is owed back to a session that is done with the work.
// Text written before the prompt is up lands in the boot and is thrown away.
func (c *client) submitNote(ctx context.Context, opened spawn.Session, note handoffNote) error {
	deadline := time.Now().Add(handoffWait)
	for {
		ready, err := callMethod(ctx, c, api.TerminalReady, api.SessionIDOptions{ID: opened.ID}, shortCall)
		if err != nil {
			return err
		}
		if ready {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%q was still not at a prompt after %s", opened.Label, handoffWait)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(handoffPoll):
		}
	}
	_, err := callMethod(ctx, c, api.RelaySubmit, api.SubmitOptions{
		SessionID: opened.ID, Message: note.text(c.lang(), opened.OriginLabel),
	}, shortCall)
	return err
}

// handoff is `lich handoff`.
func (c *client) handoff(args []string) error {
	flags := newFlagSet("handoff")
	var note handoffNote
	flags.StringVar(&note.done, "done", "", "what is finished, verified and committed")
	flags.StringVar(&note.nextSteps, "next", "", "what the successor does next")
	flags.StringVar(&note.branch, "branch", "", "the branch the work is on")
	flags.StringVar(&note.commits, "commits", "", "the commits the work landed in")
	flags.StringVar(&note.gotchas, "gotchas", "", "what would trip the successor up")
	var opts api.HandoffSessionOptions
	flags.StringVar(&opts.Kind, "kind", "", "what the successor runs; defaults to this session's provider")
	flags.StringVar(&opts.Model, "model", "", "model the provider runs, in the provider's own spelling; defaults to this session's")
	flags.StringVar(&opts.Effort, "effort", "", "reasoning effort the provider runs at; defaults to this session's")
	flags.StringVar(&opts.Profile, "profile", "", "a profile to suggest to the user for the successor; only the user can move it there")
	asJSON := flags.Bool("json", false, "print the result as JSON")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return usageError("handoff")
	}
	opened, err := c.openSuccessor(context.Background(), opts, note)
	if err != nil {
		return err
	}
	if *asJSON {
		if err := c.emit(opened); err != nil {
			return err
		}
	} else {
		fmt.Fprint(c.stdout, openedText(c.lang(), opened), c.suggestionText(opts.Profile, opened))
	}
	if err := c.submitNote(context.Background(), opened, note); err != nil {
		return fmt.Errorf("the session is open, but the handoff note did not reach it: %w", err)
	}
	if !*asJSON {
		fmt.Fprint(c.stdout, prompt.For(c.lang()).HandoffSent)
	}
	return nil
}

// handoffTools is the handoff as an MCP tool, appended to mcpTools.
var handoffTools = []mcpTool{
	{
		Name: "handoff_session",
		Description: "Hand your work off to a fresh session of yourself, at a clean break (just " +
			"committed, tests green, the next step clear) instead of carrying on in a context " +
			"that has grown long. lich opens the successor in your checkout and folder, on your " +
			"provider, model and effort unless you name others and in your profile, and types " +
			"your note at its prompt. The note is capped at 8 KB, like send_to_session: name what " +
			"to read, do not paste it. Your session stays open: the user closes it. Stop working " +
			"once this returns. Naming a profile only suggests it to the user, once the " +
			"successor's first turn ends: the successor still starts in your profile, and " +
			"moving it is always the user's click. Returns " +
			"the successor's names, and whether a suggestion was dropped and why.",
		Schema: schema(map[string]any{
			"done": property("string",
				"What is finished: verified and committed, so the successor does not redo it."),
			"next_steps": property("string", "What the successor does next, in order."),
			"branch":     property("string", "The branch the work is on."),
			"commits":    property("string", "The commits the work landed in, by hash and subject."),
			"gotchas": property("string",
				"What would trip the successor up: traps found, approaches ruled out, and why."),
			"kind": property("string",
				"What the successor runs: claude, codex, antigravity, opencode, omp, crush, "+
					"cursor or kiro. Defaults to the same agent you are."),
			"model": property("string",
				"Model the successor's provider runs, spelled as its --model flag takes it. "+
					"Defaults to yours when the provider is yours."),
			"effort": property("string",
				"Reasoning effort the successor runs at, spelled as its provider takes it. "+
					"Defaults to yours when the provider is yours."),
			"profile": property("string",
				"A profile to suggest to the user for the successor, by slug, once its first turn "+
					"ends. Only a suggestion: the successor starts in your profile and only the "+
					"user's click moves it. One "+
					"that cannot apply is dropped and the handoff goes on."),
		}, "done", "next_steps"),
		Run: func(ctx context.Context, c *client, args mcpArgs) (string, error) {
			note := handoffNote{
				done: args.text("done"), nextSteps: args.text("next_steps"), branch: args.text("branch"),
				commits: args.text("commits"), gotchas: args.text("gotchas"),
			}
			opts := api.HandoffSessionOptions{
				Kind: args.text("kind"), Model: args.text("model"), Effort: args.text("effort"),
				Profile: args.text("profile"),
			}
			opened, err := c.openSuccessor(ctx, opts, note)
			if err != nil {
				return "", err
			}
			text := openedText(c.lang(), opened) + c.suggestionText(opts.Profile, opened)
			if err := c.submitNote(ctx, opened, note); err != nil {
				return text + "\n" + handOverFailure(c.lang(), err, relaySend), nil
			}
			return text + prompt.For(c.lang()).HandoffSent, nil
		},
	},
}
