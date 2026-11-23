package subagent

import (
	"context"
	"errors"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/store"
)

func TestWorkspaceReadsTheAsker(t *testing.T) {
	w := Workspace{
		Load: func(context.Context, api.None) ([]store.Project, error) {
			return []store.Project{{Sessions: []store.Session{{ID: "s1", Kind: "codex", Label: "Relay fixes"}}}}, nil
		},
		Depth: func(string) int { return 1 },
	}
	if got, ok := w.Asker("s1"); !ok || got != (Asker{Kind: "codex", Label: "Relay fixes", Depth: 1}) {
		t.Errorf("Asker = %+v %v", got, ok)
	}
	if _, ok := w.Asker("nobody"); ok {
		t.Error("found a session that is not there")
	}
	w.Load = func(context.Context, api.None) ([]store.Project, error) { return nil, errors.New("disk gone") }
	if _, ok := w.Asker("s1"); ok {
		t.Error("an unreadable workspace found an asker")
	}
}
