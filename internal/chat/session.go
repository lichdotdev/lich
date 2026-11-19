package chat

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/project"
	"github.com/lichdotdev/lich/internal/terminal"
)

// The states a chat reports to the card, the hooks' own words
// (docs/hooks/session-state.md).
const (
	stateBusy    = "busy"
	stateDone    = "done"
	stateWaiting = "waiting"
)

// The stop reasons a turn ends with, ACP's own.
const (
	stopEndTurn   = "end_turn"
	stopCancelled = "cancelled"
	stopError     = "error"
)

var (
	errTurnRunning = errors.New("the agent is still working on the last message")
	errNoTurn      = errors.New("no turn is running")
)

// driver speaks one structured protocol to a provider's process.
type driver interface {
	// open runs the handshake and opens conversation resume, "" for a new
	// one, and returns the id the provider runs it under, "" when the
	// provider names it only once the first message is sent.
	open(ctx context.Context, resume string) (string, error)
	// prompt sends the person's message. The turn ends through endTurn.
	prompt(text string) error
	// follow sends the message the person wrote while the agent waited on
	// questions, which were just declined, in the turn still running.
	follow(text string) error
	// cancel asks the provider to stop the turn in flight.
	cancel() error
	// handle reads one line the provider wrote.
	handle(line []byte)
	// stopped is told the provider's stdout ended.
	stopped()
	// setOption sets option o to value, errRelaunch when only a new process
	// can. What the chat can be set to reaches it through setOptions.
	setOption(ctx context.Context, o api.ChatOption, value string) error
}

// permission is a question the agent asked and nobody has answered yet.
type permission struct {
	options []api.ChatPermitOption
	reply   func(api.ChatPermitOption) error
	// dismiss answers it no, for a turn that ended under it.
	dismiss func()
}

// chatSession is one session's chat while its process runs.
type chatSession struct {
	svc       *Service
	id        string
	kind      string
	projectID string
	cwd       string
	conv      *conversation
	proc      *terminal.ChatProcess
	drv       driver
	out       *wire

	mu      sync.Mutex
	running bool
	since   time.Time
	// before is the checkout as the turn in flight found it, nil where it
	// could not be read.
	before    *project.Lines
	perms     map[string]permission
	questions map[string]question
	exited    chan struct{}
	bound     string
	opts      []api.ChatOption
	// next is a message for the turn after this one, which a provider that
	// takes none during a turn is sent once the turn ends.
	next string
}

// wire writes one JSON value per line to a provider's stdin.
type wire struct {
	mu sync.Mutex
	w  io.Writer
}

func (w *wire) send(v any) error {
	line, err := json.Marshal(v)
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err = w.w.Write(append(line, '\n'))
	return err
}

// read feeds every line the provider writes to the driver until its stdout
// ends. A line has no length limit: Claude Code's system/init and a tool's
// result run past bufio.Scanner's.
func (c *chatSession) read() {
	r := bufio.NewReader(c.proc.Stdout)
	for {
		line, err := r.ReadBytes('\n')
		if line = []byte(strings.TrimSpace(string(line))); len(line) > 0 {
			c.drv.handle(line)
		}
		if err != nil {
			break
		}
	}
	c.drv.stopped()
	<-c.proc.Done()
	c.died()
}

// died closes the chat after its process exited, telling the window why when
// it took a turn with it.
func (c *chatSession) died() {
	c.mu.Lock()
	running := c.running
	c.mu.Unlock()
	if running {
		c.conv.notice(exitNotice(c.proc.Stderr()))
		c.endTurn(stopError)
	}
	close(c.exited)
	c.svc.forget(c)
}

// exitNotice is what a chat says when its process died under a turn: the end
// of its stderr, which is where a provider says why.
func exitNotice(stderr string) string {
	stderr = strings.TrimSpace(stderr)
	if stderr == "" {
		return "The agent's process exited."
	}
	return "The agent's process exited: " + lastLines(stderr, 5)
}

func lastLines(text string, n int) string {
	lines := strings.Split(text, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// beginTurn opens a turn for the message the person just sent.
func (c *chatSession) beginTurn() error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return errTurnRunning
	}
	c.running, c.since, c.before = true, time.Now(), nil
	c.mu.Unlock()
	events.Send(c.svc.hub.Emit, api.ChatTurnEvent, api.ChatTurn{SessionID: c.id, Running: true})
	c.svc.term.ReportChatState(c.id, stateBusy, "", "")
	return nil
}

// markStart reads the checkout the turn begins on. It runs before the message
// goes out, or the agent's first edit could land in the reading.
func (c *chatSession) markStart() {
	before := c.readLines()
	c.mu.Lock()
	c.before = before
	c.mu.Unlock()
}

// endTurn closes the turn in flight, answering no to whatever it still asks.
// A turn ends once: the provider's end and a process's death can both report it.
func (c *chatSession) endTurn(reason string) {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return
	}
	before := c.before
	c.mu.Unlock()
	// Read while the turn still counts as running: a message sent once it
	// does not would open the next turn under this one's end event.
	changes := turnChanges(before, c.readLines())
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return
	}
	c.running = false
	took := time.Since(c.since).Milliseconds()
	c.mu.Unlock()
	c.dismissPermissions()
	c.conv.flush()
	c.svc.term.ReportChatState(c.id, stateDone, "", "")
	events.Send(c.svc.hub.Emit, api.ChatTurnEvent, api.ChatTurn{SessionID: c.id, StopReason: reason, DurationMs: took, Changes: changes})
	c.mu.Lock()
	next := c.next
	c.next = ""
	c.mu.Unlock()
	if next != "" {
		go c.sendNext(next)
	}
}

