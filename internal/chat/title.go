package chat

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/lichdotdev/lich/internal/prompt"
)

// titleRunes caps a title: a model told to be brief can still run on.
const titleRunes = 60

// titler is a driver that names its chat by running its provider's CLI once,
// outside the protocol: no hook and no history of its own (the provider's
// print mode, ephemeral, at low effort), and none of lich's environment, which
// is what every hook reads, so the call is not taken for the session.
type titler interface {
	// titleArgs is the CLI's arguments, nil for a provider that is out.
	titleArgs(prompt string) []string
}

// titleReader is a titler whose stdout is more than the reply, and whose call
// leaves a conversation in the provider's own history: it returns the reply and
// deletes that conversation. It runs on a failed call's output too, which can
// still name the conversation.
type titleReader interface {
	titleReply(out []byte) string
}

// titleInputRunes caps the message a title is asked of: the start says what the chat is about,
// and the rest only lengthens the call.
const titleInputRunes = 1000

func clip(text string) string {
	if r := []rune(text); len(r) > titleInputRunes {
		return string(r[:titleInputRunes])
	}
	return text
}

// titleAfter names a session after its first message as that turn ends, once,
// for a chat that began with a message the person wrote.
func (c *chatSession) titleAfter(reason string) {
	t, ok := c.drv.(titler)
	turn, text := c.conv.turnText()
	if ok && t.titleArgs("") == nil {
		ok = false
	}
	if !ok || reason != stopEndTurn || turn != 1 || strings.TrimSpace(text) == "" || isEnvelope(text) {
		return
	}
	c.mu.Lock()
	done := c.titled
	c.titled = true
	c.mu.Unlock()
	if done {
		return
	}
	go c.title(t, text)
}

// title asks for the name and applies it. Every failure is silence: the card
// keeps the name lich gave it.
func (c *chatSession) title(t titler, text string) {
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	cmd, err := c.proc.Command(ctx, t.titleArgs(fmt.Sprintf(prompt.For(c.svc.lang()).ChatTitlePrompt, clip(text)))...)
	if err == nil {
		cmd.Dir = c.cwd
		cmd.Env = titleEnv(cmd.Env)
		var out []byte
		out, err = cmd.Output()
		reply := string(out)
		if r, ok := t.(titleReader); ok {
			reply = r.titleReply(out)
		}
		if title := cleanTitle(reply); err == nil && title != "" {
			err = c.svc.term.SetTitle(c.id, title)
		}
	}
	if err != nil {
		slog.Debug("chat: name the session", "session", c.id, "err", err)
	}
}

// titleEnv is env without the variables the hooks read.
func titleEnv(env []string) []string { return slices.DeleteFunc(env, titleEnvDropped) }

// titleEnvDropped is whether kv is a variable the hooks read (docs/hooks/README.md).
func titleEnvDropped(kv string) bool {
	return strings.HasPrefix(kv, "LICH_PORT=") || strings.HasPrefix(kv, "LICH_TOKEN=") || strings.HasPrefix(kv, "LICH_SESSION_ID=")
}

// cleanTitle is a model's reply as a card's name: its first line, out of
// quotes, without a closing stop, at most titleRunes long.
func cleanTitle(reply string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(reply), "\n")
	line = strings.TrimSpace(strings.Trim(line, "\"'`“”‘’「」.。 "))
	if r := []rune(line); len(r) > titleRunes {
		return string(r[:titleRunes])
	}
	return line
}
