package main

import (
	"testing"

	"github.com/lichdotdev/lich/native/backend"
)

func duplicateModel() *model {
	m := newModel(nil, func() {})
	m.projects = []backend.Project{
		{ID: "p", Sessions: []backend.Session{{ID: "resumer", Label: "Claude 4"}}},
		{ID: "q", Sessions: []backend.Session{{ID: "holder", Label: "auth-refactor"}}},
	}
	return m
}

func TestDuplicateEventRaisesTheWarningNamingBothCards(t *testing.T) {
	m := duplicateModel()
	var heard []sessionDuplicate
	m.duplicate = func(d sessionDuplicate) { heard = append(heard, d) }
	send(m, duplicateEventName, sessionDuplicate{ID: "resumer", HeldBy: "holder"})
	if len(heard) != 1 {
		t.Fatalf("heard %v, want the one duplicate report", heard)
	}
	w, ok := m.duplicateOf(heard[0])
	want := duplicateWarning{id: "resumer", label: "Claude 4", holder: "holder", holderLabel: "auth-refactor", holderProject: "q"}
	if !ok || w != want {
		t.Fatalf("warning = %+v, %v; want %+v", w, ok, want)
	}
}

func TestDuplicateWarningIsDroppedOnceACardIsGone(t *testing.T) {
	m := duplicateModel()
	for _, d := range []sessionDuplicate{{ID: "closed", HeldBy: "holder"}, {ID: "resumer", HeldBy: "closed"}} {
		if w, ok := m.duplicateOf(d); ok {
			t.Errorf("duplicateOf(%+v) = %+v, want no warning about a card not on screen", d, w)
		}
	}
}
