package main

import (
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// issueLookupDelay is how long the name has to settle before the issue
// behind a reference is looked up: every keystroke of "#381" is a valid
// reference on its way to the one meant.
const issueLookupDelay = 400 * time.Millisecond

// The wait for the prompt a handoff is written at: a worktree's first boot can
// be minutes, and a session that never comes up must stop being waited for.
const (
	promptPoll  = 250 * time.Millisecond
	promptLimit = 5 * time.Minute
)

// worktreeDialog asks for a name (blank for a random one, words for a slug,
// #N for an issue), the base it starts from, whether the checkout's
// uncommitted changes come along, and whether the session runs confined.
// Fields under mu are written by the reads it starts.
type worktreeDialog struct {
	client     *backend.Client
	root       *ui.Root
	invalidate func()

	open, focus bool
	dialog      ui.DialogState
	name        widget.Editor
	search      widget.Editor
	picker      ui.DropdownPanelState
	pickerOpen  bool
	list        widget.List
	rows        map[string]*widget.Clickable
	carry       widget.Bool
	confined    widget.Bool
	cancel      widget.Clickable
	create      widget.Clickable

	project  backend.Project
	kind     string
	source   string            // the checkout the carry copies from
	checkout backend.DiffStats // that checkout's git status
	// fork is the session a fork-to-worktree branches, nil for the + menu's
	// new worktree.
	fork *forkSource

	mu         sync.Mutex
	branches   *backend.Branches
	loadErr    string
	base       string
	checks     map[string]*baseCheck
	issueRef   int
	issue      *backend.Issue
	issueErr   string
	canConfine bool
	// rungConfined is the box's starting value, applied by the frame once
	// rungRead is set: the widget is the frame's alone.
	rungConfined, rungRead bool
	submitting             bool
	submitErr              string
	done                   bool
}

func newWorktreeDialog(c *backend.Client, root *ui.Root, invalidate func()) *worktreeDialog {
	return &worktreeDialog{client: c, root: root, invalidate: invalidate}
}

// show opens the dialog for a session of kind, based on source (a checkout
// of p) and fork (nil for a new session), and starts the reads it draws:
// branches, the sandbox rung, the base's freshness. A fork opens with its
// checkout's uncommitted work coming along, the moment a fork is usually made.
func (d *worktreeDialog) show(ctx context.Context, p backend.Project, kind, source string, checkout backend.DiffStats, fork *forkSource) {
	d.open, d.focus, d.pickerOpen = true, true, false
	d.picker.Close()
	d.project, d.kind, d.source, d.checkout, d.fork = p, kind, source, checkout, fork
	d.name.SetText("")
	d.search.SetText("")
	d.carry.Value = fork != nil
	d.mu.Lock()
	d.branches, d.loadErr, d.base, d.checks = nil, "", "", map[string]*baseCheck{}
	d.issueRef, d.issue, d.issueErr = 0, nil, ""
	d.canConfine, d.submitting, d.submitErr = false, false, ""
	d.rungConfined, d.rungRead, d.done = false, false, false
	d.mu.Unlock()
	go d.loadBranches(ctx)
	go d.loadSandbox(ctx)
}

func (d *worktreeDialog) loadBranches(ctx context.Context) {
	b, err := d.client.ListBranches(ctx, d.project.Path)
	d.mu.Lock()
	if err != nil {
		d.loadErr = backend.ErrorText(err)
	} else {
		// Picking a worktree opens its session, which forks nothing.
		if d.fork != nil {
			b.Worktrees = nil
		}
		d.branches = &b
		if d.base == "" {
			d.base = preferredBase(b, d.checkout.Branch)
		}
	}
	base := d.base
	d.mu.Unlock()
	d.checkBase(ctx, base)
	d.invalidate()
}

// preferredBase is the row the dialog opens on: the checkout's own branch,
// else the first local one.
func preferredBase(b backend.Branches, current string) string {
	for _, name := range b.Local {
		if name == current {
			return baseValue(baseLocal, name)
		}
	}
	if len(b.Local) > 0 {
		return baseValue(baseLocal, b.Local[0])
	}
	return ""
}

// loadSandbox reads whether this machine can confine and the rung the
// session's provider is on, which the box starts on (store.SandboxDefault: a
// worktree is confined from "worktrees" up, and "ask" opens ticked).
func (d *worktreeDialog) loadSandbox(ctx context.Context) {
	backend, err := d.client.SandboxBackend(ctx)
	if err != nil {
		log.Printf("worktree dialog: sandbox backend: %v", err)
		return
	}
	level := ""
	for _, scope := range []string{d.project.ID, ""} {
		v, err := d.client.GetSetting(ctx, sandboxKey(d.kind), scope)
		if err != nil {
			log.Printf("worktree dialog: sandbox rung: %v", err)
			return
		}
		if v != "" {
			level = v
			break
		}
	}
	d.mu.Lock()
	d.canConfine = backend != ""
	d.rungConfined, d.rungRead = sandboxLevel(level) != rungOff, true
	d.mu.Unlock()
	d.invalidate()
}

// checkBase starts the freshness read of base when it is a local branch not
// read yet this opening.
func (d *worktreeDialog) checkBase(ctx context.Context, base string) {
	group, branch := splitBase(base)
	if group != baseLocal {
		return
	}
	d.mu.Lock()
	if d.checks[branch] != nil {
		d.mu.Unlock()
		return
	}
	check := &baseCheck{done: make(chan struct{})}
	d.checks[branch] = check
	d.mu.Unlock()
	go func() {
		f, err := d.client.BaseFreshness(ctx, d.project.Path, branch)
		d.mu.Lock()
		check.freshness = f
		if err != nil {
			check.err = backend.ErrorText(err)
		}
		d.mu.Unlock()
		close(check.done)
		d.invalidate()
	}()
}

// pick makes value the base.
func (d *worktreeDialog) pick(ctx context.Context, value string) {
	d.mu.Lock()
	d.base = value
	d.mu.Unlock()
	d.checkBase(ctx, value)
}

// lookUpIssue follows the reference the name holds: a new one is looked up
// once the name has settled, and one that changed under the lookup is
// dropped.
func (d *worktreeDialog) lookUpIssue(ctx context.Context, typed string) {
	ref := parseIssueRef(typed)
	d.mu.Lock()
	if ref == d.issueRef {
		d.mu.Unlock()
		return
	}
	d.issueRef, d.issue, d.issueErr = ref, nil, ""
	d.mu.Unlock()
	if ref == 0 {
		return
	}
	go func() {
		time.Sleep(issueLookupDelay)
		d.mu.Lock()
		current := d.issueRef == ref
		d.mu.Unlock()
		if !current {
			return
		}
		issue, err := d.client.Issue(ctx, d.project.Path, ref)
		d.mu.Lock()
		if d.issueRef == ref {
			if err != nil {
				d.issueErr = backend.ErrorText(err)
			} else {
				d.issue = &issue
			}
		}
		d.mu.Unlock()
		d.invalidate()
	}()
}

// worktreeView is one frame's read of the dialog's state.
type worktreeView struct {
	branches   *backend.Branches
	loadErr    string
	base       string
	check      *baseCheck
	issueRef   int
	issue      *backend.Issue
	issueErr   string
	canConfine bool
	submitting bool
	submitErr  string
	done       bool
}

func (d *worktreeDialog) view() worktreeView {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, branch := splitBase(d.base)
	return worktreeView{
		branches: d.branches, loadErr: d.loadErr, base: d.base, check: d.checks[branch],
		issueRef: d.issueRef, issue: d.issue, issueErr: d.issueErr,
		canConfine: d.canConfine, submitting: d.submitting, submitErr: d.submitErr, done: d.done,
	}
}

// applyRung sets the confinement box to the rung's answer the first frame
// after it was read.
func (d *worktreeDialog) applyRung() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.rungRead {
		d.confined.Value, d.rungRead = d.rungConfined, false
	}
}

