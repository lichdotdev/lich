package main

import (
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

// The person's message is drawn as Markdown, as the agent's answers are,
// and still reads as typed to whoever reads the window without seeing it.
func TestThePersonsMessageIsDrawnAsMarkdown(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	text := "já foi feito merge na `reborn`?\nolha o **gate** também e o _CHANGELOG_"
	u.model.chat.history("s1", []backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: text}})
	r.settle(t)
	if _, ok := u.chats.pane("s1").md["user:u1"]; !ok {
		t.Error("the person's message did not go through the Markdown renderer")
	}
	writeShot(t, r, "user-markdown.png")
	if !r.shows(text) {
		t.Errorf("the message is not labelled as typed:\n%s", r.drawn())
	}
}

func TestHardBreaksKeepEveryLineButCode(t *testing.T) {
	for in, want := range map[string]string{
		"a\nb":                 "a  \nb",
		"a\n\nb":               "a\n\nb",
		"```\nx\ny\n```\nz\nw": "```\nx\ny\n```\nz  \nw",
		"- one\n- two":         "- one  \n- two",
		"trailing\n":           "trailing\n",
	} {
		if got := hardBreaks(in); got != want {
			t.Errorf("hardBreaks(%q) = %q, want %q", in, got, want)
		}
	}
}
