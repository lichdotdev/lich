// Package spawn opens new lich sessions for a caller that is not the window:
// the `lich` command line and the MCP tools beside it (internal/cli). An agent
// could already talk to the sessions around it (internal/relay); this is what
// lets it create one — with a git worktree under it when the work needs its own
// checkout.
//
// It exists as its own service because opening a session is four things at once
// and no single package owns them: a worktree (internal/project), a row
// (internal/store), a PTY (internal/terminal) and a card in the window
// (internal/events). The window does the same four in its own order
// (native/cmd/lich-native/main.go, newSession).
//
// The PTY is started here rather than left to the window, and that is the whole
// point of the feature. The window spawns a session's terminal only once
// somebody has looked at the card, so a session opened for an agent and never
// clicked would have no process in it — invisible to `lich sessions`,
// unreachable by `lich send`, useless to whoever asked for it. Starting it here
// makes it addressable the moment Open returns; the card's first view attaches
// to the PTY that is already running, exactly as it does when the window opens
// again on a backend that kept it.
package spawn

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/project"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/relay"
	"github.com/lichdotdev/lich/internal/store"
	"github.com/lichdotdev/lich/internal/terminal"

	"github.com/lichdotdev/lich/internal/events"
)

type ClosedEvent = api.SessionClosed

// The size a session opened here starts at: none of its own, which the terminal
// service reads as "whatever size the window last reported" (terminal.sizeFor).
//
// There is no terminal to measure at this end — that is what opening a session
// for an agent means — and naming a shape here was wrong rather than arbitrary.
// The provider drew its whole conversation into a grid the window did not have;
// the first view of the card resized the PTY, the TUI repainted, and everything
// it had written was gone from the screen. Copying the window's size costs
// nothing and leaves the card readable.
const (
	startCols = 0
	startRows = 0
)

// Sessions is the persistence this needs: the workspace to resolve a project
// in, and the writes that add a session to one, take one away, and park one for
// a later resume. The store implements it.
type Sessions interface {
	LoadState(_ context.Context, _ api.None) ([]store.Project, error)
	RecentProjects(_ context.Context, opts api.SearchTermOptions) ([]store.Recent, error)
	AddProject(_ context.Context, opts api.AddProjectOptions) (api.None, error)
	AddSessionFrom(_ context.Context, opts api.AddSessionFromOptions) (api.None, error)
	SandboxDefault(providerID, projectID, cwd string) bool
	SetSessionModel(sessionID, model string) error
	SetSessionEffort(sessionID, effort string) error
	SetSessionUltracode(sessionID string) error
	SetSessionSubagentDepth(sessionID string, depth int) error
	SessionSubagentDepth(sessionID string) int
	RenameSession(_ context.Context, opts api.LabelSessionOptions) (api.None, error)
	SetSessionFolder(_ context.Context, opts api.SessionFolderOptions) (api.None, error)
	RenameFolder(_ context.Context, opts api.RenameProjectFolderOptions) ([]string, error)
	ColorFolder(projectID, folder, color string) ([]string, error)
	DeleteSession(projectID, sessionID, activeID string) error
	CloseSession(_ context.Context, opts api.ParkSessionOptions) (api.None, error)
	PurgeWorktreeSessions(_ context.Context, opts api.WorktreeSessionsOptions) (api.None, error)
}

// Worktrees is the git side. The project service implements it.
type Worktrees interface {
	Branch(path string) string
	ListBranches(_ context.Context, opts api.PathOptions) (project.Branches, error)
	ListCheckouts(_ context.Context, opts api.PathOptions) ([]project.Worktree, error)
	CreateWorktree(_ context.Context, opts api.CreateWorktreeOptions) (*project.Worktree, error)
	BaseFreshness(_ context.Context, opts api.PathBranchOptions) (project.BaseFreshness, error)
	RemoveWorktree(_ context.Context, opts api.RemoveWorktreeOptions) (api.None, error)
	WorktreeDirty(_ context.Context, opts api.PathOptions) (bool, error)
	WorktreeAdopted(_ context.Context, opts api.PathOptions) (bool, error)
}