// newBranch is the branch the typed name creates: the issue's title once
// GitHub answered, its bare number until then, else the name as typed.
func (v worktreeView) newBranch(typed string) string {
	switch {
	case v.issue != nil:
		return toBranchName(issueName(*v.issue))
	case v.issueRef != 0:
		return strconv.Itoa(v.issueRef)
	}
	return toBranchName(typed)
}

// canCarry is whether the checkout's uncommitted changes can come along: it
// has some, and the base is the branch they were made on.
func (d *worktreeDialog) canCarry(base string) bool {
	group, branch := splitBase(base)
	return d.checkout.Files > 0 && group == baseLocal && branch == d.checkout.Branch
}

// worktreeRequest is what Create hands the window, read off the frame.
type worktreeRequest struct {
	project backend.Project
	kind    string
	name    string
	base    string
	check   *baseCheck
	carry   bool
	source  string
	sandbox string
	brief   string
	fork    *forkSource
}

// request reads the form into a worktreeRequest.
func (d *worktreeDialog) request(v worktreeView) worktreeRequest {
	r := worktreeRequest{
		project: d.project, kind: d.kind, name: v.newBranch(d.name.Text()), base: v.base,
		check: v.check, carry: d.carry.Value && d.canCarry(v.base), source: d.source, fork: d.fork,
	}
	if v.canConfine {
		r.sandbox = map[bool]string{true: "on", false: "off"}[d.confined.Value]
	}
	if v.issue != nil {
		r.brief = issueBrief(*v.issue)
	}
	return r
}

