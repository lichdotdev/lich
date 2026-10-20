package main

import (
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"syscall"
	"time"

	"github.com/lichdotdev/lich/internal/agentplugin"
	"github.com/lichdotdev/lich/internal/appupdate"
	"github.com/lichdotdev/lich/internal/claudeprofile"
	"github.com/lichdotdev/lich/internal/cli"
	"github.com/lichdotdev/lich/internal/codexprofile"
	"github.com/lichdotdev/lich/internal/drop"
	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/logging"
	"github.com/lichdotdev/lich/internal/ompprofile"
	"github.com/lichdotdev/lich/internal/patchnotes"
	"github.com/lichdotdev/lich/internal/project"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/quota"
	"github.com/lichdotdev/lich/internal/relay"
	"github.com/lichdotdev/lich/internal/restart"
	"github.com/lichdotdev/lich/internal/rpc"
	"github.com/lichdotdev/lich/internal/shellintegration"
	"github.com/lichdotdev/lich/internal/singleton"
	"github.com/lichdotdev/lich/internal/spawn"
	"github.com/lichdotdev/lich/internal/store"
	"github.com/lichdotdev/lich/internal/system"
	"github.com/lichdotdev/lich/internal/terminal"
	"github.com/lichdotdev/lich/internal/themes"
	"github.com/lichdotdev/lich/internal/tray"
	"github.com/lichdotdev/lich/internal/xdgprofile"
	"github.com/ncruces/zenity"
)

// changelog is embedded so the app can show a "what's new" popup after an
// update, parsed for the running version's section (internal/patchnotes).
//
//go:embed CHANGELOG.md
var changelog string

// changelogTranslations are the latest release's notes in each interface
// language (internal/patchnotes).
//
//go:embed changelog/*.md
var changelogTranslations embed.FS

// The tray icon, in the encoding each platform's tray takes (internal/tray).
var (
	//go:embed build/appicon-256.png
	trayPNG []byte
	//go:embed build/windows/lich.ico
	trayICO []byte
)

// trayIcons are the tray's icons; a dev instance's flame is amber, so it reads
// apart from an installed lich's beside it.
func trayIcons() tray.Icons {
	icons := tray.Icons{PNG: trayPNG, ICO: trayICO}
	if os.Getenv("LICH_DEV") == "" {
		return icons
	}
	devPNG, err := tray.DevPNG(trayPNG)
	if err != nil {
		slog.Warn("tray: dev icon", "err", err)
		return icons
	}
	devICO, err := tray.DevICO(trayICO)
	if err != nil {
		slog.Warn("tray: dev icon", "err", err)
		return icons
	}
	return tray.Icons{PNG: devPNG, ICO: devICO}
}

// version is the running build's version, injected at build time via
// -ldflags "-X main.version=<git tag>" (see Taskfile.yml). Unset in dev builds
// ("dev"), which the update check treats as "not a release".
var version = "dev"

func main() {
	// `lich <subcommand>` is the CLI a session's agent calls to reach the
	// sessions beside it (internal/cli). It answers before anything else here:
	// it must not open the database, take the log file or race the singleton
	// bind of the lich it is talking to. Anything that is not a subcommand
	// falls through and opens the app.
	if code := cli.Run(os.Args[1:], version, os.Getenv, os.Stdout, os.Stderr); code != cli.NotACommand {
		os.Exit(code)
	}

	noWindow := slices.Contains(os.Args[1:], restart.NoWindowArg)

	configDir, err := os.UserConfigDir()
	if err != nil {
		slog.Error("resolve config dir", "err", err)
		os.Exit(1)
	}
	// File logging before anything that can fail: every startup failure must be
	// readable after the fact — on Windows the console may not exist at all, and
	// a GUI launch (Finder, .desktop) has no stderr on any of them. The login
	// shell resolution below is why the order matters: it is the one failure
	// that leaves lich running on the launcher's bare PATH, and a warning about
	// it written before this line goes nowhere anybody can read.
	logDir := filepath.Join(configDir, "lich")
	logPath := logging.Path(logDir)
	if closer, err := logging.Init(logDir); err != nil {
		slog.Warn("file log unavailable, stderr only", "err", err)
		// Nothing to reveal or attach to a bug report; the Help section says so
		// rather than pointing at a file that was never written.
		logPath = ""
	} else {
		defer closer.Close()
	}

	launchEnv, env := resolveEnv()

	db, err := store.New()
	if err != nil {
		slog.Error("open store", "err", err)
		// Before the listener and the window, so the dialog is the only surface:
		// a launcher start would otherwise end in silence (handleBindFailure).
		_ = zenity.Error(fmt.Sprintf("lich %s could not open its workspace database.\n\n%v\n\nLog: %s",
			version, err, logPath), zenity.Title("lich"))
		os.Exit(1)
	}
	defer db.Close()

	// App events ride the /events socket; no client connected means no
	// listener yet (the window is still starting) and the event is dropped.
	hub := events.New()
	term := terminal.New(db, env, hub)
	// After db's own defer, so it runs before the database closes: what every
	// session was worked in this run is still in memory until something writes
	// it (internal/terminal.handsOn).
	defer term.FlushHandsOn()
	coord := newCoordinator()
	sys := registerServices(db, term, hub, configDir, logPath, env, launchEnv, coord)

	term.SetRestart(coord.Do)

	run(term, configDir, coord, startWindow(sys, noWindow), closeWindow(hub))
}

