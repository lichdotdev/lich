//go:build !darwin && !windows

package quota

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// With no XDG_CONFIG_HOME, cursor-agent writes its login to
// ~/.config/cursor/auth.json while its config dir is ~/.cursor: the gauge reads
// the login from the first and the account's address from the second
// (issue #765).
func TestCursorReadsTheLoginUnderDotConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv(accountHomeVar, home)
	t.Setenv(configHomeVar, "")
	t.Setenv(cursorConfigDirVar, "")
	token := cursorToken("auth0|user_01", cursorNow.Add(time.Hour))
	files := map[string]string{
		filepath.Join(home, ".config", "cursor", "auth.json"): fmt.Sprintf(`{"accessToken":%q}`, token),
		filepath.Join(home, ".cursor", "cli-config.json"):     `{"authInfo":{"email":"dev@example.com"}}`,
	}
	for path, body := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	url, _ := serve(t, http.StatusOK, cursorSummary)
	got := cursorService(url).cursorPlan(lichEnv())
	if got.Status != StatusOK || got.Account != "dev@example.com" {
		t.Errorf("plan = %+v, want the login under ~/.config/cursor read", got)
	}
}
