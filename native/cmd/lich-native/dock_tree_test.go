package main

import (
	"context"
	"reflect"
	"testing"
)

func TestFilterPathsMatchesEveryTokenAnywhereInThePath(t *testing.T) {
	paths := []string{"frontend/src/App.tsx", "native/cmd/main.go", "frontend/src/main.tsx"}
	got := filterPaths(paths, "front MAIN")
	if !reflect.DeepEqual(got, []string{"frontend/src/main.tsx"}) {
		t.Errorf("filter = %v", got)
	}
	if got := filterPaths(paths, "  "); len(got) != len(paths) {
		t.Errorf("a blank filter kept %d of %d", len(got), len(paths))
	}
}

func TestTreeFootnoteNamesTheHiddenBeforeTheCut(t *testing.T) {
	if got := treeFootnote(false, nil); got != "" {
		t.Errorf("a whole listing owes %q", got)
	}
	got := treeFootnote(true, []string{"build", "node_modules"})
	want := "Hidden: build, node_modules. This folder has more files than the tree can list."
	if got != want {
		t.Errorf("footnote = %q, want %q", got, want)
	}
}

func TestRemoteKeepsTheLastAnswerUntilTheNextLandsButDropsItAcrossAReset(t *testing.T) {
	var r remote[string]
	answered := make(chan struct{})
	r.invalidate = func() { answered <- struct{}{} }
	r.want("k1", "repo", func(context.Context) (string, error) { return "one", nil })
	<-answered
	block := make(chan struct{})
	r.want("k2", "repo", func(context.Context) (string, error) { <-block; return "two", nil })
	if data, loading, _ := r.get(); data != "one" || loading {
		t.Errorf("during a refetch = %q loading=%v, want the last answer", data, loading)
	}
	r.want("k3", "other", func(context.Context) (string, error) { <-block; return "three", nil })
	if data, loading, _ := r.get(); data != "" || !loading {
		t.Errorf("after a reset = %q loading=%v, want nothing and loading", data, loading)
	}
	close(block)
	<-answered
	if data, _, _ := r.get(); data != "three" {
		t.Errorf("answer = %q, want the latest key's", data)
	}
}