// windowStart is what a launch does about lich's window: open it, its own or
// the one of a lich already running. --no-window does neither, and leaves the
// backend for a client of its own (`lich native`); the tray and a second
// launch still open the window later.
type windowStart struct {
	open        func()
	openRunning func(configDir string, running *singleton.Info)
}

func startWindow(sys *system.Service, noWindow bool) windowStart {
	if noWindow {
		return windowStart{open: func() {}, openRunning: func(string, *singleton.Info) {}}
	}
	return windowStart{open: func() { showWindow(sys, "at launch") }, openRunning: showRunning}
}

// resolveEnv returns the environment lich was launched in and the one its
// children get, and pins the listener port into both.
//
// The launch environment is snapshotted before any tweak, minus the restart
// marker that is this process's alone (restart.WithoutMarker), because spawned
// terminal sessions must inherit what the user launched lich with (see
// terminal.childEnv) and because a re-check resolves from it again, exactly as
// this does (providers.Service.RefreshPath). ResolveShellEnv recovers the
// rc-exported vars a GUI launch misses (see its doc); its slice is what
// children inherit, and exec.LookPath reads the process PATH instead, so the
// resolved one has to land there too (see PinPath).
//
// The port is pinned rather than only resolved so the restart successor and
// every spawned session agree on where lich listens (singleton.DefaultPort).
func resolveEnv() (launchEnv, env []string) {
	launchEnv = restart.WithoutMarker(os.Environ())
	env = terminal.ResolveShellEnv(launchEnv)
	terminal.PinPath(env)
	if os.Getenv("LICH_LISTEN_PORT") == "" {
		if err := os.Setenv("LICH_LISTEN_PORT", strconv.Itoa(singleton.DefaultPort)); err != nil {
			slog.Error("set LICH_LISTEN_PORT", "err", err)
			os.Exit(1)
		}
	}
	return launchEnv, env
}

