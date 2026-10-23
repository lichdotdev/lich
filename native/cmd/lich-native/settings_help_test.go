package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

// TestBugReportURLPrefillsTheForm pins the query keys to the field ids of
// .github/ISSUE_TEMPLATE/bug.yml.
func TestBugReportURLPrefillsTheForm(t *testing.T) {
	got := bugReportURL(backend.Diagnostics{Version: "1.2.3", Platform: "linux/amd64"})
	want := "https://github.com/lichdotdev/lich/issues/new?platform=linux%2Famd64&template=bug.yml&version=1.2.3"
	if got != want {
		t.Fatalf("bugReportURL = %s, want %s", got, want)
	}
}

func TestRevealLogOutcomeReplacesTheLoadingToast(t *testing.T) {
	if got := revealLogOutcome("toast-3", nil); got.ID != "toast-3" || got.Variant != ui.ToastSuccess || got.Title != "Log folder opened." {
		t.Errorf("success = %+v, want the opened toast in toast-3's place", got)
	}
	if got := revealLogOutcome("toast-3", errors.New("no file manager")); got.ID != "toast-3" || got.Variant != ui.ToastError || !strings.Contains(got.Title, "no file manager") {
		t.Errorf("failure = %+v, want the error in toast-3's place", got)
	}
}
