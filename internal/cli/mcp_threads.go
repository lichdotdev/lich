package cli

import (
	"context"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/prompt"
)

// threadTools let the agent say which review threads it fixed, so lich
// resolves those and leaves the questions to the person.
var threadTools = []mcpTool{
	{
		Name: "mark_thread_fixed",
		Description: "Mark a pull request review thread lich handed you from its Pulls screen as fixed by a " +
			"commit you made. lich resolves the thread on GitHub and replies with that commit once the pull " +
			"request's head holds it, once that commit is pushed, so never resolve or reply yourself. " +
			"Call it only for a thread that asked for a change you made: a question, a point to discuss or a " +
			"change you did not make is never marked, and goes back to the user instead. thread_id is the id " +
			"lich gave with the thread; commit is the sha of the fix, already committed, never a name such as HEAD.",
		Schema: schema(map[string]any{
			"thread_id": property("string", "The thread's id, as lich gave it (PRRT_...)."),
			"commit":    property("string", "The sha of the commit that fixes it, full or abbreviated (git rev-parse HEAD)."),
		}, "thread_id", "commit"),
		Run: func(ctx context.Context, c *client, a mcpArgs) (string, error) {
			opts := api.MarkThreadFixedOptions{ThreadID: a.text("thread_id"), Commit: a.text("commit")}
			if _, err := callMethod(ctx, c, api.ProjectMarkThreadFixed, opts, shortCall); err != nil {
				return "", err
			}
			return prompt.For(c.lang()).ThreadMarked, nil
		},
	},
}