// Terminal is the PTY side: the same spawn the window asks for when a card is
// first viewed, and the same close it asks for when a card goes away. The
// terminal service implements it.
type Terminal interface {
	Start(_ context.Context, opts api.StartOptions) (api.None, error)
	// CloseBy closes the session on behalf of closerID, the session that asked
	// for it, or empty when none did.
	CloseBy(id, closerID string) error
	// AgentName is the name that session's agent answers to in its provider's
	// peer roster, read out of the provider's own record. Empty when there is
	// none, which is what leaves `lich close` matching the derived name.
	AgentName(id string) string
	// RunModCommand queues cmd for the session's Claude Code mod and waits,
	// until ctx ends, for the mod to confirm it.
	RunModCommand(ctx context.Context, id string, cmd terminal.ModCommand) (terminal.ModOutcome, error)
}

// Events is where an opened session is announced to the window. A nil one leaves
// the feature working and silent — the session runs, its card appears on the
// next reload.
type Events interface {
	Emit(name string, data any)
}

type Session = api.OpenedSession

// Service opens sessions on behalf of a caller outside the window.
type Service struct {
	sessions  Sessions
	worktrees Worktrees
	term      Terminal
	events    Events
	// open serializes Open. Every RPC call arrives on its own goroutine, and
	// Open reads the project's label counter and writes it back — two of them at
	// once read the same number and hand both sessions the same label, which is
	// the one thing `lich send` cannot resolve.
	open sync.Mutex
}

func New(sessions Sessions, worktrees Worktrees, term Terminal, events Events) *Service {
	return &Service{sessions: sessions, worktrees: worktrees, term: term, events: events}
}

type OpenOptions = api.OpenSessionOptions

// Open creates a session and starts its PTY.
func (s *Service) Open(_ context.Context, opts OpenOptions) (Session, error) {
	return s.openSession(request{
		fromID: opts.From, projectName: opts.Project, kind: opts.Kind, worktree: opts.Worktree,
		base: opts.Base, model: opts.Model, effort: opts.Effort, folder: opts.Folder,
		ultracode: opts.Ultracode,
	})
}

type OpenSubagentOptions = api.OpenSubagentOptions

// OpenSubagent opens a session as the subagent of the session From runs in
// (`lich open --subagent`), the way lich-plugin's mod turns Claude Code's own
// subagent call into a card. It is Open with three differences, each of which
// makes the card behave like the subagent it replaces:
//
//   - without a worktree it opens in the caller's checkout — its worktree, or
//     the project directory it runs in — rather than the project's own
//     directory, so the worker edits what the caller sees;
//   - it is filed with the caller, so workers sit under whoever asked for them:
//     in the caller's folder, or one named after the caller's label that the
//     caller is filed under too (subagentFolder);
//   - its row records how deep it runs, one level below the caller
//     (SetSessionSubagentDepth), so every spawn of it knows whether its own
//     subagents may still become cards (internal/terminal, subagentCardsOn).
//
// The project is always the caller's. A caller that is not a session lich holds
// is refused before anything is opened: there is no checkout to share and no
// one to report to.
func (s *Service) OpenSubagent(_ context.Context, opts OpenSubagentOptions) (Session, error) {
	return s.openSession(request{
		fromID: opts.From, kind: opts.Kind, worktree: opts.Worktree, base: opts.Base,
		model: opts.Model, effort: opts.Effort, ultracode: opts.Ultracode, subagent: true,
	})
}

// request is one Open or OpenSubagent call. subagent selects OpenSubagent's
// placement, folder and row depth.
type request struct {
	fromID, projectName, kind, worktree, base, model, effort, folder string
	ultracode, subagent                                              bool
}

