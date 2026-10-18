package main

import (
	"context"
	"reflect"
	"testing"
)

// newDelegatesModel is newLayoutsModel with worker-a and worker-b, in
// another project, opened by the orchestrator.
func newDelegatesModel(t *testing.T) (*model, *layoutsBackend) {
	t.Helper()
	m, b := newLayoutsModel(t)
	for i := range m.projects[1].Sessions {
		m.projects[1].Sessions[i].OriginSessionID = "orch"
	}
	return m, b
}

func shownShape(t *testing.T, m *model) string {
	t.Helper()
	lt, ok := m.shownLayoutTab()
	if !ok {
		return "<none>"
	}
	return paneShape(lt.Root)
}

func TestDelegatesReachIntoOtherProjects(t *testing.T) {
	m, _ := newDelegatesModel(t)
	if got := m.delegatesOf("orch"); !reflect.DeepEqual(got, []string{"wa", "wb"}) {
		t.Fatalf("delegates %v, want both workers of lich-plugin", got)
	}
	if got := m.delegatesOf("notes"); got != nil {
		t.Fatalf("a session that opened nothing has delegates %v", got)
	}
}

func TestShowDelegatesOpensALayoutNamedAfterTheOrchestrator(t *testing.T) {
	m, b := newDelegatesModel(t)
	if !m.showDelegates(context.Background(), "orch", []string{"wa", "wb"}) {
		t.Fatal("show refused")
	}
	if got := shownShape(t, m); got != "row(orch,column(wa,wb))" {
		t.Fatalf("on screen %s", got)
	}
	if _, project, active := layoutScreen(t, m); project != "lich" || active != "orch" {
		t.Fatalf("focus on %q in %q, want the orchestrator", active, project)
	}
	saved, err := parseLayouts(b.last(t, 1))
	if err != nil || len(saved.tabs) != 1 || saved.tabs[0].Name != "orchestrator" {
		t.Fatalf("stored %+v, %v", saved.tabs, err)
	}
}

func TestShowDelegatesStacksUnderTheOnesAlreadyThere(t *testing.T) {
	m, _ := newDelegatesModel(t)
	ctx := context.Background()
	m.splitBeside(ctx, "orch", "wa", edgeRight, false)
	if target, below := m.delegateSlot("orch", m.delegatesOf("orch")); target != "wa" || !below {
		t.Fatalf("slot %q below %v, want under worker-a", target, below)
	}
	m.showDelegates(ctx, "orch", []string{"wb"})
	if got := shownShape(t, m); got != "row(orch,column(wa,wb))" {
		t.Fatalf("on screen %s", got)
	}
}

func TestShowDelegatesBesideTheOrchestratorWhenNoneIsThere(t *testing.T) {
	m, _ := newDelegatesModel(t)
	ctx := context.Background()
	m.splitBeside(ctx, "orch", "notes", edgeRight, false)
	m.showDelegates(ctx, "orch", []string{"wa"})
	if got := shownShape(t, m); got != "row(row(orch,wa),notes)" {
		t.Fatalf("on screen %s", got)
	}
}

func TestShowDelegatesTakesADelegateOutOfAnotherLayout(t *testing.T) {
	m, _ := newDelegatesModel(t)
	ctx := context.Background()
	m.splitBeside(ctx, "notes", "wb", edgeRight, false)
	m.showDelegates(ctx, "orch", []string{"wa", "wb"})
	if got := m.layoutOf("notes"); got != "" {
		t.Fatalf("notes still in layout %q after its only partner left", got)
	}
	if got := shownShape(t, m); got != "row(orch,column(wa,wb))" {
		t.Fatalf("on screen %s", got)
	}
}

func TestDelegateOfferCountsTheOnesInTheOrchestratorsLayout(t *testing.T) {
	m, _ := newDelegatesModel(t)
	ctx := context.Background()
	if o := m.delegateOffer("orch"); o.here != 0 || !reflect.DeepEqual(o.missing, []string{"wa", "wb"}) {
		t.Fatalf("alone: %+v", o)
	}
	m.splitBeside(ctx, "notes", "wb", edgeRight, false)
	if n := m.inOtherLayouts("orch", []string{"wa", "wb"}); n != 1 {
		t.Fatalf("%d in other layouts, want worker-b's", n)
	}
	m.splitBeside(ctx, "orch", "wa", edgeRight, false)
	if o := m.delegateOffer("orch"); o.here != 1 || !reflect.DeepEqual(o.missing, []string{"wb"}) {
		t.Fatalf("with worker-a beside: %+v", o)
	}
}
