package lichclient

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestPatchNotesKeepsNoSectionApartFromAnEmptyOne(t *testing.T) {
	for name, tc := range map[string]struct {
		body      string
		wantGroup bool
	}{
		"no section":    {`{"version":"1.0.0","highlights":null,"groups":null}`, false},
		"empty section": {`{"version":"1.0.0","highlights":null,"groups":[]}`, true},
	} {
		c := fakeBackend(t, func(method string, _ []json.RawMessage) (any, int) {
			if method != "patchnotes.Current" {
				t.Errorf("method %s", method)
			}
			return json.RawMessage(tc.body), http.StatusOK
		})
		n, err := c.PatchNotes(context.Background())
		if err != nil || (n.Groups != nil) != tc.wantGroup {
			t.Errorf("%s: groups %#v, %v; want non-nil %v", name, n.Groups, err, tc.wantGroup)
		}
	}
}

func TestInstallAndUpdatePluginNameTheProvider(t *testing.T) {
	var calls []string
	c := fakeBackend(t, func(method string, args []json.RawMessage) (any, int) {
		calls = append(calls, method+" "+string(args[0]))
		return nil, http.StatusOK
	})
	ctx := context.Background()
	if err := c.InstallPlugin(ctx, "claude"); err != nil {
		t.Fatal(err)
	}
	if err := c.UpdatePlugin(ctx, "codex"); err != nil {
		t.Fatal(err)
	}
	want := []string{`agentplugin.Install "claude"`, `agentplugin.Update "codex"`}
	if len(calls) != 2 || calls[0] != want[0] || calls[1] != want[1] {
		t.Fatalf("calls %v, want %v", calls, want)
	}
}