func (s *Service) openSession(req request) (Session, error) {
	if req.base != "" && req.worktree == "" {
		return Session{}, fmt.Errorf(
			"a base is the branch a new worktree starts from, and no worktree was asked "+
				"for, so %q would be dropped in silence — name the worktree to create, or "+
				"leave the base out",
			req.base,
		)
	}

	// Held across the spawn as well as the write: the checkout and the PTY are
	// cheap beside a duplicate label, and one lock is one thing to reason about.
	s.open.Lock()
	defer s.open.Unlock()

	projects, err := s.sessions.LoadState(context.Background(), api.None{})
	if err != nil {
		return Session{}, fmt.Errorf("read the workspace: %w", err)
	}
	originID, originLabel := originOf(projects, req.fromID)
	if req.subagent && originID == "" {
		return Session{}, errors.New(
			"a subagent is opened for the session asking for it, and this is not running " +
				"in a session lich holds — open it without --subagent, or from inside a session",
		)
	}
	projects, target, err := s.ensureProject(projects, req.fromID, req.projectName)
	if err != nil {
		return Session{}, err
	}
	kind, err := resolveKind(projects, req.fromID, req.kind)
	if err != nil {
		return Session{}, err
	}
	model, effort, folder := strings.TrimSpace(req.model), strings.TrimSpace(req.effort), strings.TrimSpace(req.folder)
	if err := checkOverrides(kind, model, effort, req.ultracode); err != nil {
		return Session{}, err
	}

	at, err := s.place(target, req.worktree, req.base)
	if err != nil {
		return Session{}, err
	}
	callerToFile := ""
	if req.subagent {
		folder, callerToFile = subagentFolder(target, req.fromID, originLabel)
		if req.worktree == "" {
			at = besideCaller(target, req.fromID, at)
		}
	}
	cwd, stored, label := at.cwd, at.stored, at.label

	id, err := newSessionID()
	if err != nil {
		return Session{}, err
	}
	opened := Session{
		ID:              id,
		ProjectID:       target.ID,
		Project:         target.Name,
		Label:           label,
		Name:            relay.RosterNameOf(kind, "", cwd, id),
		Kind:            kind,
		Path:            stored,
		NextSeq:         target.NextSeq + 1,
		OriginSessionID: originID,
		OriginLabel:     originLabel,
		Folder:          folder,
		Confined:        s.sessions.SandboxDefault(kind, target.ID, cwd),
	}
	if _, err := s.sessions.AddSessionFrom(context.Background(), api.AddSessionFromOptions{ProjectID: target.ID, SessionID: id, Label: label, Kind: kind, Path: stored, NextSeq: opened.NextSeq, OriginID: originID, OriginLabel: originLabel, Folder: folder}); err != nil {
		return Session{}, err
	}
	// Before the card is announced, so the reload it triggers reads the model
	// the session was born on (the card's tooltip compares it with the running
	// one). A write that fails must not be what hides the session, nor cost it
	// its terminal: a card with no PTY behind it is invisible to `lich sessions`
	// and unreachable by `lich send`, which is the state this package exists to
	// prevent. So the error is only held: the spawn below reads the model back
	// from the row, the session starts on the provider's own default and the
	// failure is reported once it is running.
	overrideErr := s.recordOverrides(id, label, model, effort, req.ultracode)
	// Announced before the spawn, and regardless of how it goes: the row exists
	// either way, so the card has to exist either way too — a session only the
	// database knows about is one the user cannot reach to see what went wrong.
	if s.events != nil {
		events.Send(s.events.Emit, api.SessionOpenedEvent, opened)
	}
	callerErr := s.fileCaller(target.ID, callerToFile, folder)
	markErr := s.markSubagent(id, label, req)
	if _, err := s.term.Start(context.Background(), api.StartOptions{ID: id, ProjectID: target.ID, Cwd: cwd, Kind: kind, Cols: startCols, Rows: startRows}); err != nil {
		return Session{}, fmt.Errorf("session %q was created but its terminal did not start: %w", label, err)
	}
	if err := errors.Join(callerErr, overrideErr, markErr); err != nil {
		return Session{}, err
	}
	return opened, nil
}

// besideCaller is the caller's own checkout as a placement: where it runs,
// under the project's next label (the checkout's name is already the caller's).
// A caller missing from target falls back to at, which originOf has already ruled out.
func besideCaller(target store.Project, fromID string, at placement) placement {
	for _, sess := range target.Sessions {
		if sess.ID != fromID {
			continue
		}
		cwd := sess.Path
		if cwd == "" {
			cwd = target.Path
		}
		return placement{cwd: cwd, stored: sess.Path, label: fmt.Sprintf("Session %d", target.NextSeq)}
	}
	return at
}

