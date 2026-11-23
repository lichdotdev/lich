package agentplugin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/relay"
	"github.com/lichdotdev/lich/internal/semver"
	"github.com/lichdotdev/lich/internal/shquote"
)

// The Crush side. Crush has no plugin system at all: its hooks are entries in
// the user's own config, and the scripts they name have to already be somewhere
// on disk. So an install here is two writes — the scripts into lich's own
// directory, and a block of `hook add` lines into Crush's `crushrc`.
//
// `crushrc` rather than `crush.json` because it is a script, not a document:
// lich appends its own lines and removes them by their markers, leaving every
// other line — and every comment — exactly as the user wrote it. Rewriting the
// JSON would reformat a file lich does not own.
//
// What is registered depends on the Crush installed: below crushEventsVersion
// `PreToolUse` is the only event it has, so only the session id and the touched
// report ride it; from there on the state reports ride the lifecycle events too
// (docs/hooks/session-state.md, Crush).

const (
	// crushMinVersion is the first Crush release whose crushrc understands
	// `hook add` (v0.88.0). Older ones read the file, ignore the lines and say
	// nothing — the install would look done and report nothing forever — so the
	// version is checked before anything is written.
	crushMinVersion = "0.88.0"

	// crushEventsVersion is the first Crush release with the lifecycle events
	// (v0.99.0, read off its source; measured on v0.100.0). The block written
	// for an older one stays as it was.
	crushEventsVersion = "0.99.0"

	// crushHookEvent is the event a hook rides when it names none, and the only
	// one a Crush below crushEventsVersion has.
	crushHookEvent = "PreToolUse"
	// crushTurnEndEvent is the event whose registration says a block reports
	// state (blockReportsState).
	crushTurnEndEvent = "TurnEnd"
	// crushWriteTools matches the tools that change files, anchored because
	// Crush's tool names are whole lower-case words.
	crushWriteTools = "^(edit|write|multiedit|bash)$"

	// crushHookTimeout and crushMCPTimeout are the seconds Crush's own flags
	// take. A hook is one POST to lich's loopback listener, so five is already
	// far past a run that will ever succeed; the MCP registration has a binary to
	// spawn and a handshake to finish before Crush can use a tool, so it is given
	// longer. Both bound a session's own turn — a hung one is what they exist to
	// end.
	crushHookTimeout = 5
	crushMCPTimeout  = 10
	// crushVetoTimeout is longer: the veto waits on lich's answer, which may
	// open a session first (docs/hooks/subagent-cards.md, Registration).
	crushVetoTimeout = 15

	// shComment is how the marker lines hide from the shell that runs crushrc.
	shComment = "#"
	// blockOpen and blockClose delimit lich's lines inside crushrc, so an update
	// replaces exactly what a previous install wrote.
	blockOpen  = shComment + " >>> " + markerName + " v%s >>>"
	blockClose = shComment + " <<< " + markerName + " <<<"
)

// crushHooks is what a Crush below crushEventsVersion registers, one entry per
// report it can honour: the script from the plugin repository, the argument it
// takes, the name the hook is stored under, and the tools it matches ("" for
// every tool).
var crushHooks = []crushHook{
	{script: "hooks/report-session-start.sh", arg: providers.Crush, name: "lich-session-id", timeout: crushHookTimeout},
	{script: "hooks/report-touched.sh", name: "lich-touched", matcher: crushWriteTools, timeout: crushHookTimeout},
}

// crushStateHooks is what a Crush with crushEventsVersion registers. The
// session id rides TurnStart as well as SessionStart because a session resumed
// with --session or picked in Crush's own list fires no SessionStart (measured
// on v0.100.0). PermissionResult, Interrupt and the compaction pair are left
// out on purpose (docs/hooks/session-state.md, Crush).
var crushStateHooks = []crushHook{
	{event: "SessionStart", script: crushSessionStartScript, arg: providers.Crush, name: "lich-session-id", timeout: crushHookTimeout},
	{event: "TurnStart", script: crushSessionStartScript, arg: providers.Crush, name: "lich-session-id-turn", timeout: crushHookTimeout},
	{event: "TurnStart", script: crushStateScript, arg: "busy", name: "lich-turn-start", timeout: crushHookTimeout},
	{script: "hooks/report-tool.sh", name: "lich-tool", timeout: crushHookTimeout},
	{script: "hooks/report-touched.sh", name: "lich-touched", matcher: crushWriteTools, timeout: crushHookTimeout},
	{event: "PostToolUse", script: crushStateScript, arg: "busy", name: "lich-tool-done", timeout: crushHookTimeout},
	{event: "PostToolUseFailure", script: crushStateScript, arg: "busy", name: "lich-tool-failed", timeout: crushHookTimeout},
	{event: "PermissionRequest", script: crushStateScript, arg: "waiting", name: "lich-waiting", timeout: crushHookTimeout},
	{event: crushTurnEndEvent, script: crushStateScript, arg: "done", name: "lich-turn-end", timeout: crushHookTimeout},
	{event: "StopFailure", script: crushStateScript, arg: "done", name: "lich-turn-failed", timeout: crushHookTimeout},
	{event: "SessionEnd", script: crushStateScript, arg: "idle", name: "lich-session-end", timeout: crushHookTimeout},
}