// registerServices wires every service the window reaches over the loopback RPC,
// in the order main built them: a service that hands another one a callback has
// to exist first. It is lifted out of main only for its length; nothing here
// runs anywhere else.
func registerServices(db *store.Service, term *terminal.Service, hub *events.Hub,
	configDir, logPath string, env, launchEnv []string, coord *restart.Coordinator) *system.Service {
	proj := project.New(project.ZenityPicker{})
	// gh has one active account per host; a project can name a different one.
	proj.SetAccounts(db.GHAccountForPath)
	// Relocating a project is the one flow that can point two rows at the same
	// directory, so it validates the picked one against the workspace.
	proj.SetProjects(db.ProjectAt)
	proj.SetEmit(hub.Emit)
	// Parking a session records the branch its checkout was on, so the history
	// search can match a branch the row is showing (store.SetBranchOf), and a
	// copy of what was said in it, capped, so the same search reaches the
	// conversation and not only the names around it.
	db.SetBranchOf(proj.Branch)
	db.SetTranscriptOf(terminal.TranscriptText)
	// The history offers conversations started outside lich in a project's
	// checkouts (store.ExternalSessions): the providers' stores list them, git
	// says which directories are the project's.
	db.SetConversationsOf(terminal.Conversations)
	db.SetCheckoutsOf(func(path string) ([]string, error) {
		// Asked for every project on each listing: one that is not a repository,
		// or whose directory is gone, would log a git failure every time.
		if _, err := os.Stat(filepath.Join(path, ".git")); err != nil {
			return nil, err
		}
		checkouts, err := proj.ListCheckouts(path)
		if err != nil {
			return nil, err
		}
		paths := make([]string, len(checkouts))
		for i, c := range checkouts {
			paths[i] = c.Path
		}
		return paths, nil
	})

	// Every service the window uses goes through the loopback RPC
	// (internal/rpc). store.Close manages the DB lifecycle and stays Go-only.
	dispatcher := rpc.New()
	// db.SessionExists is what keeps the age rule off a live session's copies:
	// they are deleted by the session's row going away, not by the clock.
	drops := drop.New(configDir, db.SessionExists)
	// The other prune runs after each new copy; this one is what clears the
	// last of them for a lich that is never dropped on again — and the only one
	// that may remove a session's directory outright, no session having spawned
	// around it yet.
	drops.PruneStale()
	// A copy belongs to the session it was dropped into: that session reads it
	// through a bind mount when it is confined, and deleting the session's row
	// takes the copies with it.
	term.SetDropDir(drop.Dir(configDir))
	// A shell whose prompt is not marked still works; it only keeps the lines
	// a resize wraps on screen.
	shellScripts := shellintegration.Dir(configDir)
	if err := shellintegration.Install(shellScripts); err != nil {
		slog.Warn("shell integration unavailable", "err", err)
	} else {
		term.SetShellIntegrationDir(shellScripts)
	}
	db.SetSessionGone(drops.Purge)
	// The footer's attach button opens the picker through the drop service, so
	// the file it copies for a confined session is one a human chose in a dialog
	// (drop.Attach says why that matters).
	drops.SetPicker(proj.PickFile)
	dispatcher.Register("terminal", term)
	dispatcher.Register("drop", drops)
	dispatcher.Register("project", proj)
	plugins := agentplugin.New(db)
	dispatcher.Register("agentplugin", plugins)
	go plugins.RepairRegistrations()
	// A profile follows Personal's lich plugin: an install or update there
	// reaches every Claude Code and Codex profile, and a start brings in the
	// ones left behind or logged in before they got it.
	plugins.SetEmit(hub.Emit)
	go plugins.SyncProfiles()
	// Every Claude Code profile gets the lich plugin once it is logged in, run as
	// that profile so its account's plugin policy applies; a profile with a
	// session running in it is never deleted, and deleting one clears the
	// settings that named it.
	profiles := claudeprofile.New(db, hub.Emit)
	profiles.SetLichPlugin(plugins.InstallInClaudeProfile, agentplugin.PluginKey)
	profiles.SetSessions(term.ProfileSessions, func(slug string) error {
		dir, err := claudeprofile.ConfigDir(slug)
		if err != nil {
			return err
		}
		return errors.Join(quota.ForgetClaudeLogin(dir), db.ForgetClaudeProfile(slug))
	})
	dispatcher.Register("claudeprofile", profiles)
	// Codex profiles have plugin sets of their own too, and get the lich plugin
	// the same way.
	codexProfiles := codexprofile.New(db, hub.Emit)
	codexProfiles.SetLichPlugin(plugins.InstallInCodexProfile, agentplugin.PluginKey)
	codexProfiles.SetSessions(term.CodexProfileSessions, db.ForgetCodexProfile)
	dispatcher.Register("codexprofile", codexProfiles)
	// oh-my-pi's login is a terminal card's, so its service runs no login of its
	// own and has nothing to report on /events; it lists, creates and deletes.
	ompProfiles := ompprofile.New(db)
	ompProfiles.SetSessions(term.OMPProfileSessions, db.ForgetOMPProfile)
	dispatcher.Register("ompprofile", ompProfiles)
	// opencode, Cursor CLI and Kiro CLI log in from a terminal card too, and have
	// profiles only where a variable moves their logins (xdgprofile.Providers).
	for _, provider := range xdgprofile.Providers() {
		profiles := xdgprofile.New(provider, db)
		profiles.SetSessions(
			func(slug string) int { return term.XDGProfileSessions(provider.ID, slug) },
			func(slug string) error { return db.ForgetXDGProfile(provider.ID, slug) },
		)
		dispatcher.Register(provider.ID+"profile", profiles)
	}
	dispatcher.Register("appupdate", appupdate.New(version, coord.Install, hub.Emit))
	dispatcher.Register("patchnotes", patchnotes.New(version, changelog, changelogTranslations))
	dispatcher.Register("store", db)
	// Read before run writes this run's runtime file over the previous
	// run's: a file still there is the only trace a bad exit leaves, and the
	// restored workspace looks exactly like one closed on purpose.
	uncleanExit := singleton.UncleanExit(configDir, os.Getenv(restart.WaitEnv))
	sys := system.New(env, logPath, version, uncleanExit)
	// A second launch of lich asks for the window here, and `lich quit` is the
	// one way to end a lich whose window is closed.
	sys.SetQuit(coord.Quit)
	sys.SetNativeWindow(system.NativeHooks{
		Raise:  raiseNative(hub),
		Launch: launchNative(term, filepath.Join(configDir, "lich")),
		Locate: func() (string, error) { return restart.NativeExecutable(os.Getenv) },
		Failed: windowFailed(configDir, term.LiveCount, coord.Quit),
	})
	dispatcher.Register("system", sys)
	// Deleting a session for good takes any prompt parked on it, and the row is
	// the only copy of what the user wrote. Both channels are used, and neither
	// is the other's fallback: the toast is what the user reading the card sees,
	// and the desktop notification is what reaches a forfeit nobody was at the
	// window for — an agent closing a session while the window reconnects, where
	// an event with no client connected is dropped.
	db.SetScheduleForfeited(func(lost store.ForfeitedSchedule) {
		hub.Emit(store.ScheduleForfeitEventName, lost)
		summary, detail := lost.Notice()
		if err := sys.Notify(summary, detail); err != nil {
			slog.Warn("notify forfeited schedule", "session", lost.Label, "err", err)
		}
	})
	db.SetSessionRenamed(func(id, label string) {
		hub.Emit(spawn.RenamedEventName, spawn.RenamedEvent{ID: id, Label: label})
	})
	db.SetProjectChanged(hub.Emit)
	providerSvc := providers.New()
	// One resolution, three readers: the process PATH exec.LookPath resolves a
	// binary through, what a new session inherits, and the editor lookup. They
	// are replaced together or not at all — a failed re-read leaves every one of
	// them on the pin lich booted with, which is what the surface reports.
	providerSvc.SetPathRefresh(func() error {
		next, err := terminal.ReresolveShellEnv(launchEnv)
		if err != nil {
			return err
		}
		terminal.PinPath(next)
		term.SetEnv(next)
		sys.SetEnv(next)
		return nil
	})
	// Detection resolves a provider the way the spawn does, so an agent reached
	// only through the binary setting counts as installed — otherwise a machine
	// with one configured agent and nothing on PATH reads as bare, and every
	// implicit new session opens a terminal instead.
	providerSvc.SetConfiguredBin(func(id string) string { return db.ProviderBin(id, "") })
	dispatcher.Register("providers", providerSvc)
	// The quota reading is per session, not per machine: a session spawned from
	// a binary the user configured can spend another account entirely, and the
	// terminal is what knows the environment that binary set up.
	plans := quota.New()
	plans.SetSessions(func(sessionID string) quota.Account {
		env, read := term.SessionAccount(sessionID)
		return quota.Account{Env: env, Read: read}
	})
	plans.SetEnabled(func(provider string) bool {
		on, err := db.ProviderEnabled(provider)
		if err != nil {
			slog.Error("quota: read provider switch", "provider", provider, "err", err)
		}
		return on
	})
	term.SetRateLimitReports(plans.ReportClaude)
	dispatcher.Register("quota", plans)
	codexProfiles.SetAccount(func(slug string) (string, string) {
		plan, err := plans.CodexProfilePlan(slug)
		if err != nil || plan.Status != quota.StatusOK {
			return "", ""
		}
		return plan.Account, plan.Plan
	})
	// The relay is the only service whose caller is not the window: the `lich`
	// CLI running inside a session reaches it over the same listener. It watches
	// the hooks' state reports too, to notice a target that ends a turn without
	// answering the request it was given.
	rl := relay.New(db, term, hub)
	term.SetSessionState(rl.Observe)
	// A worker's mod reports its answer, and a closed worker's errand ends
	// without a word to its caller.
	term.SetWorkerAnswer(rl.WorkerAnswered)
	term.SetWorkerUnanswered(rl.WorkerUnanswered)
	term.SetSessionClosed(rl.SessionClosed)
	term.SetSessionExited(rl.SessionExited)
	term.SetErrandStatus(rl.Status)
	// Both questions the relay asks about a provider — whether its sessions
	// report at all, and whether they can answer with a tool — are about what
	// the plugin put there.
	rl.SetPlugins(plugins)
	// `lich focus` opens a card for someone outside the window, who then needs
	// the window in front, or open again: what a second launch of lich asks for.
	rl.SetRaiseWindow(func(string) { showWindow(sys, "for lich focus") })
	// The language of the text lich types into sessions is read live from the
	// settings table, by the relay at every message, by the terminal at
	// every spawn and by drop at every copy notice.
	rl.SetPromptLanguage(db.PromptLanguage)
	drops.SetPromptLanguage(db.PromptLanguage)
	term.SetPromptLanguage(db.PromptLanguage)
	// A session whose provider runs lich's hooks is held until its session-start
	// report, which comes only from the agent's own prompt, past a trust
	// question that a quiet screen cannot be told apart from.
	term.SetStartReports(plugins.Installed)
	// The scheduled prompts are the relay's other clock: nobody calls in for
	// them, they come due.
	go rl.RunSchedules()
	// A turn a usage limit ended is picked up again once it resets, through
	// the same scheduled prompt. Codex reports no end for that turn, so the
	// terminal watches its open turns for one.
	term.SetUsageLimit(rl.ParkResume)
	go term.RunLimitWatch()
	dispatcher.Register("relay", rl)
	// Its caller is not the window either: opening a session for an agent starts
	// the PTY here rather than waiting for someone to click the card.
	spawner := spawn.New(db, proj, term, hub)
	// A subagent worker in its caller's checkout closes once it reported and
	// its turn ended, the way a native subagent ends with its result.
	rl.SetWorkerFinished(spawner.CloseFinishedWorker)
	dispatcher.Register("spawn", spawner)
	dispatcher.Register("themes", themes.New(version, db, hub.Emit))
	// The tray is what a lich with its window closed shows of itself; it comes
	// up once the window hands it its words (tray.SetLabels).
	trayIcon := tray.New(trayIcons(), func() { showWindow(sys, "from the tray") }, coord.Quit, term.LiveCount)
	dispatcher.Register("tray", trayIcon)
	denyInternal(dispatcher)
	term.Mount("/rpc/", dispatcher)
	term.Mount("/drop", http.HandlerFunc(drops.Upload))
	term.Mount("/blob", http.HandlerFunc(proj.ServeBlob))
	term.Mount("/events", hub)
	return sys
}