// send hands text to the agent in the turn just begun.
func (c *chatSession) send(text string) error {
	c.markStart()
	if err := c.drv.prompt(text); err != nil {
		c.conv.notice(err.Error())
		c.endTurn(stopError)
		return err
	}
	return nil
}

// sendNext opens the turn for a message already shown, which waited for the
// last turn to end.
func (c *chatSession) sendNext(text string) {
	if err := c.beginTurn(); err != nil {
		slog.Warn("chat: send the next message", "session", c.id, "err", err)
		return
	}
	if err := c.send(text); err != nil {
		slog.Warn("chat: send the next message", "session", c.id, "err", err)
	}
}

// holdForNext keeps text for the turn after the one running, and reports
// false when none runs.
func (c *chatSession) holdForNext(text string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running {
		c.next = text
	}
	return c.running
}

// dismissPermissions answers no to every question still waiting, which a
// cancel does first: ACP has the agent wait on the answer before it stops.
func (c *chatSession) dismissPermissions() {
	c.declineQuestions()
	c.mu.Lock()
	perms := c.perms
	c.perms = map[string]permission{}
	c.mu.Unlock()
	for id, p := range perms {
		p.dismiss()
		c.conv.update(id, api.ItemPermission, true, func(it *api.ChatItem) {
			if it.Permission != nil {
				it.Permission.Tool.State = api.ToolFailed
			}
		})
	}
}

// turnRunning reports whether a turn is in flight.
func (c *chatSession) turnRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// toolStarted tells the card which tool the agent is running.
func (c *chatSession) toolStarted(name, detail string) {
	if c.turnRunning() {
		c.svc.term.ReportChatState(c.id, stateBusy, name, detail)
	}
}

// ask shows the agent's question as a permission item and waits for the
// person, unless the checkout answers every question yes.
func (c *chatSession) ask(itemID string, tool api.ChatToolCall, p permission) {
	if c.proc.Birth.AutoApprove {
		if opt, ok := allowOption(p.options); ok {
			if err := p.reply(opt); err != nil {
				slog.Warn("chat: auto-approve", "session", c.id, "err", err)
			}
			return
		}
	}
	c.mu.Lock()
	c.perms[itemID] = p
	c.mu.Unlock()
	c.conv.put(api.ChatItem{ID: itemID, Kind: api.ItemPermission, Permission: &api.ChatPermission{Tool: tool, Options: p.options}}, true)
	c.svc.term.ReportChatState(c.id, stateWaiting, tool.Name, tool.Title)
}

// allowOption is the option that lets the tool run this once.
func allowOption(options []api.ChatPermitOption) (api.ChatPermitOption, bool) {
	for _, kind := range []string{api.PermitAllowOnce, api.PermitAllowAlways} {
		for _, o := range options {
			if o.Kind == kind {
				return o, true
			}
		}
	}
	return api.ChatPermitOption{}, false
}

// answer replies to permission item itemID with option optionID.
func (c *chatSession) answer(itemID, optionID string) error {
	c.mu.Lock()
	p, ok := c.perms[itemID]
	if ok {
		delete(c.perms, itemID)
	}
	c.mu.Unlock()
	if !ok {
		return fmt.Errorf("permission %q is not waiting for an answer", itemID)
	}
	var chosen api.ChatPermitOption
	for _, o := range p.options {
		if o.ID == optionID {
			chosen = o
		}
	}
	if chosen.ID == "" {
		c.mu.Lock()
		c.perms[itemID] = p
		c.mu.Unlock()
		return fmt.Errorf("permission %q has no option %q", itemID, optionID)
	}
	if err := p.reply(chosen); err != nil {
		return err
	}
	c.conv.update(itemID, api.ItemPermission, true, func(it *api.ChatItem) { it.Permission.Chosen = chosen.ID })
	c.svc.term.ReportChatState(c.id, stateBusy, "", "")
	return nil
}

// bind records the conversation the provider runs, which the terminal resumes
// after a switch back. expected is the one lich asked to reopen: a provider
// that opened another one instead lost the conversation, and says so here.
func (c *chatSession) bind(providerID, expected string) {
	if providerID == "" {
		return
	}
	c.mu.Lock()
	same := c.bound == providerID
	c.bound = providerID
	c.mu.Unlock()
	if same {
		return
	}
	if expected != "" && providerID != expected {
		c.conv.notice(fmt.Sprintf("The conversation did not come back: the agent opened %s instead of %s.", providerID, expected))
	}
	if err := c.svc.term.BindConversation(c.id, providerID); err != nil {
		slog.Warn("chat: record conversation", "session", c.id, "err", err)
	}
}

// stop ends the process and waits for the chat to close.
func (c *chatSession) stop() {
	c.proc.Close()
	<-c.exited
}
