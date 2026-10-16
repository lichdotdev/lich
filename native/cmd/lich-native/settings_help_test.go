package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// TestBugReportURLPrefillsTheForm pins the query keys to the field ids of
// .github/ISSUE_TEMPLATE/bug.yml, as support-url.test.ts does for the web.
func TestBugReportURLPrefillsTheForm(t *testing.T) {
	got := bugReportURL(lichclient.Diagnostics{Version: "1.2.3", Platform: "linux/amd64"})
	want := "https://github.com/omartelo/lich/issues/new?platform=linux%2Famd64&template=bug.yml&version=1.2.3"
	if got != want {
		t.Fatalf("bugReportURL = %s, want %s", got, want)
	}
}

func TestRevealLogOutcomeReplacesTheLoadingToast(t *testing.T) {
	if got := revealLogOutcome("toast-3", nil); got.ID != "toast-3" || got.Variant != ui.ToastSuccess || got.Title != helpLogOpened {
		t.Errorf("success = %+v, want the opened toast in toast-3's place", got)
	}
	if got := revealLogOutcome("toast-3", errors.New("no file manager")); got.ID != "toast-3" || got.Variant != ui.ToastError || !strings.Contains(got.Title, "no file manager") {
		t.Errorf("failure = %+v, want the error in toast-3's place", got)
	}
}