// denyInternal hides the methods that exist for Go callers inside this process
// but must never answer an RPC POST. Registration exposes every exported method
// of a service (internal/rpc), so each of these is one /rpc/ path away from the
// window:
//
//   - store.Close ends the database the whole app is still using.
//   - drop.Upload and drop.Save take the dropped file's bytes as the request
//     body, so the upload is its own endpoint — the RPC envelope is a JSON
//     argument array with a 1MB bound.
//   - relay.Observe is the hooks' session-state stream, which arrives over
//     /hook: forging a SessionEnd here closes another session's errands.
//   - relay.WorkerAnswered is a worker's answer, which arrives over
//     /mod/answer: called here it answers another session's errand.
//     relay.WorkerUnanswered arrives the same way and ends one unanswered.
//   - relay.SessionClosed is what the terminal tells the relay as it closes a
//     session: called here it ends a running worker's errand, in silence when
//     the closer named is the worker's caller. relay.SessionExited is what it
//     tells the relay as it reaps a process that ended on its own: called here
//     it erases a live session's reported state.
//   - terminal.CloseBy is a close on behalf of a session, which `lich close`
//     reaches through spawn.Close: called here with the worker's caller as the
//     closer, it ends that caller's errand without telling it. The window
//     closes with terminal.Close.
//   - drop.Purge deletes every copy dropped into a session, by id: the window
//     closes sessions through the store, which is what reports one gone.
//   - drop.SetPicker is startup wiring like the ones below, and nilling it
//     would leave the footer's attach button with no dialog to open.
//   - terminal.SessionAccount hands back the environment of the process a
//     session runs, credentials and all, so the quota reader can tell which
//     account that session spends.
//   - agentplugin.RepairRegistrations rewrites provider config files and
//     shells out to their CLIs; it is a launch step, not something a window asks.
//   - relay.RunSchedules is the scheduled-prompt loop, started once at launch
//     and never returning: called over /rpc/ it holds that request open for the
//     life of the process and starts a second loop racing the first for every
//     due prompt. terminal.RunLimitWatch is the same kind of loop, for the
//     Codex turns a usage limit ended.
//   - relay.ParkResume parks a prompt that types itself at a session; the
//     terminal calls it for a turn a usage limit ended, with the reset it read.
//   - spawn.CloseFinishedWorker closes a session with none of spawn.Close's
//     checks; the relay calls it for a worker that reported back.
//   - relay.SetPlugins, relay.SetWorkerFinished, relay.SetPromptLanguage, drop.SetPromptLanguage, project.SetAccounts, project.SetProjects,
//     system.SetQuit, system.SetNativeWindow,
//     quota.SetSessions, store.SetSessionGone, store.SetScheduleForfeited,
//     store.SetSessionRenamed, store.SetProjectChanged,
//     store.SetBranchOf, store.SetTranscriptOf, store.SetConversationsOf,
//     store.SetCheckoutsOf, terminal.SetDropDir,
//     terminal.SetRateLimitReports, terminal.SetUsageLimit, terminal.SetWorkerAnswer,
//     terminal.SetWorkerUnanswered, terminal.SetStartReports, terminal.SetPromptLanguage, terminal.SetSessionClosed and terminal.SetSessionExited are startup wiring. Called with [null] they silently
//     nil what they wired (encoding/json leaves a func or pointer alone on
//     null), and the write races the readers already serving — nilling
//     SetProjects also disarms the guard that keeps two projects off the same
//     directory, and SetDropDir points the sandbox's read-only bind wherever
//     the caller likes.
//   - store.SetSessionProfile moves a session to another Claude Code profile
//     without moving its conversation, which only terminal.ContinueIn may do;
//     store.ForgetClaudeProfile, store.ForgetCodexProfile,
//     store.ForgetOMPProfile and store.ForgetXDGProfile are the delete's own
//     cleanup.
//   - agentplugin.SetEmit is startup wiring like the setters above, and
//     agentplugin.SyncProfiles is the start's pass over the profiles: called
//     over /rpc/ it would run a second one racing the first.
//   - claudeprofile.SetLichPlugin, claudeprofile.SetSessions,
//     codexprofile.SetLichPlugin, codexprofile.SetSessions,
//     codexprofile.SetAccount and
//     ompprofile.SetSessions and the SetSessions of opencodeprofile,
//     cursorprofile and kiroprofile are startup wiring like the setters above.
//   - quota.ReportClaude is a session's own reading of its plan, which arrives
//     over /mod/usage: called here it would put any numbers on the gauge.
//   - terminal.EnqueueModCommand and terminal.RunModCommand queue a command for
//     a session's mod with none of spawn.Control's checks (Claude Code only,
//     never the caller's own session); `lich control` and control_session
//     reach them through spawn.Control, `lich ask` and ask_session through
//     spawn.Ask.
//   - terminal.SubmitPrompt queues a prompt for a session's mod with none of the
//     relay's ticket accounting; the relay is its one caller.
func denyInternal(d *rpc.Handler) {
	for _, method := range []string{
		"store.Close",
		"system.SetQuit",
		"system.SetNativeWindow",
		"store.SetSessionGone",
		"store.SetScheduleForfeited",
		"store.SetSessionRenamed",
		"store.SetProjectChanged",
		"store.SetBranchOf",
		"store.SetTranscriptOf",
		"store.SetConversationsOf",
		"store.SetCheckoutsOf",
		"drop.Upload",
		"drop.Save",
		"drop.Purge",
		"drop.SetPicker",
		"drop.SetPromptLanguage",
		"relay.Observe",
		"relay.WorkerAnswered",
		"relay.WorkerUnanswered",
		"relay.SessionClosed",
		"relay.SessionExited",
		"terminal.CloseBy",
		"relay.RunSchedules",
		"relay.ParkResume",
		"agentplugin.RepairRegistrations",
		"agentplugin.SetEmit",
		"agentplugin.SyncProfiles",
		"relay.SetPlugins",
		"relay.SetWorkerFinished",
		"relay.SetPromptLanguage",
		"spawn.CloseFinishedWorker",
		"project.SetAccounts",
		"project.SetProjects",
		"quota.SetSessions",
		"quota.ReportClaude",
		"terminal.SessionAccount",
		"terminal.SetDropDir",
		"terminal.SetRateLimitReports",
		"terminal.SetUsageLimit",
		"terminal.RunLimitWatch",
		"terminal.SetWorkerAnswer",
		"terminal.SetWorkerUnanswered",
		"terminal.SetSessionClosed",
		"terminal.SetSessionExited",
		"terminal.SetStartReports",
		"terminal.SetPromptLanguage",
		"terminal.EnqueueModCommand",
		"terminal.RunModCommand",
		"terminal.SubmitPrompt",
		"store.SetSessionProfile",
		"store.ForgetClaudeProfile",
		"store.ForgetCodexProfile",
		"store.ForgetOMPProfile",
		"store.ForgetXDGProfile",
		"claudeprofile.SetLichPlugin",
		"claudeprofile.SetSessions",
		"codexprofile.SetLichPlugin",
		"codexprofile.SetSessions",
		"codexprofile.SetAccount",
		"ompprofile.SetSessions",
		"opencodeprofile.SetSessions",
		"cursorprofile.SetSessions",
		"kiroprofile.SetSessions",
	} {
		d.Deny(method)
	}
}

