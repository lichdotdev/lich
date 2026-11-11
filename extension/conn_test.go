package extension

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"
)

// fakeLich is lich's end of a Conn: it reads what the extension writes and
// answers calls with answer, which returns nil to leave one unanswered.
type fakeLich struct {
	toExt  *io.PipeWriter
	writeM sync.Mutex
	calls  chan CallData
	lines  chan Message
}

func newFakeLich(t *testing.T, answer func(CallData) *ResultData) (*fakeLich, *Conn) {
	t.Helper()
	extReads, lichWrites := io.Pipe()
	lichReads, extWrites := io.Pipe()
	f := &fakeLich{toExt: lichWrites, calls: make(chan CallData, 64), lines: make(chan Message, 64)}
	go f.serve(t, lichReads, answer)
	t.Cleanup(func() {
		_ = lichWrites.Close()
		_ = lichReads.Close()
	})
	return f, NewConn(extReads, extWrites)
}

func (f *fakeLich) serve(t *testing.T, r io.Reader, answer func(CallData) *ResultData) {
	in := bufio.NewScanner(r)
	for in.Scan() {
		var msg Message
		if err := json.Unmarshal(in.Bytes(), &msg); err != nil {
			t.Errorf("the extension wrote a line that does not decode: %v: %s", err, in.Bytes())
			return
		}
		call, ok := msg.Data.(CallData)
		if !ok {
			f.lines <- msg
			continue
		}
		f.calls <- call
		if res := answer(call); res != nil {
			f.send(t, CallResult.New(*res))
		}
	}
}

func (f *fakeLich) send(t *testing.T, msg Message) {
	raw, err := json.Marshal(msg)
	if err != nil {
		t.Error(err)
		return
	}
	f.writeM.Lock()
	defer f.writeM.Unlock()
	if _, err := f.toExt.Write(append(raw, '\n')); err != nil {
		t.Errorf("write to the extension: %v", err)
	}
}

func answerSessions(call CallData) *ResultData {
	res := ListSessions.Answer(call.ID, []Session{{Label: "worker-" + call.ID}}).Data.(ResultData)
	return &res
}

func within(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestConnCorrelatesConcurrentCalls(t *testing.T) {
	_, conn := newFakeLich(t, answerSessions)
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sessions, err := conn.Sessions(within(t))
			if err != nil || len(sessions) != 1 {
				t.Errorf("got %v, %v", sessions, err)
			}
		}()
	}
	wg.Wait()
}

func TestConnAnswersOutOfOrder(t *testing.T) {
	held := make(chan CallData, 1)
	f, conn := newFakeLich(t, func(call CallData) *ResultData {
		if call.ID == "1" {
			held <- call
			return nil
		}
		return answerSessions(call)
	})
	first := make(chan []Session, 1)
	go func() {
		sessions, err := conn.Sessions(within(t))
		if err != nil {
			t.Error(err)
		}
		first <- sessions
	}()
	call := <-held
	second, err := conn.Sessions(within(t))
	if err != nil || second[0].Label != "worker-2" {
		t.Fatalf("second call got %v, %v", second, err)
	}
	f.send(t, ListSessions.Answer(call.ID, []Session{{Label: "late"}}))
	if got := <-first; got[0].Label != "late" {
		t.Errorf("first call got %v", got)
	}
}

func TestConnReturnsAnErrorResultAsAnError(t *testing.T) {
	_, conn := newFakeLich(t, func(call CallData) *ResultData {
		return &ResultData{ID: call.ID, Error: "needs sessions:write"}
	})
	_, err := conn.SendToSession(within(t), SendParams{Session: "a", Prompt: "b"})
	if err == nil || err.Error() != "sessions.send: needs sessions:write" {
		t.Errorf("got %v", err)
	}
}

func TestConnCallHonoursItsContext(t *testing.T) {
	f, conn := newFakeLich(t, func(CallData) *ResultData { return nil })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := conn.Projects(ctx)
		done <- err
	}()
	call := <-f.calls
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
	// The late answer is dropped, not handed to the extension as a message.
	f.send(t, ListProjects.Answer(call.ID, []Project{}))
	f.send(t, SurfaceOpened.New(SurfaceData{Surface: "main"}))
	if msg := <-conn.Messages(); msg.Kind != SurfaceOpened.Name {
		t.Errorf("first message is %q, want surface.opened", msg.Kind)
	}
}

