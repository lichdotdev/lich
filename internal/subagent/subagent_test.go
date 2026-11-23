package subagent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/prompt"
)

func spawnReq(id string, tasks ...api.SubagentTask) api.SubagentRequest {
	if len(tasks) == 0 {
		tasks = []api.SubagentTask{{Title: "Fix the handshake race", Prompt: "Fix #412.", Type: "general-purpose"}}
	}
	return api.SubagentRequest{SessionID: "s1", ProviderSessionID: "p1", Call: api.SubagentSpawn, Tool: "Agent", ToolUseID: id, Tasks: tasks}
}

// A repeat of the same tool call answers the same and opens nothing more.
func TestARepeatOpensNothing(t *testing.T) {
	svc, w, _ := newFake(Asker{Kind: "claude"})
	_, commit := svc.Decide(context.Background(), spawnReq("toolu_1"))
	commit(true)
	svc.Decide(context.Background(), spawnReq("toolu_1"))
	if len(w.opened) != 1 || len(w.sent) != 1 {
		t.Errorf("opened %d and sent %d, want one each", len(w.opened), len(w.sent))
	}
}

// A deny whose answer never reached the hook closes what it opened, as the
// asker, and the conversation's later wait runs natively.
func TestAnUnwrittenDenyClosesWhatItOpened(t *testing.T) {
	svc, w, vetoes := newFake(Asker{Kind: "claude"})
	answer, commit := svc.Decide(context.Background(), spawnReq("toolu_1"))
	if answer.Decision != api.SubagentDeny {
		t.Fatalf("answer = %+v", answer)
	}
	commit(false)
	if len(w.closed) != 1 || w.closed[0].From != "s1" || w.closed[0].Target != "w1" {
		t.Errorf("closed = %+v, want w1 closed by s1", w.closed)
	}
	if len(*vetoes) != 0 {
		t.Errorf("an unwritten deny was told as a veto: %+v", *vetoes)
	}
	wait := api.SubagentRequest{SessionID: "s1", ProviderSessionID: "p1", Call: api.SubagentWait, Tool: "wait"}
	if got, _ := svc.Decide(context.Background(), wait); got.Decision != api.SubagentAllow {
		t.Errorf("wait after an unwritten deny = %+v, want allow", got)
	}
}

// A written deny is told as a veto, with the sessions it opened, and kept
// for a window that attaches later.
func TestAWrittenDenyIsAVeto(t *testing.T) {
	svc, _, vetoes := newFake(Asker{Kind: "claude"})
	_, commit := svc.Decide(context.Background(), spawnReq("toolu_1"))
	commit(true)
	want := api.SubagentVeto{SessionID: "s1", ToolUseID: "toolu_1", Sessions: []api.SubagentOpened{fixtureSessions[0]}}
	if len(*vetoes) != 1 || (*vetoes)[0].ToolUseID != want.ToolUseID || (*vetoes)[0].Sessions[0] != want.Sessions[0] {
		t.Errorf("vetoes = %+v, want %+v", *vetoes, want)
	}
	if kept, _ := svc.Vetoed(context.Background(), api.None{}); len(kept) != 1 {
		t.Errorf("Vetoed = %+v, want the one veto", kept)
	}
}

// A call is opened whole or not at all: a task that fails to open closes the
// ones before it and the call runs natively.
func TestAFailedTaskClosesTheOthers(t *testing.T) {
	svc, w, _ := newFake(Asker{Kind: "kiro"})
	w.failAt = 2
	tasks := []api.SubagentTask{{Prompt: "Map the relay."}, {Prompt: "Fix #412."}}
	if got, _ := svc.Decide(context.Background(), spawnReq("t1", tasks...)); got.Decision != api.SubagentAllow {
		t.Fatalf("answer = %+v, want allow", got)
	}
	if len(w.closed) != 1 || w.closed[0].Target != "w1" {
		t.Errorf("closed = %+v, want w1", w.closed)
	}
}

// slowWorld opens a session only after the decision's budget ran out.
type slowWorld struct {
	*fakeWorld
	release chan struct{}
}

