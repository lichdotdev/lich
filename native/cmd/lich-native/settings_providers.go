package main

import (
	"fmt"
	"log"
	"slices"
	"strings"
	"sync"

	"gioui.org/widget"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// ponytail: English until the native window has i18n (settings.providersPane,
// settings.providerDetail, settings.providerBinary, settings.providerBinSettings,
// settings.planUsageSetting, settings.ultracodeSetting,
// settings.subagentCardsSetting, settings.pasteUnfoldSetting,
// settings.restoreSetting, settings.providerToggleRow,
// settings.providerDocsLink, env.providerSummary).
const (
	providersDetecting   = "Detecting providers…"
	providersAdd         = "Add provider"
	providersMoreFound   = "%d more detected"
	providersCheckAgain  = "Check again"
	providersChecking    = "Checking…"
	providersPathReread  = "PATH could not be re-read; relaunch lich." // path-refresh.ts PATH_REREAD_FAILED
	providersNotFound    = "Not found on PATH"
	providersHowToGet    = "How to install"
	providersInstalled   = "Installed"
	providersCustomPath  = "Installed · custom path"
	providersSignedOut   = "Signed out"
	providersWindowPct   = "%d%% of the %s window"
	providersPercentUsed = "%d%% used"
	providerBack         = "All providers"
	providerEnabledLabel = "Enabled"
	providerDocsLabel    = "Documentation"
	providerTagDefault   = "default here"
	providerTagNoPrompts = "no prompts"

	providerDefaultTitle = "Default provider"
	providerDefaultAlso  = "agent new session"
	providerDefaultDesc  = "Which provider implicit session actions spawn: the new-session shortcut, an empty project's New session button, and newly created worktrees."

	planTitle = "Plan usage"
	planAlso  = "quota subscription limit"
	planDesc  = "How much of each window your subscription has spent, read from your account and refreshed every 5 minutes while lich is open."

	binTitle        = "Binary"
	binAlso         = "path executable command"
	binDesc         = "Which executable a session spawns."
	binDescProject  = "Which executable a session in %s spawns."
	binPickTitle    = "Choose the %s binary"
	binBrowse       = "Browse"
	binSourceName   = "set for %s"
	binSourceThis   = "set for this project"
	binSourceGlobal = "set for all projects"
	binSourcePath   = "from $PATH"
	binGlobalPath   = "$PATH"

	skipTitle        = "Skip permission prompts"
	skipAlso         = "yolo dangerous approvals"
	skipDesc         = "How far %s runs without asking: it edits files, runs commands and installs things unconfirmed, and lich spawns it with %s."
	skipConfirmTitle = "Skip permission prompts: %s"
	skipConfirmDesc  = "%s will edit files, run commands and install things unconfirmed, spawned with %s. %s"
	skipConfirm      = "Skip prompts"

	ultracodeTitle = "Ultracode"
	ultracodeAlso  = "workflow orchestration multi-agent effort"
	ultracodeDesc  = "Every %s session starts with ultracode on, orchestrating multi-agent workflows at whatever effort it runs. It stays on when a session restarts or resumes."

	subagentTitle = "Subagents as lich sessions"
	subagentAlso  = "agent subagent card worktree delegate background"
	subagentDesc  = "A general-purpose subagent opens its own card and worktree instead of running hidden inside %s. Applies to sessions opened after you change it."

	pasteTitle           = "Unfold long pastes"
	pasteAlso            = "paste fold chip summary placeholder pasted text"
	pasteDescNextPaste   = "A paste %s would fold into a placeholder lands as the full text, so you can edit it before sending. Applies to the next paste, in every %s session."
	pasteDescNextSession = "A paste %s would fold into a placeholder lands as the full text, so you can edit it before sending. Applies to sessions opened after you change it."

	restoreTitle = "Restored sessions"
	restoreAlso  = "resume conversation start new restart ask"
	restoreDesc  = "What a card that ran %s before lich restarted does the first time you open it."
)

// ponytail: English until the native window has i18n (env.binary).
const (
	binExecutable    = "executable"
	binNoSuchFile    = "no such file"
	binNotOnPath     = "not on $PATH"
	binNotExecutable = "not executable"
	binHomeShortcut  = "~ is not expanded"
	binRelativePath  = "relative path"
	binDetailHome    = "lich spawns the binary directly, so ~ is taken literally rather than expanded. Use the full path."
	binDetailRel     = "A relative path is resolved against each session's own working directory, so it names a different binary per session. Use a full path."
	binDetailBroken  = "Sessions will not start until this is fixed. An override can also be switched off or cleared, falling back to the layer below."
	binParkedProject = "%s override off"
	binParkedFallbck = "project"
	binParkedGlobal  = "global override off"
)

// The binary layers, in the order the spawn reads them (store.ProviderBin).
const (
	binScopeProject = "project"
	binScopeGlobal  = "global"
	binScopePath    = "path"
)

// The skip ladder's rungs, ordered by exposure (providers-store.ts SkipLevel).
const (
	skipNever      = "never"
	skipWorktrees  = "worktrees"
	skipEverywhere = "everywhere"
)

var skipRungs = []struct{ value, label, consequence string }{
	{skipNever, "Never", "Every edit and command waits for you, in every checkout."},
	{skipWorktrees, "Worktrees only", "Sessions in the project directory keep asking."},
	{skipEverywhere, "Everywhere", "Including the tree you work in. Nothing will ask."},
}

// restoreRungs are the three answers of the resume prompt, as the value the
// spawn gate reads (restoreChoice).
var restoreRungs = []struct{ value, label, consequence string }{
	{restoreAsk, "Ask", "Every restored card asks before it opens."},
	{restoreResume, "Resume", "Restored cards continue their conversation on open. For an empty one, open a new session."},
	{restoreFresh, "Start new", "Restored cards open empty. The old conversation is not picked up."},
}

// skipFlags is terminal.skipPermissionFlags: how each provider spells "run
// every tool without asking". A provider listed here has the control; one
// that is not would store a setting nothing reads.
var skipFlags = map[string]string{
	"claude":      "--dangerously-skip-permissions",
	"codex":       "--dangerously-bypass-approvals-and-sandbox",
	"antigravity": "--dangerously-skip-permissions",
	"opencode":    "--auto",
	"omp":         "--auto-approve",
	"crush":       "--yolo",
	"cursor":      "--force",
	"kiro":        "--trust-all-tools",
}

// loginCommands is how each provider's login is redone, for a plan reading
// that has none (PlanUsageSetting).
var loginCommands = map[string]string{
	"claude":      "claude",
	"codex":       "codex login",
	"antigravity": "agy",
	"opencode":    "opencode auth login",
	"cursor":      "cursor-agent login",
}

// The ways a provider honours the paste-unfold flag (pasteUnfoldTiming).
const (
	pasteNone uint8 = iota
	pasteNextPaste
	pasteNextSession
)

func pasteUnfoldTiming(kind string) uint8 {
	switch kind {
	case "claude", "kiro":
		return pasteNextPaste
	case "opencode":
		return pasteNextSession
	}
	return pasteNone
}

// supportsUltracode and supportsSubagentCards are terminal.SupportsUltracode
// and the lich-plugin mod: Claude Code is the only provider with either.
func supportsUltracode(kind string) bool     { return kind == "claude" }
func supportsSubagentCards(kind string) bool { return kind == "claude" }

func canResume(kind string) bool { return slices.Contains(resumableKinds, kind) }

func hasSkipFlag(kind string) bool {
	_, ok := skipFlags[kind]
	return ok
}

func hasPasteUnfold(kind string) bool { return pasteUnfoldTiming(kind) != pasteNone }

// providerKinds are every provider the window knows, terminals aside.
func providerKinds() []string {
	var kinds []string
	for kind := range providerIcons {
		if kind != "shell" {
			kinds = append(kinds, kind)
		}
	}
	slices.Sort(kinds)
	return kinds
}

func kindsWhere(has func(kind string) bool) []string {
	return slices.DeleteFunc(providerKinds(), func(kind string) bool { return !has(kind) })
}

// providerEntries are the Providers section's searchable rows, in the order
// its screens draw them. A perProvider row is the block of a provider's own
// screen: settings-index.ts lists it once per enabled provider. Its keys are
// providerBinEntries and the rows Settings keeps global.
func providerEntries() []settingEntry {
	return []settingEntry{
		{title: providerDefaultTitle, also: providerDefaultAlso, key: providerDefaultKey, scope: scopeOverridable},
		{title: profileTitle, also: claudeProfileProvider.words.searchAlso, key: claudeProfileKey, scope: scopeOverridable, perProvider: true, onlyFor: []string{claudeKind}},
		{title: profilesTitle, also: claudeProfileProvider.words.profilesAlso, perProvider: true, onlyFor: []string{claudeKind}},
		{title: profileTitle, also: codexProfileProvider.words.searchAlso, key: codexProfileKey, scope: scopeOverridable, perProvider: true, onlyFor: []string{codexKind}},
		{title: profilesTitle, also: codexProfileProvider.words.profilesAlso, perProvider: true, onlyFor: []string{codexKind}},
		{title: profileTitle, also: ompProfileProvider.words.searchAlso, key: ompProfileKey, scope: scopeOverridable, perProvider: true, onlyFor: []string{ompKind}},
		{title: profilesTitle, also: ompProfileProvider.words.profilesAlso, perProvider: true, onlyFor: []string{ompKind}},
		{title: planTitle, also: planAlso, perProvider: true},
		{title: binTitle, also: binAlso, perProvider: true},
		{title: skipTitle, also: skipAlso, perProvider: true, onlyFor: kindsWhere(hasSkipFlag)},
		{title: ultracodeTitle, also: ultracodeAlso, perProvider: true, onlyFor: kindsWhere(supportsUltracode)},
		{title: subagentTitle, also: subagentAlso, perProvider: true, onlyFor: kindsWhere(supportsSubagentCards)},
		{title: pasteTitle, also: pasteAlso, perProvider: true, onlyFor: kindsWhere(hasPasteUnfold)},
		{title: restoreTitle, also: restoreAlso, perProvider: true, onlyFor: kindsWhere(canResume)},
	}
}

// providerBinEntries are every provider's binary override, scoped on its own
// key like the sandbox rungs: read per project whether or not the provider is
// enabled, drawn on the provider's screen.
func providerBinEntries() []settingEntry {
	var entries []settingEntry
	for _, kind := range providerKinds() {
		entries = append(entries, settingEntry{key: providerBinKey(kind), scope: scopeOverridable})
	}
	return entries
}

// providerBinKey is providers-store.ts binKey (store.binKey): Claude keeps the
// key it had before providers could be chosen.
func providerBinKey(kind string) string {
	if kind == "claude" {
		return "claude.bin"
	}
	return "provider." + kind + ".bin"
}

// providerBinOffKey parks a binary layer without erasing it: the path stays,
// the layer stops resolving.
func providerBinOffKey(kind string) string { return providerBinKey(kind) + ".off" }

func providerSkipKey(kind string, worktree bool) string {
	if worktree {
		return "provider." + kind + ".skip-permissions.worktree"
	}
	return "provider." + kind + ".skip-permissions"
}

func providerUltracodeKey(kind string) string { return "provider." + kind + ".ultracode" }

func providerSubagentKey(kind string) string { return "provider." + kind + ".subagentCards" }

func providerPasteKey(kind string) string { return "provider." + kind + ".pasteUnfold" }

// providerGlobalKeys are the global values a provider's screen and row
// read; only the ones the provider has a control for.
func providerGlobalKeys(kind string) []string {
	keys := []string{providerEnabledKey(kind), providerBinKey(kind), providerBinOffKey(kind), restoreKey(kind)}
	if prov, ok := profileProviderOf(kind); ok {
		keys = append(keys, prov.key)
	}
	if hasSkipFlag(kind) {
		keys = append(keys, providerSkipKey(kind, false), providerSkipKey(kind, true))
	}
	if supportsUltracode(kind) {
		keys = append(keys, providerUltracodeKey(kind))
	}
	if supportsSubagentCards(kind) {
		keys = append(keys, providerSubagentKey(kind))
	}
	if hasPasteUnfold(kind) {
		keys = append(keys, providerPasteKey(kind))
	}
	return keys
}

// skipLevelOf folds the stored pair into a rung. The fourth combination, skip
// in the working tree and ask in worktrees, reads as everywhere: it is what
// that pair already did to the checkout the user works in.
func skipLevelOf(here, worktrees bool) string {
	switch {
	case here:
		return skipEverywhere
	case worktrees:
		return skipWorktrees
	}
	return skipNever
}

// skipLevelPair is the pair a rung writes: both keys, always, so the one that
// did not change never holds an answer to a question nobody is asked.
func skipLevelPair(level string) (here, worktrees bool) {
	return level == skipEverywhere, level != skipNever
}

// skipClimbs is climbsToRiskier over the ladder: only a move toward more
// exposure is confirmed, so taking the automation away is never the harder
// direction. Re-choosing the rung is no move.
func skipClimbs(current, next string) bool {
	rank := func(v string) int {
		return slices.IndexFunc(skipRungs, func(r struct{ value, label, consequence string }) bool { return r.value == v })
	}
	return rank(next) > rank(current)
}

func skipRung(value string) (label, consequence string) {
	for _, r := range skipRungs {
		if r.value == value {
			return r.label, r.consequence
		}
	}
	return value, ""
}

// binLayer is a configurable layer as the screen holds it: the path written
// into it and whether its switch parks it.
type binLayer struct {
	bin string
	off bool
}

// resolves reports a layer a session could spawn from. A parked layer never
// is, whatever it holds.
func (l binLayer) resolves() bool { return !l.off && strings.TrimSpace(l.bin) != "" }

func (l binLayer) parked() bool { return l.off && strings.TrimSpace(l.bin) != "" }

// winningBinScope is binary-layers.ts winningScope: the first layer that
// resolves, project then global, else $PATH.
func winningBinScope(project, global binLayer) string {
	switch {
	case project.resolves():
		return binScopeProject
	case global.resolves():
		return binScopeGlobal
	}
	return binScopePath
}

// configuredBin is the path the winning layer holds, "" when $PATH wins:
// whatever a parked layer still holds is not what spawns.
func configuredBin(scope string, project, global binLayer) string {
	switch scope {
	case binScopeProject:
		return strings.TrimSpace(project.bin)
	case binScopeGlobal:
		return strings.TrimSpace(global.bin)
	}
	return ""
}

// parkedLabel names the overrides that are set but switched off, the fact
// a screen with no switches would otherwise hide. A layer is named only when
// switching it back on would change what spawns.
func parkedLabel(scope string, project, global binLayer, projectName string) string {
	var names []string
	if scope != binScopeProject && project.parked() {
		if projectName == "" {
			projectName = binParkedFallbck
		}
		names = append(names, fmt.Sprintf(binParkedProject, projectName))
	}
	if scope == binScopePath && global.parked() {
		names = append(names, binParkedGlobal)
	}
	var out strings.Builder
	for _, n := range names {
		out.WriteString(" · " + n)
	}
	return out.String()
}

// binCheckLabel is the verdict beside a path. A missing binary reads
// differently by what was asked for: a bare name was looked up on $PATH and a
// path was looked for where it was written.
func binCheckLabel(c lichclient.BinaryCheck, bin string) string {
	switch c.Status {
	case lichclient.CheckOK:
		return binExecutable
	case "not-found":
		if strings.ContainsAny(bin, `/\`) {
			return binNoSuchFile
		}
		return binNotOnPath
	case "not-executable":
		return binNotExecutable
	case "home-shortcut":
		return binHomeShortcut
	case "relative":
		return binRelativePath
	}
	return ""
}

// binCheckDetail is the sentence under a failing binary: what will happen and
// what to do about it.
func binCheckDetail(c lichclient.BinaryCheck) string {
	switch c.Status {
	case "home-shortcut":
		return binDetailHome
	case "relative":
		return binDetailRel
	case "not-found", "not-executable":
		return binDetailBroken
	}
	return ""
}

// installSummary is what an installed provider's row says about where its
// binary came from: a configured path is the one that answers when `which`
// does not.
func installSummary(p providerInfo) string {
	if p.source == "setting" {
		return providersCustomPath
	}
	return providersInstalled
}

// planSummary is the plan reading in the width a list row has, "" for a
// provider lich reads no plan from and while the first reading is in flight.
// A signed-out reading is words: the row is where a lapsed login shows.
func planSummary(plan lichclient.QuotaPlan, ok bool) string {
	if !ok {
		return ""
	}
	if plan.Status == "signed-out" {
		return providersSignedOut
	}
	if plan.Status != "ok" {
		return ""
	}
	w, found := hottestWindow(plan)
	if !found {
		return ""
	}
	if length := formatWindow(w.Seconds); length != "" {
		return fmt.Sprintf(providersWindowPct, w.Percent, length)
	}
	return fmt.Sprintf(providersPercentUsed, w.Percent)
}

// providerTags are the facts a list row states without being opened: this is
// the project's own default, and nothing asks in any checkout.
func providerTags(kind, projectDefault, skip string) []string {
	var tags []string
	if projectDefault == kind {
		tags = append(tags, providerTagDefault)
	}
	if skip == skipEverywhere {
		tags = append(tags, providerTagNoPrompts)
	}
	return tags
}

// providerState is one provider row's widgets.
type providerState struct {
	open   widget.Clickable
	toggle ui.SwitchState
	docs   widget.Clickable
}

// providerSettings is the Providers pane's state: the verdicts it has asked
// for, the screen being looked at, and the pane's controls. The providers
// themselves are the window's book.
type providerSettings struct {
	book *providerBook

	mu sync.Mutex
	// projectOff are the bin-off flags of the project being looked at,
	// "<project>\x00<provider>" to the stored value, read once per visit.
	projectOff map[string]string
	checks     map[string]lichclient.BinaryCheck // a binary's verdict, by what was asked
	asked      map[string]bool
	askedOff   map[string]bool
	rechecking bool
	recheckErr string
	// seq numbers the writes to a key, so only the last of a burst is sent.
	seq  map[string]int
	send sync.Mutex

	// open is the provider whose own screen is showing, "" for the list.
	open string
	// adding is the Add provider list's fold, settled on its first draw.
	adding, addingSet bool
	// unwatch stops each plan reading the open pane follows, by quota key.
	unwatch map[string]func()

	addToggle, recheck, back, docs widget.Clickable
	rows                           map[string]*providerState
	picker                         ui.DropdownPanelState
	options                        map[string]*widget.Clickable

	bin binInput

	// claude, codex and omp are the profiles of the screens that have them.
	claude, codex, omp profileSettings

	skip, restore  widget.Enum
	ultra, agents  ui.SwitchState
	unfold, header ui.SwitchState

	// change is the rung climb the dialog asks about while asking; it stays
	// after an answer so the closing dialog keeps its text.
	change  skipChange
	asking  bool
	dialog  ui.DialogState
	cancel  widget.Clickable
	confirm widget.Clickable
}

// skipChange is a climb waiting on its confirmation.
type skipChange struct {
	provider providerInfo
	level    string
}

func (p *providerSettings) snapshot() providersMachine { return p.book.snapshot() }

func (p *providerSettings) state(id string) *providerState {
	if p.rows == nil {
		p.rows = map[string]*providerState{}
	}
	if p.rows[id] == nil {
		p.rows[id] = new(providerState)
	}
	return p.rows[id]
}

// revalidate has the verdicts asked again: Settings opening, or a re-read of
// $PATH, restates every one on screen.
func (p *providerSettings) revalidate() {
	p.mu.Lock()
	p.checks, p.asked, p.askedOff, p.projectOff = nil, nil, nil, nil
	p.mu.Unlock()
	p.addingSet = false
	p.claude.asked = false
}

// recheckPath is CheckAgainButton: re-read the login shell's $PATH, then read
// the providers again.
func (p *providerSettings) recheckPath(s *settingsScreen) {
	p.mu.Lock()
	p.rechecking, p.recheckErr = true, ""
	p.mu.Unlock()
	go func() {
		err := s.client.RefreshPath(s.ctx)
		if err == nil {
			err = p.book.reload(s.ctx)
		}
		p.mu.Lock()
		p.rechecking = false
		if err != nil {
			log.Printf("settings: re-check providers: %v", err)
			p.recheckErr = providersPathReread
		} else {
			p.checks, p.asked = nil, nil
		}
		p.mu.Unlock()
		s.invalidate()
	}()
}

// enqueue sends do for write id once, unless a later write to the same id
// came in first: typing a path writes on every keystroke, and the store must
// end on the last of them whatever order the calls land in.
func (p *providerSettings) enqueue(id string, do func()) {
	p.mu.Lock()
	if p.seq == nil {
		p.seq = map[string]int{}
	}
	p.seq[id]++
	n := p.seq[id]
	p.mu.Unlock()
	go func() {
		p.send.Lock()
		defer p.send.Unlock()
		p.mu.Lock()
		stale := p.seq[id] != n
		p.mu.Unlock()
		if !stale {
			do()
		}
	}()
}

// writeProviderKey stores value at key in scope ("" for Global), and in the
// book or the values the pane draws from so the change shows on the next
// frame. after runs once the backend has it.
func (s *settingsScreen) writeProviderKey(key, scope, value string, after func()) {
	switch {
	case scope == "":
		s.providers.book.setGlobal(key, value)
	case key == providerDefaultKey:
		s.providers.book.setProjectDefault(scope, value)
	default:
		s.mu.Lock()
		if value == "" {
			delete(s.values[key], scope)
		} else {
			if s.values == nil {
				s.values = scopedValues{}
			}
			if s.values[key] == nil {
				s.values[key] = map[string]string{}
			}
			s.values[key][scope] = value
		}
		s.mu.Unlock()
	}
	s.providers.enqueue(key+"\x00"+scope, func() {
		if err := s.client.SetSetting(s.ctx, key, scope, value); err != nil {
			log.Printf("settings: save %s: %v", key, err)
			return
		}
		if after != nil {
			after()
		}
	})
}

// setProviderEnabled switches a provider on or off. Its sessions are left
// alone; the book tells every reader.
func (s *settingsScreen) setProviderEnabled(id string, on bool) {
	flag := "0"
	if on {
		flag = "1"
	}
	s.providers.book.setEnabled(id, on)
	s.writeProviderKey(providerEnabledKey(id), "", flag, s.providersChanged)
}

// providersChanged tells the book that something it reads was written.
func (s *settingsScreen) providersChanged() { s.providers.book.changed(s.ctx) }

// setProviderDefault stores the default of the open scope.
func (s *settingsScreen) setProviderDefault(id string) {
	s.writeProviderKey(providerDefaultKey, s.scope, id, s.providersChanged)
}

// watchPlans follows exactly the plan readings keys name, none to stop. They
// are the one store's, shared with the footer and the card menu.
func (s *settingsScreen) watchPlans(keys ...string) {
	if s.footer == nil {
		return
	}
	p := &s.providers
	for key, unwatch := range p.unwatch {
		if !slices.Contains(keys, key) {
			unwatch()
			delete(p.unwatch, key)
		}
	}
	for _, key := range keys {
		if p.unwatch == nil {
			p.unwatch = map[string]func(){}
		}
		if p.unwatch[key] == nil {
			p.unwatch[key] = s.footer.quota.watch(key)
		}
	}
}

// planKeys are the readings the open pane draws: the machine's own on the
// Providers pane, and the profile of the scope on the screen of a provider
// with profiles.
func (s *settingsScreen) planKeys(values scopedValues) []string {
	if !s.open || s.section != sectionProviders {
		return nil
	}
	keys := []string{""}
	m := s.providers.snapshot()
	if c := s.providers.profiles(s.providers.open); c != nil && m.loaded {
		if key := profileQuotaKey(c.provider, s.scopeProfile(c, values, m)); key != "" {
			keys = append(keys, key)
		}
	}
	return keys
}

// planOf is the machine's reading for kind, false for a provider lich reads no
// plan from and until the first reading lands.
func (s *settingsScreen) planOf(kind string) (lichclient.QuotaPlan, bool) {
	if s.footer == nil {
		return lichclient.QuotaPlan{}, false
	}
	return planFor(s.footer.quota.plans(""), kind)
}

// profiles is the profiles of the screen kind names, nil for a provider with
// none and until the window has bound its book.
func (p *providerSettings) profiles(kind string) *profileSettings {
	var c *profileSettings
	switch kind {
	case claudeKind:
		c = &p.claude
	case codexKind:
		c = &p.codex
	case ompKind:
		c = &p.omp
	}
	if c == nil || c.book == nil {
		return nil
	}
	return c
}