// run serves the loopback listener, opens the window on it (the native one,
// a process of its own) and serves until lich is told to quit. The window is
// not the app's lifecycle: closing it leaves every session running, and a
// second launch opens a new one on them (handleBindFailure).
func run(term *terminal.Service, configDir string, coord *restart.Coordinator, start windowStart, closeNative func()) {
	info := term.Transport()
	if info.Port == 0 {
		handleBindFailure(configDir, term.TransportError(), start.openRunning) // never returns
	}

	// The runtime file lets install.sh reach a running lich for /restart when it
	// runs outside a lich terminal (no LICH_PORT/LICH_TOKEN in the env), and lets
	// a second launch find this instance to show its window instead of dying (see
	// handleBindFailure). Removed on the clean exit; a stale file from a crash is
	// harmless (the token check rejects a mismatched or dead listener).
	if path, err := singleton.Write(configDir, info.Port, info.Token); err != nil {
		slog.Warn("runtime file", "err", err)
	} else {
		defer func() { _ = os.Remove(path) }()
	}

	serve(coord, start.open, closeNative)
}

// serve opens the window and returns when lich is told to quit (`lich quit`,
// a restart, SIGINT or SIGTERM), so the caller's defers still run. The native
// window is told to close unless it moves onto a successor, which only a
// restart in place has: an update on Windows closes it too.
func serve(coord *restart.Coordinator, open, closeNative func()) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)
	coord.SetStop(func() {
		// A second quit while the first is unwinding has nothing left to do.
		select {
		case stop <- os.Interrupt:
		default:
		}
	})
	open()
	<-stop
	slog.Info("quitting")
	if !coord.WindowMovesToSuccessor() {
		closeNative()
	}
}

