// Reuses serviceWithSessions from the Unix-tagged tests, so it carries the
// same tag. Nothing here is platform-specific.
//go:build !windows

package terminal

import (
	"context"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/lichdotdev/lich/api"
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

// settleProgram waits for the session's publisher to finish.
func settleProgram(t *testing.T, sess *session) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		sess.prog.mu.Lock()
		running := sess.prog.running
		sess.prog.mu.Unlock()
		if !running {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("the program status publisher never finished")
}

func TestProgramReportReachesTheCard(t *testing.T) {
	t.Parallel()
	svc, sess := programService(t, hookStore{})
	feed(t, svc, sess, "\x1b]7501;state=working:app=cargo:msg="+b64("Compiling"), "\x1b\\")
	if st := svc.Status("s1"); st.State != statusBusy || st.Tool != "cargo" || st.Detail != "Compiling" {
		t.Fatalf("status = %+v, want busy cargo · Compiling", st)
	}
	svc.programTyped("s1")
	feed(t, svc, sess, "\x1b]133;A\a")
	if st := svc.Status("s1"); st.State != statusIdle {
		t.Fatalf("after the prompt = %+v, want idle", st)
	}
}

func TestProgramReportYieldsToTheHook(t *testing.T) {
	t.Parallel()
	svc, sess := programService(t, hookStore{})
	svc.onHookState(hookRequest{SessionID: "s1", State: statusBusy, Tool: "Bash"})
	feed(t, svc, sess, "\x1b]7501;state=blocked\a")
	if st := svc.Status("s1"); st.State != statusBusy || st.Tool != "Bash" {
		t.Fatalf("status = %+v, want the hook's busy", st)
	}
	svc.onHookState(hookRequest{SessionID: "s1", State: statusIdle})
	feed(t, svc, sess, "\x1b]7501;state=done\a")
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

// programService is a service over s1 whose PTY swallows what is written.
func programService(t *testing.T, store Store) (*Service, *session) {
	t.Helper()
	svc := serviceWithSessions(t, store, "s1")
	sess := svc.sessionOf("s1")
	sess.pty = &shortWritePTY{chunk: 1 << 16}
	return svc, sess
}

// feed reads chunks as the stream goroutine does and waits for what they
// published to go out.
func feed(t *testing.T, svc *Service, sess *session, chunks ...string) {
	t.Helper()
	feedProgram(svc, "s1", chunks...)
	settleProgram(t, sess)
}

func TestAnEmulatorReplyLeavesAFinishedRecord(t *testing.T) {
	t.Parallel()
	svc, sess := programService(t, hookStore{})
	feed(t, svc, sess, "\x1b]7501;state=error:msg="+b64("Linker failed")+"\a")
	for _, reply := range []string{"\x1b[?62;22c", "\x1b[12;40R", "\x1b[<35;10;5M"} {
		svc.onInput("s1", []byte(reply))
		settleProgram(t, sess)
		if st := svc.Status("s1"); st.State != statusDone {
			t.Fatalf("after %q: %+v, want the finished record kept", reply, st)
		}
	}
}

// blockedPTY never finishes a write, the way a child not reading stdin does.
type blockedPTY struct {
	shortWritePTY
	entered atomic.Int32
	release chan struct{}
}

func (p *blockedPTY) Write(b []byte) (int, error) {
	p.entered.Add(1)
	<-p.release
	return len(b), nil
}

func TestQueriesToAChildNotReadingUseOneWriter(t *testing.T) {
	t.Parallel()
	svc, sess := programService(t, hookStore{})
	fake := &blockedPTY{release: make(chan struct{})}
	sess.pty = fake
	t.Cleanup(func() { close(fake.release) })
	feedProgram(svc, "s1", strings.Repeat("\x1b]7501;?\a", 200))
	deadline := time.Now().Add(5 * time.Second)
	for fake.entered.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	if n := fake.entered.Load(); n != 1 {
		t.Fatalf("%d writes blocked on the PTY, want 1", n)
	}
}

// slowUnread holds a write of unread=true until released.
type slowUnread struct {
	hookStore
	entered chan struct{}
	release chan struct{}
}

func (s slowUnread) SetSessionUnread(_ context.Context, opts api.SessionUnreadOptions) (api.None, error) {
	if opts.Unread {
		s.entered <- struct{}{}
		<-s.release
	}
	return api.None{}, nil
}

func TestAStalledPublishDoesNotHoldTheReadLoop(t *testing.T) {
	t.Parallel()
	store := slowUnread{entered: make(chan struct{}, 1), release: make(chan struct{})}
	svc, _ := programService(t, store)
	defer close(store.release)
	read := make(chan struct{})
	go func() {
		feedProgram(svc, "s1", "\x1b]7501;state=done\a")
		close(read)
	}()
	select {
	case <-read:
	case <-time.After(2 * time.Second):
		t.Fatal("the PTY read waited on the store")
	}
}

func TestDoneFlipsStopWritingUnreadPastTheBurst(t *testing.T) {
	t.Parallel()
	store := newRecordedUnread()
	svc, sess := programService(t, store)
	for range 5 {
		feed(t, svc, sess, "\x1b]7501;state=done\a")
		feed(t, svc, sess, "\x1b]7501;state=working\a")
	}
	if got := store.seen(); len(got) != progUnreadBurst {
		t.Fatalf("unread writes %v, want %d", got, progUnreadBurst)
	}
}

func TestAStaleProgramReportCannotLandAfterTheHook(t *testing.T) {
	t.Parallel()
	store := slowUnread{entered: make(chan struct{}, 1), release: make(chan struct{})}
	svc, sess := programService(t, store)
	read := make(chan struct{})
	go func() {
		feedProgram(svc, "s1", "\x1b]7501;state=done\a")
		close(read)
	}()
	<-store.entered
	svc.onHookState(hookRequest{SessionID: "s1", State: statusBusy, Tool: "Bash"})
	close(store.release)
	<-read
	settleProgram(t, sess)
	if st := svc.Status("s1"); st.State != statusBusy {
		t.Fatalf("status = %+v, want the hook's busy last", st)
	}
}

func TestAHeldBackWaitingDoesNotSilenceTheProgram(t *testing.T) {
	t.Parallel()
	svc, sess := programService(t, hookStore{})
	feed(t, svc, sess, "\x1b]7501;state=working:app=make\a")
	svc.onHookState(hookRequest{SessionID: "s1", State: statusWaiting})
	feed(t, svc, sess, "\x1b]7501;state=blocked:msg="+b64("Approve?")+"\a")
	if st := svc.Status("s1"); st.State != statusWaiting || st.Reason != "Approve?" {
		t.Fatalf("status = %+v, want the program's blocked", st)
	}
}

func TestAWorkingRecordOutranksAFinishedOne(t *testing.T) {
	t.Parallel()
	svc, sess := programService(t, hookStore{})
	feed(t, svc, sess, "\x1b]7501;state=done:id=build\a", "\x1b]7501;state=working:id=test:app=go\a")
	if st := svc.Status("s1"); st.State != statusBusy || st.Tool != "go" {
		t.Fatalf("status = %+v, want busy while tests run", st)
	}
	feed(t, svc, sess, "\x1b]7501;state=error:id=lint\a")
	if st := svc.Status("s1"); st.State != statusBusy {
		t.Fatalf("status = %+v, want busy: an error reads as finished on the card", st)
	}
}

func TestAWorkingReportWithoutALabelClearsTheTool(t *testing.T) {
	t.Parallel()
	svc, sess := programService(t, hookStore{})
	feed(t, svc, sess, "\x1b]7501;state=working:app=cargo:msg="+b64("Compiling")+"\a")
	feed(t, svc, sess, "\x1b]7501;state=working\a")
	if st := svc.Status("s1"); st.State != statusBusy || st.Tool != "" || st.Detail != "" {
		t.Fatalf("status = %+v, want busy with no tool", st)
	}
}

func wsPair(t *testing.T) (*websocket.Conn, chan string, chan string) {
	t.Helper()
	inputs, typed := make(chan string, 4), make(chan string, 4)
	tr, err := newTransport(func(id string, data []byte) { inputs <- id + ":" + string(data) }, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}
	tr.setTyped(func(id string) { typed <- id })
	conn, err := dial(t, tr, tr.token)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(websocket.StatusNormalClosure, "") })
	return conn, inputs, typed
}