func (s slowWorld) OpenSubagentNow(ctx context.Context, opts api.OpenSubagentOptions) (api.OpenedSession, error) {
	<-s.release
	return s.fakeWorld.OpenSubagentNow(ctx, opts)
}

// A call that cannot be opened in time is allowed, and a session that opens
// after that is closed rather than left running beside the native subagent.
func TestAnOpenPastTheBudgetIsClosed(t *testing.T) {
	w := &fakeWorld{asker: Asker{Kind: "claude"}}
	slow := slowWorld{fakeWorld: w, release: make(chan struct{})}
	svc := New(w, slow, w, func() prompt.Lang { return prompt.English }, nil)
	svc.budget = 20 * time.Millisecond
	if got, _ := svc.Decide(context.Background(), spawnReq("toolu_1")); got.Decision != api.SubagentAllow {
		t.Fatalf("answer = %+v, want allow past the budget", got)
	}
	close(slow.release)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		w.mu.Lock()
		closed := len(w.closed)
		w.mu.Unlock()
		if closed == 1 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("closed = %+v, want the late session closed", w.closed)
}

// Isolation gets each task a worktree of its own, named after its title, and
// the model asked for is the worker's.
func TestIsolationAndModelReachTheOpen(t *testing.T) {
	svc, w, _ := newFake(Asker{Kind: "claude"})
	req := spawnReq("toolu_1")
	req.Isolation, req.Model = "worktree", " opus "
	_, commit := svc.Decide(context.Background(), req)
	commit(false)
	if got := w.opened[0]; !strings.HasPrefix(got.Worktree, "subagent/fix-the-handshake-race-") || got.Model != "opus" {
		t.Errorf("opened %+v", got)
	}
	if w.closed[0].Worktree != removeWorktree {
		t.Errorf("an isolated card closed with worktree %q, want it removed", w.closed[0].Worktree)
	}
}

// A retry with a new tool call and the same prompt names the card already
// working on it, and is told as a veto so its chat draws that call as opened.
func TestARetryWithTheSamePromptIsAVetoToo(t *testing.T) {
	svc, w, vetoes := newFake(Asker{Kind: "claude"})
	_, commit := svc.Decide(context.Background(), spawnReq("toolu_1"))
	commit(true)
	answer, commit := svc.Decide(context.Background(), spawnReq("toolu_2"))
	commit(true)
	if answer.Decision != api.SubagentDeny || len(w.opened) != 1 {
		t.Fatalf("answer %+v after %d opens, want a deny opening nothing", answer, len(w.opened))
	}
	if len(*vetoes) != 2 || (*vetoes)[1].ToolUseID != "toolu_2" {
		t.Errorf("vetoes = %+v, want the retry told too", *vetoes)
	}
}

// Once the card answered, the same prompt is a new task and gets a card of
// its own: a deny naming a finished session would leave its report unsent.
func TestTheSamePromptAfterTheWorkerAnsweredOpensAgain(t *testing.T) {
	svc, w, _ := newFake(Asker{Kind: "claude"})
	_, commit := svc.Decide(context.Background(), spawnReq("toolu_1"))
	commit(true)
	w.answered = map[string]bool{fixtureSessions[0].Ticket: true}
	_, commit = svc.Decide(context.Background(), spawnReq("toolu_2"))
	commit(true)
	if len(w.opened) != 2 {
		t.Errorf("opened %d, want a second card", len(w.opened))
	}
}

// "inherit" (Cursor CLI's Task tool sends it) and an empty model both mean the
// provider's default: no model reaches the open, so no --model is spawned.
func TestInheritIsTheProvidersDefaultModel(t *testing.T) {
	for _, model := range []string{"inherit", " Inherit ", "", "  "} {
		svc, w, _ := newFake(Asker{Kind: "cursor"})
		req := spawnReq("toolu_1")
		req.Model = model
		_, commit := svc.Decide(context.Background(), req)
		commit(true)
		if got := w.opened[0].Model; got != "" {
			t.Errorf("model %q opened with %q, want the provider's default", model, got)
		}
	}
}