// newCoordinator is the in-place restart: the update flow (install.sh) POSTs
// /restart after replacing the binary. os.Environ() here carries the pinned
// LICH_LISTEN_PORT so the successor rebinds the same port. A missing
// executable path only disables restart; the app still runs.
func newCoordinator() *restart.Coordinator {
	exe, err := os.Executable()
	if err != nil {
		slog.Warn("resolve executable — restart disabled", "err", err)
		exe = ""
	}
	return restart.New(exe, os.Environ())
}

// windowFailed is the dialog for a window that would not open. A launcher
// start has no terminal, so without it a window that will not start reads as
// lich doing nothing (#409). With no session running there is nothing left to
// keep lich for, and it quits. The dialog waits on its button off the caller's
// goroutine: a second launch asking for the window is waiting on the reply.
func windowFailed(configDir string, live func() int, quit func() error) func(string) {
	return func(reason string) {
		slog.Error("native window", "reason", reason)
		running := live() > 0
		text := fmt.Sprintf("lich could not open its window.\n\n%s\n\n", reason)
		if running {
			text += "lich is still running with its sessions: launch it again to reopen the window, " +
				"or run `lich quit` to end it.\n\n"
		}
		text += "Log: " + logging.Path(filepath.Join(configDir, "lich"))
		go func() {
			_ = zenity.Error(text, zenity.Title("lich"))
			if running {
				return
			}
			if err := quit(); err != nil {
				slog.Warn("quit after the window failed", "err", err)
			}
		}()
	}
}

