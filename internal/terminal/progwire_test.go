// Reuses serviceWithSessions from the Unix-tagged tests, so it carries the
// same tag. Nothing here is platform-specific.
//go:build !windows

package terminal

import (
	"testing"
	"time"
)

// replyPTY hands every write to the test.
type replyPTY struct {
	shortWritePTY
	writes chan string
}

func (p *replyPTY) Write(b []byte) (int, error) {
	p.writes <- string(b)
	return len(b), nil
}

func feedProgram(svc *Service, id string, chunks ...string) {
	sess := svc.sessionOf(id)
	for _, c := range chunks {
		svc.readProgramStatus(id, sess, []byte(c))
	}
}

func TestProgramReportReachesTheCard(t *testing.T) {
	t.Parallel()
	svc := serviceWithSessions(t, hookStore{}, "s1")
	feedProgram(svc, "s1", "\x1b]7501;state=working:app=cargo:msg="+b64("Compiling"), "\x1b\\")
	if st := svc.Status("s1"); st.State != statusBusy || st.Tool != "cargo" || st.Detail != "Compiling" {
		t.Fatalf("status = %+v, want busy cargo · Compiling", st)
	}
	svc.programTyped("s1")
	feedProgram(svc, "s1", "\x1b]133;A\a")
	if st := svc.Status("s1"); st.State != statusIdle {
		t.Fatalf("after the prompt = %+v, want idle", st)
	}
}

func TestProgramReportYieldsToTheHook(t *testing.T) {
	t.Parallel()
	svc := serviceWithSessions(t, hookStore{}, "s1")
	svc.onHookState(hookRequest{SessionID: "s1", State: statusBusy, Tool: "Bash"})
	feedProgram(svc, "s1", "\x1b]7501;state=blocked\a")
	if st := svc.Status("s1"); st.State != statusBusy || st.Tool != "Bash" {
		t.Fatalf("status = %+v, want the hook's busy", st)
	}
	svc.onHookState(hookRequest{SessionID: "s1", State: statusIdle})
	feedProgram(svc, "s1", "\x1b]7501;state=done\a")
	if st := svc.Status("s1"); st.State != statusDone {
		t.Fatalf("after the session ended = %+v, want the program's done", st)
	}
}

func TestProgramQueryIsAnsweredWithItsTerminator(t *testing.T) {
	t.Parallel()
	svc := serviceWithSessions(t, hookStore{}, "s1")
	fake := &replyPTY{writes: make(chan string, 2)}
	svc.sessionOf("s1").pty = fake
	for _, c := range []struct{ in, want string }{
		{"\x1b]7501;?\x1b\\", "\x1b]7501;?\x1b\\"},
		{"\x1b]7501;?\a", "\x1b]7501;?\a"},
	} {
		feedProgram(svc, "s1", c.in)
		select {
		case got := <-fake.writes:
			if got != c.want {
				t.Errorf("reply %q, want %q", got, c.want)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("no reply to %q", c.in)
		}
	}
	if st := svc.Status("s1"); st.State != "" {
		t.Fatalf("a query set the status: %+v", st)
	}
}
