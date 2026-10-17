package main

import (
	"context"
	"crypto/rand"
	"flag"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"gioui.org/widget"

	"github.com/omartelo/lich/native/i18n"
	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// issueLookupDelay is how long the name has to settle before the issue
// behind a reference is looked up: every keystroke of "#381" is a valid
// reference on its way to the one meant.
const issueLookupDelay = 400 * time.Millisecond

// The wait for the prompt an issue is pasted at (write-at-prompt.ts): a
// worktree's first boot can be minutes, and a session that never comes up must
// stop being waited for.
const (
	promptPoll  = 250 * time.Millisecond
	promptLimit = 5 * time.Minute
)

// worktreeDialog is WorktreeDialog: a name (blank for a random one, words for
// a slug, #N for an issue), the base it starts from, whether the checkout's
// uncommitted changes come along, and whether the session runs confined.
// Fields under mu are written by the reads it starts.
type worktreeDialog struct {
	client     *lichclient.Client
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

	project  lichclient.Project
	kind     string
	checkout lichclient.DiffStats // the project's own checkout, for the carry

	mu         sync.Mutex
	branches   *lichclient.Branches
	loadErr    string
	base       string
	checks     map[string]*baseCheck
	issueRef   int
	issue      *lichclient.Issue
	issueErr   string
	canConfine bool
	// rungConfined is the box's starting value, applied by the frame once
	// rungRead is set: the widget is the frame's alone.
	rungConfined, rungRead bool
	submitting             bool
	submitErr              string
	done                   bool
}

func newWorktreeDialog(c *lichclient.Client, root *ui.Root, invalidate func()) *worktreeDialog {
	return &worktreeDialog{client: c, root: root, invalidate: invalidate}
}

// show opens the dialog on p's checkout, for a session of kind, and starts
// the reads it draws: branches, the sandbox rung, the base's freshness.
func (d *worktreeDialog) show(ctx context.Context, p lichclient.Project, kind string, checkout lichclient.DiffStats) {
	d.open, d.focus, d.pickerOpen = true, true, false
	d.picker.Close()
	d.project, d.kind, d.checkout = p, kind, checkout
	d.name.SetText("")
	d.search.SetText("")
	d.carry.Value = false
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
		d.loadErr = lichclient.ErrorText(err)
	} else {
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
func preferredBase(b lichclient.Branches, current string) string {
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
	var backend string
	if err := d.client.Call(ctx, "system.SandboxBackend", &backend); err != nil {
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
			check.err = lichclient.ErrorText(err)
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
				d.issueErr = lichclient.ErrorText(err)
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
	branches   *lichclient.Branches
	loadErr    string
	base       string
	check      *baseCheck
	issueRef   int
	issue      *lichclient.Issue
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
	project lichclient.Project
	kind    string
	name    string
	base    string
	check   *baseCheck
	carry   bool
	sandbox string
	brief   string
}

// request reads the form into a worktreeRequest.
func (d *worktreeDialog) request(v worktreeView) worktreeRequest {
	r := worktreeRequest{
		project: d.project, kind: d.kind, name: v.newBranch(d.name.Text()), base: v.base,
		check: v.check, carry: d.carry.Value && d.canCarry(v.base),
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
	if r.check != nil {
		<-r.check.done
	}
	start, remote := startPoint(r.base, r.check, r.carry)
	wt, err := u.client.CreateWorktree(u.ctx, r.project.Path, r.project.ID, r.name, start, remote)
	if err != nil {
		d.setSubmitting(lichclient.ErrorText(err))
		return
	}
	if r.carry {
		u.carryInto(r.project.Path, wt)
	}
	id, err := u.addWorktreeSession(r, wt, r.sandbox)
	if err != nil {
		d.setSubmitting(lichclient.ErrorText(err))
		return
	}
	if r.brief != "" {
		go u.writeAtPrompt(id, r.brief)
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
func (u *window) carryInto(from string, wt *lichclient.Worktree) {
	if wt.Reused {
		u.toaster.Show(ui.Toast{Variant: ui.ToastWarning, Title: i18n.T("sidebar.worktreeDialog.carryReused", i18n.Args{"name": wt.Name})})
		return
	}
	if err := u.client.CarryUncommitted(u.ctx, from, wt.Path); err != nil {
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("sidebar.worktreeDialog.carryFailed", i18n.Args{"error": lichclient.ErrorText(err)})})
	}
}

// addWorktreeSession records the worktree's session, named after it, and
// puts it on screen.
func (u *window) addWorktreeSession(r worktreeRequest, wt *lichclient.Worktree, sandbox string) (string, error) {
	id := strings.ToLower(rand.Text())
	if err := u.client.AddSessionWithSandbox(u.ctx, r.project.ID, id, wt.Name, r.kind, wt.Path, r.project.NextSeq+1, sandbox); err != nil {
		return "", err
	}
	return id, u.showNewSession(r.project.ID, id)
}

// reopenWorktree brings back the session parked on an existing worktree, or
// opens a fresh one there.
func (u *window) reopenWorktree(r worktreeRequest, path string) {
	d := u.worktree
	restored, err := u.client.ReopenWorktreeSession(u.ctx, r.project.ID, path, strings.ToLower(rand.Text()))
	if err != nil {
		d.setSubmitting(lichclient.ErrorText(err))
		return
	}
	if restored == nil {
		name := path[strings.LastIndexAny(path, `/\`)+1:]
		if _, err := u.addWorktreeSession(r, &lichclient.Worktree{Name: name, Path: path}, ""); err != nil {
			d.setSubmitting(lichclient.ErrorText(err))
			return
		}
		d.closeAfter()
		return
	}
	if err := u.showNewSession(r.project.ID, restored.ID); err != nil {
		d.setSubmitting(lichclient.ErrorText(err))
		return
	}
	d.closeAfter()
}

// showNewSession reloads the project and puts session id on screen.
func (u *window) showNewSession(projectID, id string) error {
	if err := u.model.reload(u.ctx); err != nil {
		return err
	}
	u.logErr("load folds", u.model.loadFolds(u.ctx))
	u.model.setActive(u.ctx, id)
	u.logErr("set active session", u.client.SetActiveSession(u.ctx, projectID, id))
	u.model.invalidate()
	return nil
}

// writeAtPrompt is write-at-prompt.ts: text written before the session sits
// at a prompt lands in its boot and is thrown away.
func (u *window) writeAtPrompt(id, text string) {
	deadline := time.Now().Add(promptLimit)
	for {
		ready, err := u.client.Ready(u.ctx, id)
		if err == nil && ready {
			break
		}
		if time.Now().After(deadline) {
			u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("sidebar.worktreeDialog.issueLost", i18n.Args{"error": i18n.T("sidebar.worktreeDialog.neverPrompted")})})
			return
		}
		time.Sleep(promptPoll)
	}
	if err := u.client.Write(u.ctx, id, text); err != nil {
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("sidebar.worktreeDialog.issueLost", i18n.Args{"error": lichclient.ErrorText(err)})})
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
	u.worktree.show(u.ctx, p, u.model.newSessionKind(), checkout)
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