// A handler that calls lich while the extension is not reading Messages must
// still get its answer: results never queue behind messages.
func TestConnDeliversMessagesInOrderWithoutBlockingResults(t *testing.T) {
	f, conn := newFakeLich(t, answerSessions)
	for i := range 5 {
		f.send(t, FireEvent.New(EventData{Handler: Handler(fmt.Sprint(i))}))
	}
	f.send(t, SessionStatus.New(SessionStatusData{ID: "s1", State: "done"}))
	first := <-conn.Messages()
	if _, err := conn.Sessions(within(t)); err != nil {
		t.Fatalf("a call made while messages were waiting: %v", err)
	}
	if first.Data.(EventData).Handler != "0" {
		t.Errorf("first message %+v", first)
	}
	for i := 1; i < 5; i++ {
		if got := (<-conn.Messages()).Data.(EventData).Handler; got != Handler(fmt.Sprint(i)) {
			t.Errorf("message %d is handler %q", i, got)
		}
	}
	ev := (<-conn.Messages()).Data.(LichEventData)
	if status, err := SessionStatus.Decode(ev); err != nil || status.State != "done" {
		t.Errorf("status %+v, %v", status, err)
	}
}

func TestConnEndsWhenLichCloses(t *testing.T) {
	f, conn := newFakeLich(t, func(CallData) *ResultData { return nil })
	done := make(chan error, 1)
	go func() {
		_, err := conn.Sessions(context.Background())
		done <- err
	}()
	<-f.calls
	f.send(t, SurfaceClosed.New(SurfaceData{Surface: "main"}))
	_ = f.toExt.Close()
	if err := <-done; !errors.Is(err, ErrClosed) {
		t.Errorf("the call in flight got %v, want ErrClosed", err)
	}
	if msg, ok := <-conn.Messages(); !ok || msg.Kind != SurfaceClosed.Name {
		t.Errorf("the message sent before the close was lost: %+v, %t", msg, ok)
	}
	if _, ok := <-conn.Messages(); ok {
		t.Error("Messages is still open after lich closed")
	}
	if _, err := conn.Sessions(context.Background()); !errors.Is(err, ErrClosed) {
		t.Errorf("a call after the close got %v", err)
	}
}

func TestConnWritesWholeLinesFromManyGoroutines(t *testing.T) {
	f, conn := newFakeLich(t, answerSessions)
	const writers = 50
	var wg sync.WaitGroup
	for i := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := conn.Send(ShowToast.New(ToastData{Title: fmt.Sprint(i)})); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	for range writers {
		if msg := <-f.lines; msg.Kind != ShowToast.Name {
			t.Errorf("got %q", msg.Kind)
		}
	}
}

func TestEachHelperCallsItsMethod(t *testing.T) {
	f, conn := newFakeLich(t, func(call CallData) *ResultData {
		return &ResultData{ID: call.ID, Data: json.RawMessage(`null`)}
	})
	ctx := within(t)
	helpers := map[string]func() error{
		AskSession.Name:    func() error { _, err := conn.Ask(ctx, AskParams{Session: "a", Question: "q"}); return err },
		CollectAnswer.Name: func() error { _, err := conn.Answer(ctx, AnswerParams{Ticket: "t"}); return err },
		ListServices.Name:  func() error { _, err := conn.Services(ctx, "lich"); return err },
		StartService.Name:  func() error { _, err := conn.StartService(ctx, ServiceParams{Service: "web"}); return err },
		StopService.Name:   func() error { _, err := conn.StopService(ctx, ServiceParams{Service: "web"}); return err },
		RestartService.Name: func() error {
			_, err := conn.RestartService(ctx, ServiceParams{Service: "web"})
			return err
		},
		ServiceOutput.Name:  func() error { _, err := conn.ServiceOutput(ctx, ServiceParams{Service: "web"}); return err },
		GitStatus.Name:      func() error { _, err := conn.GitStatus(ctx, "/src"); return err },
		GitPullRequest.Name: func() error { _, err := conn.PullRequest(ctx, "/src"); return err },
		Notify.Name:         func() error { return conn.Notify(ctx, "t", "b") },
		ShowCode.Name:       func() error { _, err := conn.ShowCode(ctx, ShowCodeParams{Session: "s", Path: "a.go"}); return err },
		OpenURL.Name:        func() error { _, err := conn.OpenURL(ctx, "s", "https://example.com"); return err },
	}
	for name, helper := range helpers {
		if err := helper(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		if call := <-f.calls; call.Method != name {
			t.Errorf("the helper for %s called %s", name, call.Method)
		}
	}
}
