package main

import (
	"testing"

	"github.com/lichdotdev/lich/native/backend"
)

func TestAccountGate(t *testing.T) {
	two := []string{"github.com/octocat", "github.com/octocat-work"}
	tests := []struct {
		name string
		st   backend.GitHubAccountState
		gate accountGate
		pick string
	}{
		{"one account has nothing to choose", backend.GitHubAccountState{Accounts: two[:1]}, gateNone, "github.com/octocat"},
		{"gh not signed in", backend.GitHubAccountState{}, gateNone, ""},
		{"active account with two signed in", backend.GitHubAccountState{Accounts: two}, gateChoose, "github.com/octocat"},
		{"stored account that sees the repository", backend.GitHubAccountState{Account: two[0], Accounts: two}, gateNone, "github.com/octocat-work"},
		{"stored account GitHub hides it from", backend.GitHubAccountState{Account: two[0], Accounts: two, Hidden: true}, gateHidden, "github.com/octocat-work"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := accountGateOf(tt.st); got != tt.gate {
				t.Errorf("accountGateOf() = %v, want %v", got, tt.gate)
			}
			if got := defaultPick(tt.st); got != tt.pick {
				t.Errorf("defaultPick() = %q, want %q", got, tt.pick)
			}
		})
	}
}