const (
	crushSessionStartScript = "hooks/report-session-start.sh"
	crushStateScript        = "hooks/report-state.sh"
)

type crushHook struct {
	// event is crushHookEvent when empty.
	event   string
	script  string
	arg     string
	name    string
	matcher string
	timeout int
}

// crushHooksFor is what version's crushrc registers on crush, the Crush release
// ("" when unread): the reports that Crush has events for, and from
// SubagentHookRelease the veto on Crush's one subagent tool.
func crushHooksFor(version, crush string) []crushHook {
	hooks := crushHooks
	if crushHasEvents(crush) {
		hooks = crushStateHooks
	}
	if !asksSubagentHook(version) {
		return hooks
	}
	return append(slices.Clone(hooks), crushHook{script: vetoScript, arg: providers.Crush, name: "lich-subagent-cards", matcher: "^agent$", timeout: crushVetoTimeout})
}

// crushSourcedScripts are the files the registered scripts source rather than
// run: no hook names them, but report-session-start reports nothing without
// them beside it, and the state reports read a tool's detail from detail.jq.
var crushSourcedScripts = []string{"hooks/conversation-id.sh", "hooks/detail.jq"}

func (s *Service) crushInstall() error {
	crush, err := s.crushSupportsHooks()
	if err != nil {
		return err
	}
	version, err := s.releaseVersion()
	if err != nil {
		return err
	}
	dir, err := pluginScriptDir(providers.Crush)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	scripts := slices.Clone(crushSourcedScripts)
	for _, hook := range crushHooksFor(version, crush) {
		if !slices.Contains(scripts, hook.script) {
			scripts = append(scripts, hook.script)
		}
	}
	for _, script := range scripts {
		data, err := s.fetchFile(version, script)
		if err != nil {
			return err
		}
		// Executable: Crush dispatches a shebang'd script through os/exec, which
		// needs the bit the way any other shell would.
		if err := writeFile(filepath.Join(dir, filepath.Base(script)), data, 0o755); err != nil {
			return err
		}
	}
	skills, err := s.crushSkillsDir()
	if err != nil {
		return err
	}
	if err := s.installSkills(version, skills); err != nil {
		return err
	}
	return s.writeCrushrc(version, crush, dir, s.lichBin())
}

// writeCrushrc replaces lich's block in Crush's crushrc, creating the file when
// it is not there yet. Everything outside the markers is carried over verbatim.
func (s *Service) writeCrushrc(version, crush, scriptDir, lichBin string) error {
	path, err := s.crushrcPath()
	if err != nil {
		return err
	}
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	body := replaceBlock(string(existing), crushrcBlock(version, crush, scriptDir, lichBin))
	return writeFile(path, []byte(body), 0o644)
}

// crushrcBlock is the block lich owns: its markers, a line saying who wrote it,
// one `hook add` per report, and the `mcp add` that gives the session the tools
// to drive the others.
func crushrcBlock(version, crush, scriptDir, lichBin string) string {
	var b strings.Builder
	fmt.Fprintf(&b, blockOpen+"\n", version)
	fmt.Fprintf(&b, "%s Managed by lich. Edit through Settings, not by hand.\n", shComment)
	for _, hook := range crushHooksFor(version, crush) {
		// Two layers of quoting, and both are load-bearing: the inner one makes
		// the script path survive as one word inside the command Crush runs, the
		// outer one makes that whole command survive as one argument to the
		// crushrc builtin reading this line.
		command := shquote.Quote(filepath.Join(scriptDir, filepath.Base(hook.script)))
		if hook.arg != "" {
			command += " " + hook.arg
		}
		event := hook.event
		if event == "" {
			event = crushHookEvent
		}
		fmt.Fprintf(&b, "hook add %s --name %s", event, hook.name)
		if hook.matcher != "" {
			fmt.Fprintf(&b, " --matcher %s", shquote.Quote(hook.matcher))
		}
		fmt.Fprintf(&b, " --command %s --timeout %d\n", shquote.Quote(command), hook.timeout)
	}
	if lichBin != "" {
		// The operations Claude Code and Codex are handed on their own command
		// line at spawn (docs/cli.md, Registration). Crush takes no such flag,
		// and this is the only other place it reads a server from — the same
		// file its hooks come from, under the same markers, so an update
		// replaces it and an uninstall would take it away with them.
		//
		// The path is this binary's, resolved now: the registration is a line in
		// a file, so it cannot name $LICH_BIN and have Crush expand it per
		// session. It is only the transport — which lich a session talks to is
		// decided by the coordinates in its PTY, not by which binary runs.
		fmt.Fprintf(&b, "mcp add %s --type stdio --command %s --args %s --timeout %d\n",
			relay.MCPServerName, shquote.Quote(lichBin), relay.MCPSubcommand, crushMCPTimeout)
	}
	b.WriteString(blockClose + "\n")
	return b.String()
}