// handleBindFailure runs when the pinned listener would not bind, and never
// returns. It gathers the two inputs the decision needs, asks
// singleton.BindFailureVerdict what they mean, and performs the effects. cause
// is the bind error, carried into the log for the launches that end here, and
// openRunning is what a duplicate launch asks of the lich already running.
func handleBindFailure(configDir string, cause error, openRunning func(string, *singleton.Info)) {
	port := os.Getenv("LICH_LISTEN_PORT")
	restartWait := os.Getenv(restart.WaitEnv)
	// A restart successor never probes: the verdict is already decided, and the
	// probe would only cost it a timeout on a port it is racing for.
	var running *singleton.Info
	if restartWait == "" {
		want, _ := strconv.Atoi(port)
		running, _ = singleton.Detect(configDir, want, singleton.Ping)
	}
	if singleton.BindFailureVerdict(restartWait, running) == singleton.BindFailureIsDuplicate {
		slog.Info("lich already running, showing its window",
			"pid", running.PID, "port", running.Port)
		openRunning(configDir, running)
		os.Exit(0)
	}
	// No "is the port free?" here: on Windows the answer is regularly yes and
	// the bind still fails, so the OS error is the message.
	slog.Error("loopback listener failed to start", "port", port, "err", cause)
	// The window never opens on this path, so a double-click launch dies in
	// silence with only the log to explain — and nothing on screen says a log
	// exists. The dialog is the one surface left. Best effort: where no dialog
	// backend answers, the log line above still has the story.
	_ = zenity.Error(fmt.Sprintf(
		"lich could not listen on 127.0.0.1:%s.\n\n%v\n\n"+
			"If another program is holding the port, set the LICH_LISTEN_PORT "+
			"environment variable to a free one and launch lich again.\n\n"+
			"Log: %s",
		port, cause, logging.Path(filepath.Join(configDir, "lich"))),
		zenity.Title("lich"))
	os.Exit(1)
}