// subagentFolder is the folder a subagent is filed under, and the caller's id
// when the caller has to be filed there too. A caller already in a folder keeps
// it and its workers join it, since moving it out would undo the user's own
// grouping; one in none goes into a folder named after its label, beside them.
func subagentFolder(target store.Project, fromID, originLabel string) (string, string) {
	for _, sess := range target.Sessions {
		if sess.ID == fromID && sess.Folder != "" {
			return sess.Folder, ""
		}
	}
	return originLabel, fromID
}

// fileCaller files the session that asked for a subagent under its workers'
// folder and tells the window, after the worker's card so the caller's move is
// never what hides it. A write that fails is reported once the worker is
// running, for the reason recordOverrides gives.
func (s *Service) fileCaller(projectID, callerID, folder string) error {
	if callerID == "" {
		return nil
	}
	if _, err := s.sessions.SetSessionFolder(context.Background(), api.SessionFolderOptions{SessionID: callerID, Folder: folder}); err != nil {
		return fmt.Errorf("the subagent is open, but its caller could not be filed under %q: %w", folder, err)
	}
	if s.events != nil {
		events.Send(s.events.Emit, api.SessionsFiledEvent, FiledEvent{ProjectID: projectID, IDs: []string{callerID}, Folder: folder})
	}
	return nil
}

// markSubagent records a subagent's depth on its row before its terminal
// starts, which reads it back (internal/terminal, subagentCardsOn): one level
// below the session that asked for it. A write that fails is reported once the
// session is running, for the reason recordOverrides gives.
func (s *Service) markSubagent(id, label string, req request) error {
	if !req.subagent {
		return nil
	}
	depth := s.sessions.SessionSubagentDepth(req.fromID) + 1
	if err := s.sessions.SetSessionSubagentDepth(id, depth); err != nil {
		return fmt.Errorf(
			"session %q is open, but it could not be marked as a subagent, so it runs as "+
				"a session nobody opened: its own subagents may open cards, and it does not "+
				"report back by itself: %w",
			label, err,
		)
	}
	return nil
}

// checkOverrides refuses a model, effort or ultracode the provider would
// silently drop, so the caller hears about it instead of getting a session on
// its defaults.
func checkOverrides(kind, model, effort string, ultracode bool) error {
	if model != "" && !terminal.SupportsModel(kind) {
		return fmt.Errorf(
			"%s cannot be told which model to run when lich starts it — open the session without a model and pick one inside it",
			kindName(kind),
		)
	}
	if effort != "" && !terminal.SupportsEffort(kind) {
		return fmt.Errorf(
			"%s cannot be told a reasoning effort when lich starts it: open the session without an effort and set one inside it",
			kindName(kind),
		)
	}
	if ultracode && !terminal.SupportsUltracode(kind) {
		return fmt.Errorf(
			"%s has no ultracode: only Claude Code sessions can be opened with it",
			kindName(kind),
		)
	}
	return nil
}

// recordOverrides writes the model, effort and ultracode a session was opened
// with onto its row, where every later spawn reads them back.
func (s *Service) recordOverrides(id, label, model, effort string, ultracode bool) error {
	if model != "" {
		if err := s.sessions.SetSessionModel(id, model); err != nil {
			return overrideNotRecorded(label, "model", err)
		}
	}
	if effort != "" {
		if err := s.sessions.SetSessionEffort(id, effort); err != nil {
			return overrideNotRecorded(label, "reasoning effort", err)
		}
	}
	if ultracode {
		if err := s.sessions.SetSessionUltracode(id); err != nil {
			return overrideNotRecorded(label, "ultracode", err)
		}
	}
	return nil
}

func overrideNotRecorded(label, what string, err error) error {
	return fmt.Errorf(
		"session %q is open, but the %s could not be recorded, so it runs on the "+
			"provider's own default: %w",
		label, what, err,
	)
}

// checkout is the worktree a session is being opened in: where it lives,
// whether it was just created, and what the card should be called — empty when
// the branch's name is already taken and the caller should fall back to the
// project's own counter.
// placement is where an opened session lives. cwd is where the PTY starts;
// stored is what the row records, which is empty for a session in the
// project's own directory (the store's spelling, and what the window's grouping
// reads).
type placement struct {
	cwd, stored, label string
}

