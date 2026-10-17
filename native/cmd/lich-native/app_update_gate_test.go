package main

import (
	"testing"

	"github.com/lichdotdev/lich/native/lichclient"
	"github.com/lichdotdev/lich/native/ui"
)

func TestShouldPromptUpdate(t *testing.T) {
	newer := lichclient.AppUpdateStatus{UpdateAvailable: true, LatestVersion: "1.4.0"}
	if !shouldPromptUpdate(newer, "") || !shouldPromptUpdate(newer, "1.3.0") {
		t.Error("a newer release Later was not clicked for did not prompt")
	}
	if shouldPromptUpdate(newer, "1.4.0") {
		t.Error("a release Later was clicked for prompted again")
	}
	if shouldPromptUpdate(lichclient.AppUpdateStatus{LatestVersion: "1.4.0"}, "") {
		t.Error("an up-to-date lich prompted")
	}
}

func TestUpdateProgressToast(t *testing.T) {
	cases := []struct {
		name        string
		p           *lichclient.AppUpdateProgress
		title, desc string
		bar         *ui.ToastProgress
	}{
		{"before the first step", nil, "Downloading lich 1.4.0", "", &ui.ToastProgress{Percent: -1}},
		{"known total", &lichclient.AppUpdateProgress{Phase: "download", Received: 12_300_000, Total: 41_000_000}, "Downloading lich 1.4.0", "30% · 12.3 MB of 41.0 MB", &ui.ToastProgress{Percent: 30}},
		{"no Content-Length", &lichclient.AppUpdateProgress{Phase: "download", Received: 5_000_000, Total: -1}, "Downloading lich 1.4.0", "5.0 MB", &ui.ToastProgress{Percent: -1}},
		{"installer", &lichclient.AppUpdateProgress{Phase: "installer"}, "Installing lich 1.4.0…", "lich closes and reopens on its own.", nil},
	}
	for _, c := range cases {
		got := updateProgressToast("1.4.0", c.p)
		if got.Title != c.title || got.Description != c.desc {
			t.Errorf("%s: got (%q, %q), want (%q, %q)", c.name, got.Title, got.Description, c.title, c.desc)
		}
		if (got.Progress == nil) != (c.bar == nil) || got.Progress != nil && *got.Progress != *c.bar {
			t.Errorf("%s: bar %+v, want %+v", c.name, got.Progress, c.bar)
		}
	}
}

func TestUpdateProgressToastSpinsOnlyThroughTheInstaller(t *testing.T) {
	if got := updateProgressToast("1.4.0", &lichclient.AppUpdateProgress{Phase: "installer"}); !got.Spin {
		t.Error("installer: no spinner")
	}
	for _, p := range []*lichclient.AppUpdateProgress{nil, {Phase: "download", Received: 1, Total: 2}} {
		if got := updateProgressToast("1.4.0", p); got.Icon != nil || got.Spin {
			t.Errorf("%+v: icon %v spin %v, want no glyph while downloading", p, got.Icon, got.Spin)
		}
	}
}

func TestUpdateFailureTextNamesThePhase(t *testing.T) {
	cases := []struct {
		last *lichclient.AppUpdateProgress
		want string
	}{
		{nil, "Update failed: boom"},
		{&lichclient.AppUpdateProgress{Phase: "installer"}, "Install failed: boom"},
		{&lichclient.AppUpdateProgress{Phase: "download", Received: 50, Total: 100}, "Download failed at 50%: boom"},
		{&lichclient.AppUpdateProgress{Phase: "download", Received: 50, Total: -1}, "Download failed: boom"},
	}
	for _, c := range cases {
		if got := updateFailureText(c.last, "boom"); got != c.want {
			t.Errorf("last %+v: got %q, want %q", c.last, got, c.want)
		}
	}
}