func wsWrite(t *testing.T, conn *websocket.Conn, kind websocket.MessageType, data []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Write(ctx, kind, data); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func recvString(t *testing.T, ch chan string, what string) string {
	t.Helper()
	select {
	case got := <-ch:
		return got
	case <-time.After(5 * time.Second):
		t.Fatalf("no %s", what)
		return ""
	}
}

// Only the text frame says somebody typed. The binary format is what it
// always was, which is what a NUL typed with Ctrl+Space rides on.
func TestATextFrameIsTypingAndBinaryFramesAreUnchanged(t *testing.T) {
	t.Parallel()
	conn, inputs, typed := wsPair(t)
	reply, _ := encodeFrame("s1", []byte("\x1b[?62c"))
	nul, _ := encodeFrame("s1", []byte{0})
	wsWrite(t, conn, websocket.MessageBinary, reply)
	if got := recvString(t, inputs, "reply"); got != "s1:\x1b[?62c" {
		t.Fatalf("input %q", got)
	}
	wsWrite(t, conn, websocket.MessageText, []byte("s1"))
	if got := recvString(t, typed, "typed"); got != "s1" {
		t.Fatalf("typed %q", got)
	}
	wsWrite(t, conn, websocket.MessageBinary, nul)
	if got := recvString(t, inputs, "NUL"); got != "s1:\x00" {
		t.Fatalf("NUL reached the PTY as %q", got)
	}
	if len(typed) != 0 || len(inputs) != 0 {
		t.Fatalf("%d typed, %d inputs left over", len(typed), len(inputs))
	}
}

func TestABinaryFrameStartingWithNULIsStillRefused(t *testing.T) {
	t.Parallel()
	conn, inputs, typed := wsPair(t)
	good, _ := encodeFrame("s1", []byte("k"))
	wsWrite(t, conn, websocket.MessageBinary, append([]byte{0}, good...))
	wsWrite(t, conn, websocket.MessageBinary, good)
	if got := recvString(t, inputs, "the frame behind it"); got != "s1:k" {
		t.Fatalf("input %q, want the zero-led frame dropped", got)
	}
	if len(typed) != 0 || len(inputs) != 0 {
		t.Fatalf("%d typed, %d inputs left over", len(typed), len(inputs))
	}
}

// recordedUnread keeps every unread write in order.
type recordedUnread struct {
	hookStore
	mu     *sync.Mutex
	writes *[]bool
}

func newRecordedUnread() recordedUnread {
	return recordedUnread{mu: &sync.Mutex{}, writes: &[]bool{}}
}

func (s recordedUnread) SetSessionUnread(_ context.Context, opts api.SessionUnreadOptions) (api.None, error) {
	s.mu.Lock()
	*s.writes = append(*s.writes, opts.Unread)
	s.mu.Unlock()
	return api.None{}, nil
}

func (s recordedUnread) seen() []bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(*s.writes)
}

