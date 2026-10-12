package main

import (
	"testing"

	"github.com/omartelo/lich/native/lichclient"
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