// windowRaiseEvent asks the native window on /events to come forward, and
// windowCloseEvent to close, lich quitting.
const (
	windowRaiseEvent = "window-raise"
	windowCloseEvent = "window-close"
)

// windowLeaveWait bounds waiting for the native window to end once told to
// close. It ends at once (os.Exit on the event); the bound is for a window
// that never reads it, which must not keep lich from quitting.
const windowLeaveWait = 5 * time.Second

// closeWindow tells the native window to close and waits for its process to
// end, so a Windows update that runs once lich has exited finds lich-native.exe
// free to replace.
func closeWindow(hub *events.Hub) func() {
	return func() {
		gone := hub.Gone()
		hub.Emit(windowCloseEvent, nil)
		select {
		case <-gone:
		case <-time.After(windowLeaveWait):
			slog.Warn("the window did not close", "waited", windowLeaveWait)
		}
	}
}

// raiseNative brings the native window forward through /events, which only it
// holds once it attached.
func raiseNative(hub *events.Hub) func() bool {
	return func() bool {
		if !hub.Attached() {
			return false
		}
		hub.Emit(windowRaiseEvent, nil)
		return true
	}
}

// launchNative starts the native window again on this backend, pointed at it
// the way `lich native` points the first one, logs how it ends and hands
// that end to exited.
func launchNative(term *terminal.Service, logDir string) func(string, func(string)) error {
	return func(executable string, exited func(string)) error {
		info := term.Transport()
		env := append(os.Environ(), "LICH_PORT="+strconv.Itoa(info.Port), "LICH_TOKEN="+info.Token)
		cmd, err := restart.StartNative(executable, env, logDir)
		if err != nil {
			return err
		}
		go func() {
			restart.WatchNative(cmd)
			exited(cmd.ProcessState.String())
		}()
		return nil
	}
}

// showWindow opens the window where nobody waits on the answer but the log:
// the launch and the tray.
func showWindow(sys *system.Service, from string) {
	if err := sys.ShowWindow(system.ShowWindowOptions{}); err != nil {
		slog.Warn("show the window "+from, "err", err)
	}
}

// showRunning asks the running lich for its window: it opens one when the user
// had closed it, and brings the open one forward otherwise. The running lich
// owns that window, so it is tied to that backend and not to this launch,
// which exits.
func showRunning(_ string, running *singleton.Info) {
	if err := singleton.Show(running); err != nil {
		slog.Warn("show the running lich's window", "err", err)
	}
}