// A value held back and written later would undo a card read in between, so
// each change is written when it happens or not at all.
func TestAProgramsDoneIsMarkedUnreadWhenItHappens(t *testing.T) {
	t.Parallel()
	store := newRecordedUnread()
	svc, sess := programService(t, store)
	feed(t, svc, sess, "\x1b]7501;state=done\a")
	feed(t, svc, sess, "\x1b]7501;state=working\a")
	feed(t, svc, sess, "\x1b]7501;state=done\a")
	if got, want := store.seen(), []bool{true, false, true}; !slices.Equal(got, want) {
		t.Fatalf("unread writes %v, want %v", got, want)
	}
}

func TestAHookReleasingTheCardShowsTheLiveRecord(t *testing.T) {
	t.Parallel()
	svc, sess := programService(t, hookStore{})
	svc.onHookState(hookRequest{SessionID: "s1", State: statusBusy, Tool: "Bash"})
	feed(t, svc, sess, "\x1b]7501;state=working:app=make\a")
	svc.onHookState(hookRequest{SessionID: "s1", State: statusIdle})
	settleProgram(t, sess)
	if st := svc.Status("s1"); st.State != statusBusy || st.Tool != "make" {
		t.Fatalf("status = %+v, want the program's working", st)
	}
}

func TestAPromptReleasingTheCardShowsAFinishedRecord(t *testing.T) {
	t.Parallel()
	svc, sess := programService(t, hookStore{})
	svc.onHookState(hookRequest{SessionID: "s1", State: statusBusy, Tool: "Bash"})
	feed(t, svc, sess, "\x1b]7501;state=done:app=make\a")
	feed(t, svc, sess, "\x1b]133;A\a")
	if st := svc.Status("s1"); st.State != statusDone {
		t.Fatalf("status = %+v, want the program's done", st)
	}
}
