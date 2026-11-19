package main

import "testing"

func TestDiffFeedKeepsItsReadWhenTheSameCheckoutIsFollowedAgain(t *testing.T) {
	_, client := newSidebarBackend(t)
	f := &diffFeed{client: client, invalidate: func() {}}
	f.path, f.read = "/repo", &diffRead{text: "diff"}

	f.follow("/repo")
	defer f.close()
	if f.latest() == nil {
		t.Error("coming back to the same checkout dropped the diff the panel had shown")
	}
	f.close()
	f.follow("/other")
	if f.latest() != nil {
		t.Error("another checkout kept the read of the first")
	}
}