// lichBinary is the lich an install writes into a harness's config — Crush's
// crushrc, omp's mcp.json, Cursor's. An unresolvable executable leaves the
// registration out rather than writing a command that cannot run: for Crush and
// omp the hooks are the part that makes them useful and they do not depend on
// it, and Cursor's install refuses outright, since its registration is all lich
// writes there.
//
// A lich started with `go run` cannot register itself, and that is the case
// worth naming: os.Executable answers with the binary the Go toolchain built
// into its cache, which is deleted when that run ends. Writing it produces a
// registration that works for the rest of that session and then fails silently
// forever — the exact shape a `task dev` install left in a real
// ~/.cursor/mcp.json.
//
// So a dev build registers the installed lich on PATH instead of nothing.
// Nothing would have been the safe answer for a file, and the wrong one for the
// person writing it: `task dev` is how this project is worked on, and an install
// that refuses there is an install nobody can test. It is also not a compromise
// — the registration is only the transport, and which lich a session reaches is
// decided by the coordinates in its PTY, never by which binary the command
// names. Any lich that starts is the right one.
func lichBinary() string {
	exe, err := os.Executable()
	return resolveLichBinary(exe, err, exec.LookPath)
}

// resolveLichBinary is the decision behind lichBinary, with both answers it
// depends on passed in: what this process is, and what PATH holds. Split out so
// the `go run` branch is reachable from a test — os.Executable cannot be made to
// answer with a cache path from inside one.
func resolveLichBinary(exe string, exeErr error, lookPath func(string) (string, error)) string {
	if exeErr == nil && exe != "" && !underGoBuildCache(exe) {
		return exe
	}
	installed, err := lookPath(lichCommand)
	if err != nil {
		return ""
	}
	return installed
}

// lichCommand is lich's own name on PATH, for the fallback above.
const lichCommand = "lich"

// underGoBuildCache reports whether path is the throwaway binary `go run` built.
// It is matched on shape — `.../go-build<digits>/b<digits>/exe/<name>` — because
// there is nothing to ask: the toolchain exports no marker, and the temporary
// root moves with TMPDIR. The layout has been stable for the whole life of the
// build cache, and the cost of the match being wrong is a registration lich
// declines to write and says so.
func underGoBuildCache(path string) bool {
	dir, file := filepath.Split(filepath.Clean(path))
	if file == "" {
		return false
	}
	dir = filepath.Clean(dir)
	if filepath.Base(dir) != "exe" {
		return false
	}
	for dir = filepath.Dir(dir); dir != "" && dir != string(filepath.Separator); dir = filepath.Dir(dir) {
		if strings.HasPrefix(filepath.Base(dir), "go-build") {
			return true
		}
		if parent := filepath.Dir(dir); parent == dir {
			break
		}
	}
	return false
}

// replaceBlock returns existing with lich's block replaced by block — appended
// when there is none, and left where it was when there is. A file whose opening
// marker lost its close is treated as having no block: appending is recoverable,
// eating the rest of the file is not.
func replaceBlock(existing, block string) string {
	start := indexOfOpenMarker(existing)
	if start < 0 {
		return appendBlock(existing, block)
	}
	end := strings.Index(existing[start:], blockClose)
	if end < 0 {
		return appendBlock(existing, block)
	}
	end += start + len(blockClose)
	// The block always ends in a newline, so the one that followed the old close
	// would otherwise become a blank line on every reinstall.
	rest := strings.TrimPrefix(existing[end:], "\n")
	return existing[:start] + block + rest
}

// indexOfOpenMarker finds lich's opening marker whatever version it names.
func indexOfOpenMarker(existing string) int {
	prefix, _, _ := strings.Cut(blockOpen, "%s")
	return strings.Index(existing, prefix)
}

// appendBlock puts the block at the end, on its own line and after a blank one
// when the file already has content.
func appendBlock(existing, block string) string {
	if strings.TrimSpace(existing) == "" {
		return block
	}
	return strings.TrimRight(existing, "\n") + "\n\n" + block
}

