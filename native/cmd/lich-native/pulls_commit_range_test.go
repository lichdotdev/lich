package main

import (
	"testing"

	"github.com/lichdotdev/lich/native/backend"
)

func TestCommitPickRunsFromTheAnchor(t *testing.T) {
	const url = "https://github.com/o/r/pull/1"
	commits := []backend.PRCommit{{OID: "c0"}, {OID: "c1"}, {OID: "c2"}, {OID: "c3"}}
	var k commitPick
	if _, _, ok := k.span(url, commits); ok {
		t.Fatal("a new pick spans commits, want all of them")
	}
	for _, step := range []struct {
		i        int
		extend   bool
		from, to int
	}{
		{i: 2, from: 2, to: 2},
		{i: 0, extend: true, from: 0, to: 2},
		{i: 3, extend: true, from: 2, to: 3},
		{i: 1, from: 1, to: 1},
	} {
		k.pick(url, commits, step.i, step.extend)
		if from, to, ok := k.span(url, commits); !ok || from != step.from || to != step.to {
			t.Errorf("after pick(%d, %v): span = %d..%d %v, want %d..%d", step.i, step.extend, from, to, ok, step.from, step.to)
		}
	}
	if _, _, ok := k.span("https://github.com/o/r/pull/2", commits); ok {
		t.Error("the pick spans another pull request")
	}
	if _, _, ok := k.span(url, commits[2:]); ok {
		t.Error("the pick spans after a push took its commit out")
	}
	k.pick("https://github.com/o/r/pull/2", commits, 3, true)
	if from, to, _ := k.span("https://github.com/o/r/pull/2", commits); from != 3 || to != 3 {
		t.Errorf("Shift+click on another pull request = %d..%d, want 3 alone", from, to)
	}
	k.clear()
	if _, _, ok := k.span("https://github.com/o/r/pull/2", commits); ok {
		t.Error("All commits still spans")
	}
}