func (d *worktreeDialog) setSubmitting(err string) {
	d.mu.Lock()
	d.submitting, d.submitErr = err == "", err
	d.mu.Unlock()
	d.invalidate()
}

// openWorktree carries out a request off the frame: reopens a worktree the
// base names, or creates one from where the base says and opens its session.
// A failure stays in the dialog, so git's words can be read in place.
func (u *window) openWorktree(r worktreeRequest) {
	d := u.worktree
	if group, path := splitBase(r.base); group == baseWorktree {
		u.reopenWorktree(r, path)
		return
	}
	if r.fork != nil {
		if err := u.conversationLeft(r.project, *r.fork); err != nil {
			d.setSubmitting(err.Error())
			return
		}
	}
	if r.check != nil {
		<-r.check.done
	}
	start, remote := startPoint(r.base, r.check, r.carry)
	wt, err := u.client.CreateWorktree(u.ctx, r.project.Path, r.project.ID, r.name, start, remote)
	if err != nil {
		d.setSubmitting(backend.ErrorText(err))
		return
	}
	if r.carry {
		u.carryInto(r.source, wt)
	}
	id, err := u.worktreeSession(r, wt)
	if err != nil {
		d.setSubmitting(backend.ErrorText(err))
		return
	}
	if r.brief != "" {
		go u.submitAtPrompt(id, r.brief)
	}
	d.closeAfter()
}

// closeAfter marks the request carried out; the frame closes the dialog.
func (d *worktreeDialog) closeAfter() {
	d.mu.Lock()
	d.submitting, d.done = false, true
	d.mu.Unlock()
	d.invalidate()
}

// carryInto copies the checkout's uncommitted work into the new worktree,
// and says so when it cannot: the worktree is worth opening either way.
func (u *window) carryInto(from string, wt *backend.Worktree) {
	if wt.Reused {
		u.toaster.Show(ui.Toast{Variant: ui.ToastWarning, Title: i18n.T("sidebar.worktreeDialog.carryReused", i18n.Args{"name": wt.Name})})
		return
	}
	if err := u.client.CarryUncommitted(u.ctx, from, wt.Path); err != nil {
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("sidebar.worktreeDialog.carryFailed", i18n.Args{"error": backend.ErrorText(err)})})
	}
}

// worktreeSession is the new worktree's session: a fork of the dialog's
// source, or a fresh one.
func (u *window) worktreeSession(r worktreeRequest, wt *backend.Worktree) (string, error) {
	if r.fork != nil {
		return u.addForkSession(r.project, *r.fork, wt.Path)
	}
	return u.addWorktreeSession(r, wt, r.sandbox)
}

// addWorktreeSession records the worktree's session, named after it, and
// puts it on screen.
func (u *window) addWorktreeSession(r worktreeRequest, wt *backend.Worktree, sandbox string) (string, error) {
	id, err := u.recordWorktreeSession(r, wt, sandbox)
	if err != nil {
		return "", err
	}
	return id, u.showNewSession(r.project.ID, id)
}