// crushInstalledVersion reads the version off the block lich wrote into
// crushrc, or ("", false) when there is none.
func (s *Service) crushInstalledVersion() (string, bool) {
	path, err := s.crushrcPath()
	if err != nil {
		return "", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return versionOf(data, shComment+" >>>")
}

// crushReportsState reports whether lich's block in crushrc registers the state
// reports, which only one written for a Crush with crushEventsVersion does.
func (s *Service) crushReportsState() bool {
	path, err := s.crushrcPath()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(path)
	return err == nil && blockReportsState(string(data))
}

// crushBlockBehind reports whether the Crush installed now has the lifecycle
// events and lich's block registers none of them: a Crush upgraded under an
// install that was current for the old one, which only writing the block again
// moves.
func (s *Service) crushBlockBehind() bool {
	crush, err := s.crushSupportsHooks()
	return err == nil && crushHasEvents(crush) && !s.crushReportsState()
}

// crushHasEvents is whether a Crush release ("" when unread) has the lifecycle
// events.
func crushHasEvents(crush string) bool {
	return crush != "" && !semver.Less(crush, crushEventsVersion)
}

// blockReportsState is crushReportsState's reading of one crushrc: a turn-end
// registration inside lich's markers. A user's own TurnEnd hook outside them is
// not lich's report.
func blockReportsState(rc string) bool {
	start := indexOfOpenMarker(rc)
	if start < 0 {
		return false
	}
	block, _, _ := strings.Cut(rc[start:], blockClose)
	return strings.Contains(block, "hook add "+crushTurnEndEvent+" ")
}

// crushSupportsHooks refuses an install onto a Crush too old to read the lines
// it would write, and returns its version. A version lich cannot read at all is
// allowed through as "": a refusal there would be lich guessing, and the
// failure it prevents is one the user can see and undo.
func (s *Service) crushSupportsHooks() (string, error) {
	out, err := s.read(providers.Crush, "--version")
	if err != nil {
		return "", fmt.Errorf("crush --version: %w: %s", err, strings.TrimSpace(out))
	}
	version := parseCrushVersion(out)
	if version == "" || !semver.Less(version, crushMinVersion) {
		return version, nil
	}
	return "", fmt.Errorf(
		"Crush %s cannot load hooks from crushrc — update to %s or newer",
		version, crushMinVersion)
}

// parseCrushVersion pulls the bare version out of `crush --version`, whose
// output is "crush version v0.88.1". Empty when the shape is not that.
func parseCrushVersion(out string) string {
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) == 0 {
		return ""
	}
	version := strings.TrimPrefix(fields[len(fields)-1], "v")
	if version == "" || version[0] < '0' || version[0] > '9' {
		return ""
	}
	return version
}

// crushrcPath resolves the global crushrc: the sibling of Crush's global
// config, in the directory `crush dirs` prints first. Asking the CLI rather
// than rebuilding its path rule keeps lich right when that rule differs per OS
// or moves.
func (s *Service) crushrcPath() (string, error) {
	out, err := s.read(providers.Crush, "dirs")
	if err != nil {
		return "", fmt.Errorf("crush dirs: %w: %s", err, strings.TrimSpace(out))
	}
	dir, _, _ := strings.Cut(strings.TrimSpace(out), "\n")
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return "", fmt.Errorf("crush dirs printed no config directory")
	}
	return filepath.Join(dir, "crushrc"), nil
}

// crushSkillsDir is the global skills directory Crush reads: $CRUSH_SKILLS_DIR
// when set, since Crush then reads that one alone, else `skills` beside the
// crushrc (Crush's GlobalSkillsDirs, v0.98.1).
func (s *Service) crushSkillsDir() (string, error) {
	if dir := os.Getenv("CRUSH_SKILLS_DIR"); dir != "" {
		return dir, nil
	}
	rc, err := s.crushrcPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(rc), "skills"), nil
}

// pluginScriptDir is where lich keeps the hook scripts it installs for one
// file-shipped harness: its own config directory, not the harness's. They are
// lich's copy of another repository's files, and a harness that never installed
// them should not be carrying them.
//
// One directory per provider, even though Crush and Kiro ship the same scripts.
// releaseVersion resolves the newest release compatible with this lich, not a
// version fixed by it, so a shared directory would let a
// Kiro install fetched today overwrite the scripts a Crush install fetched at an
// older release — leaving Crush's marker naming a version that is no longer on
// disk, which is the one thing that marker exists to answer.
func pluginScriptDir(provider string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config directory: %w", err)
	}
	return filepath.Join(dir, "lich", "plugin", "hooks", provider), nil
}