// place settles where Open roots a session: the project's own directory under
// the project's next default label, or the worktree asked for (resolveCheckout).
func (s *Service) place(target store.Project, worktree, base string) (placement, error) {
	at := placement{cwd: target.Path, label: fmt.Sprintf("Session %d", target.NextSeq)}
	if worktree == "" {
		return at, nil
	}
	found, err := s.resolveCheckout(target, worktree, base)
	if err != nil {
		return placement{}, err
	}
	at.cwd, at.stored = found.path, storedPath(target.Path, found.path)
	if found.label != "" {
		at.label = found.label
	}
	return at, nil
}

type checkout struct {
	path  string
	label string
}

// resolveCheckout settles the worktree a session is opened in: the one already
// holding that branch, or a new one created off base.
//
// An existing checkout is opened, not created: asking for a worktree by the
// branch it holds is asking to work on that branch, and a caller that cannot
// see the repository has no way to know which of the two it is asking for.
//
// The card is named after the checkout, as the window names it — unless a
// session there already took that name. Two live sessions answering to one
// label is the one thing `lich send` cannot resolve, so the second is left
// unnamed here and takes the project's counter instead.
func (s *Service) resolveCheckout(target store.Project, worktree, base string) (checkout, error) {
	label := worktree
	if labelTaken(target, worktree, "") {
		label = ""
	}
	if path, ok := s.checkoutAt(target, worktree); ok {
		return checkout{path: path, label: label}, nil
	}
	wt, err := s.addWorktree(target, worktree, base)
	if err != nil {
		return checkout{}, err
	}
	return checkout{path: wt.Path, label: label}, nil
}

// addWorktree creates the checkout a worktree session lives in.
func (s *Service) addWorktree(target store.Project, name, base string) (*project.Worktree, error) {
	base, remote, err := s.resolveBase(target, base)
	if err != nil {
		return nil, err
	}
	wt, err := s.worktrees.CreateWorktree(context.Background(), api.CreateWorktreeOptions{ProjectPath: target.Path, ProjectID: target.ID, Name: name, Base: base, BaseIsRemote: remote})
	if err != nil {
		return nil, err
	}
	return wt, nil
}

// resolveBase settles what a new worktree branches off, and whether that base is
// a remote branch — which decides whether it is fetched first and tracked.
//
// An empty base is the project's current branch, the one thing a caller with no
// view of the repository can be assumed to have meant. A named base is looked up
// rather than trusted: git would happily branch off a typo that happens to be a
// valid revision, and the worktree it left behind is one nobody asked for.
func (s *Service) resolveBase(target store.Project, base string) (string, bool, error) {
	if base == "" {
		current := s.worktrees.Branch(target.Path)
		if current == "" {
			return "", false, fmt.Errorf(
				"%s has no current branch to start from (detached HEAD) — name one with the base",
				target.Name,
			)
		}
		return s.newerOf(target, current)
	}

	branches, err := s.worktrees.ListBranches(context.Background(), api.PathOptions{Path: target.Path})
	if err != nil {
		return "", false, err
	}
	for _, name := range branches.Local {
		if strings.EqualFold(name, base) {
			return s.newerOf(target, name)
		}
	}
	// A branch already checked out in another worktree is reported there rather
	// than among the local ones, and is still a perfectly good base to branch off.
	for _, wt := range branches.Worktrees {
		if strings.EqualFold(wt.Name, base) {
			return wt.Name, false, nil
		}
	}
	for _, name := range branches.Remote {
		if strings.EqualFold(name, base) {
			return name, true, nil
		}
	}
	return "", false, fmt.Errorf("no branch named %q in %s", base, target.Name)
}

// newerOf is where a worktree off the local branch starts: its upstream when
// that has commits the branch lacks and the branch has none of its own, the
// branch otherwise. A fetch that failed keeps the branch, the base the caller
// asked for; the native dialog decides the same way and passes the result in.
func (s *Service) newerOf(target store.Project, branch string) (string, bool, error) {
	f, err := s.worktrees.BaseFreshness(context.Background(), api.PathBranchOptions{Path: target.Path, Branch: branch})
	if err != nil {
		return "", false, err
	}
	if f.FetchError == "" && f.Upstream != "" && f.Behind > 0 && f.Ahead == 0 {
		return f.Upstream, true, nil
	}
	return branch, false, nil
}