// recordWorktreeSession records the worktree's session, named after it,
// without reloading the projects.
func (u *window) recordWorktreeSession(r worktreeRequest, wt *backend.Worktree, sandbox string) (string, error) {
	id := strings.ToLower(rand.Text())
	return id, u.client.AddSessionWithSandbox(u.ctx, r.project.ID, id, wt.Name, r.kind, wt.Path, r.project.NextSeq+1, sandbox)
}

// reopenWorktree brings back the session parked on an existing worktree, or
// opens a fresh one there.
func (u *window) reopenWorktree(r worktreeRequest, path string) {
	d := u.worktree
	restored, err := u.client.ReopenWorktreeSession(u.ctx, r.project.ID, path, strings.ToLower(rand.Text()))
	if err != nil {
		d.setSubmitting(backend.ErrorText(err))
		return
	}
	if restored == nil {
		name := path[strings.LastIndexAny(path, `/\`)+1:]
		if _, err := u.addWorktreeSession(r, &backend.Worktree{Name: name, Path: path}, ""); err != nil {
			d.setSubmitting(backend.ErrorText(err))
			return
		}
		d.closeAfter()
		return
	}
	if err := u.showNewSession(r.project.ID, restored.ID); err != nil {
		d.setSubmitting(backend.ErrorText(err))
		return
	}
	d.closeAfter()
}

// showNewSession reloads the project and puts session id on screen.
func (u *window) showNewSession(projectID, id string) error {
	if err := u.loadSessions(); err != nil {
		return err
	}
	u.model.setActive(u.ctx, id)
	u.logErr("set active session", u.client.SetActiveSession(u.ctx, projectID, id))
	u.model.invalidate()
	return nil
}

// loadSessions reloads the projects, so a session just recorded has its card.
func (u *window) loadSessions() error {
	if err := u.model.reload(u.ctx); err != nil {
		return err
	}
	u.logErr("load folds", u.model.loadFolds(u.ctx))
	return nil
}

// submitAtPrompt sends message as the session's prompt once it sits at one.
func (u *window) submitAtPrompt(id, message string) {
	if err := u.handOff(id, message); err != nil {
		u.toaster.Show(handoffLostToast(err))
	}
}

func handoffLostToast(err error) ui.Toast {
	return ui.Toast{Variant: ui.ToastError, Title: i18n.T("sidebar.worktreeDialog.handoffLost", i18n.Args{"error": backend.ErrorText(err)})}
}

// handOff is submitAtPrompt without the toast: why message never landed, nil
// once it did.
func (u *window) handOff(id, message string) error {
	if !u.awaitPrompt(id) {
		return errors.New(i18n.T("sidebar.worktreeDialog.neverPrompted"))
	}
	return u.client.Submit(u.ctx, id, message)
}

// awaitPrompt waits for the session to sit at a prompt, false when it never
// does: text written before lands in the boot and is thrown away.
func (u *window) awaitPrompt(id string) bool {
	deadline := time.Now().Add(promptLimit)
	for {
		ready, err := u.client.Ready(u.ctx, id)
		if err == nil && ready {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(promptPoll)
	}
}

// showWorktreeDialog opens the dialog on the active project, for a session of
// the kind a new one would spawn.
func (u *window) showWorktreeDialog() {
	p, ok := u.model.project()
	if !ok {
		return
	}
	u.settings.hide()
	checkout, _ := u.model.gitStatus(p.Path)
	u.worktree.show(u.ctx, p, u.model.newSessionKind(), p.Path, checkout, nil)
	// The git store reads the checkouts cards show; the project's own may
	// have none on screen, and the carry row needs where it stands now.
	go u.model.refreshPath(u.ctx, p.Path)
}

var flagWorktreeDialog = flag.String("worktree-dialog", "", "open the new-worktree dialog at start with this typed as its name (\" \" for blank), for -shot")

// worktreeShot opens the dialog at start as -worktree-dialog asks.
func (u *window) worktreeShot() {
	if *flagWorktreeDialog == "" {
		return
	}
	u.showWorktreeDialog()
	u.worktree.name.SetText(strings.TrimSpace(*flagWorktreeDialog))
}
