package main

import (
	"reflect"
	"testing"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

func artifactNames(files []backend.Artifact) []string {
	var names []string
	for _, f := range files {
		names = append(names, f.Name)
	}
	return names
}

func TestArtifactStoreTakesEachSessionsWholeList(t *testing.T) {
	var s artifactStore
	s.apply(backend.SessionArtifacts{SessionID: "a", Files: []backend.Artifact{{Name: "x.html", Modified: 1}}})
	s.apply(backend.SessionArtifacts{SessionID: "b", Files: []backend.Artifact{{Name: "y.svg", Modified: 1}}})
	s.apply(backend.SessionArtifacts{SessionID: "a", Files: []backend.Artifact{{Name: "z.html", Modified: 2}, {Name: "x.html", Modified: 1}}})
	if got := artifactNames(s.of("a")); !reflect.DeepEqual(got, []string{"z.html", "x.html"}) {
		t.Errorf("a: %v", got)
	}
	if got := artifactNames(s.of("b")); !reflect.DeepEqual(got, []string{"y.svg"}) {
		t.Errorf("b kept its own list: %v", got)
	}
	s.apply(backend.SessionArtifacts{SessionID: "a"})
	if got := s.of("a"); got != nil {
		t.Errorf("a closed and kept %v", got)
	}
}

func TestArtifactUnreadUntilOpenedAndAgainWhenRewritten(t *testing.T) {
	var s artifactStore
	s.apply(backend.SessionArtifacts{SessionID: "a", Files: []backend.Artifact{{Name: "x.html", Modified: 1}}})
	if !s.isUnread("a", "x.html") {
		t.Fatal("a new file reads as read")
	}
	s.apply(backend.SessionArtifacts{SessionID: "a", Files: []backend.Artifact{{Name: "y.html", Modified: 2}, {Name: "x.html", Modified: 1}}})
	if !s.isUnread("a", "x.html") {
		t.Error("an unopened file went read when another arrived")
	}
	s.markRead("a", "x.html")
	if s.isUnread("a", "x.html") {
		t.Error("opened and still unread")
	}
	s.apply(backend.SessionArtifacts{SessionID: "a", Files: []backend.Artifact{{Name: "x.html", Modified: 3}, {Name: "y.html", Modified: 2}}})
	if !s.isUnread("a", "x.html") {
		t.Error("rewritten and still read")
	}
}

func TestArtifactGroupsAreTheActiveSessionsThenEachPeerWithFiles(t *testing.T) {
	lists := map[string][]backend.Artifact{
		"me":    {{Name: "mine.html"}},
		"other": {{Name: "theirs.html"}},
	}
	peers := []backend.Session{{ID: "empty"}, {ID: "me"}, {ID: "other", Label: "Auth refactor"}}
	own, others := artifactGroups("me", peers, func(id string) []backend.Artifact { return lists[id] })
	if got := artifactNames(own); !reflect.DeepEqual(got, []string{"mine.html"}) {
		t.Errorf("own: %v", got)
	}
	if len(others) != 1 || others[0].session.ID != "other" || artifactNames(others[0].files)[0] != "theirs.html" {
		t.Errorf("others: %+v", others)
	}
	if n := artifactCount(own, others); n != 2 {
		t.Errorf("count %d", n)
	}
}

func menuLabels(entries []ui.MenuEntry) []string {
	var labels []string
	for _, e := range entries {
		if e.Kind == ui.MenuSeparator {
			labels = append(labels, "---")
			continue
		}
		labels = append(labels, e.Label)
	}
	return labels
}

func TestArtifactMenuLeadsWithOpenOnItsOwnSessionAndAttachOnAnother(t *testing.T) {
	own := menuLabels(artifactMenu(true, artifactActions{}))
	if want := []string{"Open in browser", "Attach to prompt", "---", "Copy path", "Show in file manager"}; !reflect.DeepEqual(own, want) {
		t.Errorf("own session: %v", own)
	}
	other := menuLabels(artifactMenu(false, artifactActions{}))
	if want := []string{"Attach to prompt", "Open in browser", "---", "Copy path", "Show in file manager"}; !reflect.DeepEqual(other, want) {
		t.Errorf("other session: %v", other)
	}
}
