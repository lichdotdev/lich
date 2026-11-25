package main

import "testing"

func TestLatestSendsOneAtATimeAndKeepsTheNewest(t *testing.T) {
	var l latest
	if !l.offer("a") {
		t.Fatal("the first drawing waited with none running")
	}
	for _, s := range []string{"b", "c"} {
		if l.offer(s) {
			t.Fatalf("%q went while a ran", s)
		}
	}
	if s, ok := l.done(); !ok || s != "c" {
		t.Fatalf("after a: %q, %v; want c, true", s, ok)
	}
	if s, ok := l.done(); ok {
		t.Fatalf("after c: %q waited, none was asked for", s)
	}
	if !l.offer("d") {
		t.Fatal("a drawing waited once the page was idle")
	}
}
