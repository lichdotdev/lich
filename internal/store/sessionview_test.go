package store

import (
	"testing"

	"github.com/lichdotdev/lich/api"
)

// A new session is drawn as a chat where its provider has one, unless the
// setting asks for the terminal; a fork is drawn as its terminal either way.
func TestANewSessionIsBornInTheViewTheSettingNames(t *testing.T) {
	chatKind := func(kind string) bool { return kind == "claude" }
	cases := []struct {
		name, setting, kind, want string
	}{
		{"chat by default", "", "claude", api.ViewChat},
		{"chat asked", api.ViewChat, "claude", api.ViewChat},
		{"terminal asked", api.ViewTerminal, "claude", ""},
		{"a provider with no chat", api.ViewChat, "codex", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newTestStore(t)
			s.SetChatKinds(chatKind)
			ctx := t.Context()
			if _, err := s.SetSetting(ctx, api.SetSettingOptions{Key: SessionViewKey, Value: c.setting}); err != nil {
				t.Fatal(err)
			}
			if _, err := s.AddProject(ctx, api.AddProjectOptions{ID: "p1", Name: "p", Path: t.TempDir()}); err != nil {
				t.Fatal(err)
			}
			if _, err := s.AddSession(ctx, api.AddSessionOptions{ProjectID: "p1", SessionID: "s1", Label: "S", Kind: c.kind, NextSeq: 2}); err != nil {
				t.Fatal(err)
			}
			if got := s.SessionView("s1"); got != c.want {
				t.Errorf("view = %q, want %q", got, c.want)
			}
		})
	}
}

func TestAForkIsDrawnAsItsTerminal(t *testing.T) {
	s := newTestStore(t)
	s.SetChatKinds(func(string) bool { return true })
	ctx := t.Context()
	if _, err := s.AddProject(ctx, api.AddProjectOptions{ID: "p1", Name: "p", Path: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddSessionFrom(ctx, api.AddSessionFromOptions{ProjectID: "p1", SessionID: "f1", Label: "F", Kind: "claude", NextSeq: 2, OriginID: "s1", OriginLabel: "S"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MarkForked(ctx, api.SessionIDOptions{ID: "f1"}); err != nil {
		t.Fatal(err)
	}
	if got := s.SessionView("f1"); got != "" {
		t.Errorf("a fork is drawn as %q, want its terminal", got)
	}
}

func TestWithoutChatKindsEverySessionIsBornOnItsTerminal(t *testing.T) {
	s := newTestStore(t)
	ctx := t.Context()
	if _, err := s.AddProject(ctx, api.AddProjectOptions{ID: "p1", Name: "p", Path: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddSession(ctx, api.AddSessionOptions{ProjectID: "p1", SessionID: "s1", Label: "S", Kind: "claude", NextSeq: 2}); err != nil {
		t.Fatal(err)
	}
	if got := s.SessionView("s1"); got != "" {
		t.Errorf("view = %q, want the terminal", got)
	}
}
