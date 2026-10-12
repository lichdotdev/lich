package lichclient

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestGitHubAccountsReadsNullAsSignedInToNone(t *testing.T) {
	c := fakeBackend(t, func(method string, _ []json.RawMessage) (any, int) {
		if method != "project.GitHubAccounts" {
			t.Errorf("method %s", method)
		}
		return nil, http.StatusOK
	})
	accounts, err := c.GitHubAccounts(context.Background())
	if err != nil || accounts == nil || len(accounts) != 0 {
		t.Fatalf("got %#v, %v; want an empty list", accounts, err)
	}
}

func TestBinaryCheckFailedSparesOkAndEmpty(t *testing.T) {
	for status, want := range map[string]bool{CheckOK: false, CheckEmpty: false, "not-found": true, "relative": true} {
		if got := (BinaryCheck{Status: status}).Failed(); got != want {
			t.Errorf("%s: Failed = %v, want %v", status, got, want)
		}
	}
}
