package terminal

import "testing"

func TestProgramStatusShowsTheMostUrgentRecord(t *testing.T) {
	t.Parallel()
	var p programStatus
	p.apply(progReport{state: progWorking, app: "ci", msg: "pipeline"})
	p.apply(progReport{state: progWorking, id: "ci/build", msg: "compiling"})
	ev, ok := p.next("s1")
	if !ok || ev != (statusEvent{ID: "s1", State: statusBusy, Tool: "ci", Detail: "pipeline"}) {
		t.Fatalf("root first among equals: %+v %v", ev, ok)
	}
	p.apply(progReport{state: progBlocked, id: "ci/deploy", msg: "Approve?"})
	ev, ok = p.next("s1")
	if !ok || ev != (statusEvent{ID: "s1", State: statusWaiting, Reason: "Approve?"}) {
		t.Fatalf("blocked outranks working: %+v %v", ev, ok)
	}
	p.apply(progReport{state: progClear, id: "ci"})
	ev, ok = p.next("s1")
	if !ok || ev != (statusEvent{ID: "s1", State: statusBusy, Tool: "ci", Detail: "pipeline"}) {
		t.Fatalf("clear removes the id and everything under it: %+v %v", ev, ok)
	}
}

func TestProgramStatusInheritsTheAncestorsApp(t *testing.T) {
	t.Parallel()
	var p programStatus
	p.apply(progReport{state: progIdle, app: "deploy"})
	p.apply(progReport{state: progWorking, id: "us/east", msg: "rolling"})
	ev, _ := p.next("s1")
	if ev.Tool != "deploy" || ev.Detail != "rolling" {
		t.Fatalf("event = %+v, want deploy · rolling", ev)
	}
}

func TestProgramStatusLifetime(t *testing.T) {
	t.Parallel()
	var p programStatus
	p.apply(progReport{state: progWorking, id: "a"})
	p.apply(progReport{state: progDone, id: "b"})
	if !p.typed() || p.typed() {
		t.Fatal("typing ends done once")
	}
	p.apply(progReport{state: progError, id: "c"})
	if !p.promptStarted() {
		t.Fatal("a prompt ends working")
	}
	if ev, _ := p.next("s1"); ev.State != statusDone {
		t.Fatalf("a prompt keeps error: %+v", ev)
	}
	if !p.reset() || len(p.recs) != 0 {
		t.Fatal("RIS ends everything")
	}
	if ev, ok := p.next("s1"); !ok || ev.State != statusIdle {
		t.Fatalf("no records left clears the card: %+v %v", ev, ok)
	}
	if _, ok := p.next("s1"); ok {
		t.Fatal("the clear is told once")
	}
}

func TestProgramStatusEvictsTheOldestRecord(t *testing.T) {
	t.Parallel()
	var p programStatus
	p.apply(progReport{state: progBlocked, id: "first"})
	for i := range progMaxRecords {
		p.apply(progReport{state: progIdle, id: "r" + string(rune('a'+i%26)) + string(rune('a'+i/26))})
	}
	if _, ok := p.recs["first"]; ok || len(p.recs) != progMaxRecords {
		t.Fatalf("held %d records, first kept %v", len(p.recs), ok)
	}
}

func TestProgramStatusYieldsToAHook(t *testing.T) {
	t.Parallel()
	var p programStatus
	p.setHooked(true)
	p.apply(progReport{state: progWorking})
	if _, ok := p.next("s1"); ok {
		t.Fatal("a program spoke over a hook")
	}
	p.setHooked(false)
	if ev, ok := p.next("s1"); !ok || ev.State != statusBusy {
		t.Fatalf("after the hook left: %+v %v", ev, ok)
	}
	// Codex, opencode and most others never report the session ended: the
	// shell's prompt coming back is when their CLI has gone.
	p.setHooked(true)
	p.promptStarted()
	p.apply(progReport{state: progWorking, app: "make"})
	if ev, ok := p.next("s1"); !ok || ev.Tool != "make" {
		t.Fatalf("after the shell's prompt: %+v %v", ev, ok)
	}
}

func TestProgramStatusPublishesEveryChangeAtOnce(t *testing.T) {
	t.Parallel()
	var p programStatus
	for _, state := range []string{progBlocked, progWorking, progBlocked, progDone, progError, progWorking} {
		p.apply(progReport{state: state, msg: state})
		if _, ok := p.next("s1"); !ok && state != progError {
			t.Fatalf("%s was not published", state)
		}
	}
	p.apply(progReport{state: progWorking, msg: progWorking})
	if _, ok := p.next("s1"); ok {
		t.Fatal("an unchanged state was published again")
	}
}