// resolveProject picks the project the session lands in: the one named, or the
// caller's own when nothing is named. A name reaches only the projects the
// window is holding, and so does a path — opening one that is not on screen is
// ensureProject's, and only an open does it.
func resolveProject(projects []store.Project, fromID, name string) (store.Project, error) {
	if name == "" {
		for _, p := range projects {
			for _, sess := range p.Sessions {
				if sess.ID == fromID {
					return p, nil
				}
			}
		}
		return store.Project{}, fmt.Errorf(
			"no project given, and this is not running in a lich session — name one. %s",
			knownProjects(projects),
		)
	}

	match, err := projectFilter(name)
	if err != nil {
		return store.Project{}, err
	}
	var matches []store.Project
	for _, p := range projects {
		if match(p) {
			matches = append(matches, p)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		if looksLikePath(name) {
			return store.Project{}, fmt.Errorf("no open project at %q. %s", name, knownProjects(projects))
		}
		return store.Project{}, fmt.Errorf("no open project named %q. %s", name, knownProjects(projects))
	default:
		return store.Project{}, fmt.Errorf(
			"%q names %d open projects — they cannot be told apart by name", name, len(matches),
		)
	}
}

// resolveKind settles what the new session runs: the provider named, or the
// caller's own. A caller that is not a session at all falls back to the default
// provider, which is also what the window opens a session with.
func resolveKind(projects []store.Project, fromID, kind string) (string, error) {
	if kind != "" {
		if !providers.Known(kind) && kind != terminal.KindShell {
			return "", fmt.Errorf("%q is not a provider lich runs. %s", kind, knownKinds())
		}
		return kind, nil
	}
	for _, p := range projects {
		for _, sess := range p.Sessions {
			if sess.ID == fromID {
				return sess.Kind, nil
			}
		}
	}
	return providers.Claude, nil
}

// originOf names the session that asked for this one: its id, and the label it
// answers to at this moment. A fromID that names no session in the workspace —
// the command line run outside one, or a caller whose card has since gone — is
// no origin at all, because there would be nothing to draw for it.
func originOf(projects []store.Project, fromID string) (string, string) {
	if fromID == "" {
		return "", ""
	}
	for _, p := range projects {
		for _, sess := range p.Sessions {
			if sess.ID == fromID {
				return fromID, sess.Label
			}
		}
	}
	return "", ""
}

// labelTaken reports whether a project already holds a session under a label,
// ignoring the session exceptID names — the one a rename is about to give that
// label, which is not competition with itself.
func labelTaken(p store.Project, label, exceptID string) bool {
	for _, sess := range p.Sessions {
		if sess.ID == exceptID {
			continue
		}
		if strings.EqualFold(sess.Label, label) {
			return true
		}
	}
	return false
}

func knownProjects(projects []store.Project) string {
	if len(projects) == 0 {
		return "No project is open."
	}
	names := make([]string, 0, len(projects))
	for _, p := range projects {
		names = append(names, p.Name)
	}
	return "Open projects: " + strings.Join(names, ", ") + "."
}

// kindName is what a provider is called in an error the user reads, falling
// back to the kind itself — which is what a shell session has.
func kindName(kind string) string {
	for _, p := range providers.Registry {
		if p.ID == kind {
			return p.Name
		}
	}
	return kind
}

func knownKinds() string {
	kinds := make([]string, 0, len(providers.Registry)+1)
	for _, p := range providers.Registry {
		kinds = append(kinds, p.ID)
	}
	kinds = append(kinds, terminal.KindShell)
	return "Known kinds: " + strings.Join(kinds, ", ") + "."
}

// sessionIDBytes is the length of a session id before hex encoding. The window
// mints a UUID for the same column; both are opaque to everything that reads
// them, so this only has to be as unguessable and as unique.
const sessionIDBytes = 16

func newSessionID() (string, error) {
	raw := make([]byte, sessionIDBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

// storedPath is what the row records for a session rooted at cwd: empty for the
// project's own directory, the checkout itself for a worktree. The store's own
// spelling, and what the window's grouping reads.
func storedPath(projectPath, cwd string) string {
	if cwd == projectPath {
		return ""
	}
	return cwd
}
