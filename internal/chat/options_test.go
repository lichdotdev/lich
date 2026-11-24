//go:build !windows

package chat

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

func (r *rig) options() []api.ChatOption {
	r.t.Helper()
	opts, err := r.svc.Options(r.t.Context(), api.SessionIDOptions{ID: "s1"})
	if err != nil {
		r.t.Fatalf("Options: %v", err)
	}
	return opts
}

func (r *rig) option(category string) (api.ChatOption, bool) {
	for _, o := range r.options() {
		if o.Category == category {
			return o, true
		}
	}
	return api.ChatOption{}, false
}

func (r *rig) set(id, value string) error {
	_, err := r.svc.SetOption(r.t.Context(), api.ChatSetOptionOptions{ID: "s1", OptionID: id, Value: value})
	return err
}

func (r *rig) mustSet(id, value string) {
	r.t.Helper()
	if err := r.set(id, value); err != nil {
		r.t.Fatalf("SetOption(%s=%s): %v", id, value, err)
	}
}

// current is the choice in force of category, waiting for it to become want.
func (r *rig) current(category, want string) {
	r.t.Helper()
	r.eventually(category+" = "+want, func() bool {
		o, ok := r.option(category)
		return ok && o.Current == want
	})
}

func choiceIDs(o api.ChatOption) []string {
	ids := make([]string, 0, len(o.Choices))
	for _, ch := range o.Choices {
		ids = append(ids, ch.ID)
	}
	return ids
}

func (r *rig) optionEvents() int {
	n := 0
	for _, e := range r.snapshot() {
		if e.name == api.ChatOptionsEvent.Name {
			n++
		}
	}
	return n
}

// checkCurrents fails on an option whose current choice is not one it offers.
func checkCurrents(t *testing.T, opts []api.ChatOption) {
	t.Helper()
	for _, o := range opts {
		if !hasChoice(o, o.Current) {
			t.Errorf("option %s is on %q, which is none of %v", o.ID, o.Current, choiceIDs(o))
		}
	}
}

// TestClaudeOptionsSetInFlight: a Claude Code chat offers the models
// initialize lists, the efforts of the one it runs and the permission modes,
// and sets each over the protocol, recording the choice for later spawns.
func TestClaudeOptionsSetInFlight(t *testing.T) {
	r := newRig(t, providers.Claude)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	opts := r.options()
	checkCurrents(t, opts)
	model, _ := r.option(api.OptionModel)
	if model.Current != "default" || model.Choices[1].Label != "Opus 5.5" {
		t.Errorf("model option = %+v, want the default alias and the provider's names", model)
	}
	mode, _ := r.option(api.OptionMode)
	if mode.Current != "auto" || !slices.Contains(choiceIDs(mode), "plan") {
		t.Errorf("mode option = %+v", mode)
	}
	for _, ch := range mode.Choices {
		if ch.Danger != (ch.ID == claudeBypass) {
			t.Errorf("mode %s Danger = %v", ch.ID, ch.Danger)
		}
	}
	before := r.optionEvents()

	r.mustSet(claudeEffortOption, "low")
	r.current(api.OptionEffort, "low")
	if !strings.Contains(r.fakeLog(), `"effortLevel":"low"`) || r.db.SessionEffort("s1") != "low" {
		t.Errorf("effort not set over the protocol and recorded: %q", r.db.SessionEffort("s1"))
	}
	if r.optionEvents() == before {
		t.Error("no options event after a change")
	}

	r.mustSet(claudeModelOption, "haiku")
	r.current(api.OptionModel, "haiku")
	effort, _ := r.option(api.OptionEffort)
	if !slices.Equal(choiceIDs(effort), []string{"low", "medium"}) || r.db.SessionModel("s1") != "haiku" {
		t.Errorf("after the model change effort = %v, model recorded %q", choiceIDs(effort), r.db.SessionModel("s1"))
	}

	r.mustSet(claudeModeOption, "plan")
	r.current(api.OptionMode, "plan")
	if r.db.SessionChatMode("s1") != "plan" {
		t.Errorf("chat mode recorded = %q, want plan", r.db.SessionChatMode("s1"))
	}
	if mode, _ := r.option(api.OptionMode); slices.Contains(choiceIDs(mode), claudeAuto) {
		t.Errorf("haiku offers auto: %v", choiceIDs(mode))
	}
	if err := r.set(claudeModeOption, "dontAsk"); err == nil {
		t.Error("a mode the provider refused was accepted")
	}
	if err := r.set(claudeModeOption, "sideways"); err == nil {
		t.Error("a mode the chat does not offer was accepted")
	}
	if err := r.set("colour", "red"); err == nil {
		t.Error("an option the chat does not have was accepted")
	}

	r.mustSet(claudeModelOption, "old")
	if _, ok := r.option(api.OptionEffort); ok {
		t.Error("a model with no effort levels still offers an effort")
	}
	r.mustSet(claudeModeOption, "default")
	r.prompt("plan")
	r.turnEnd(0)
	r.current(api.OptionMode, "plan")
	checkCurrents(t, r.options())
}

// TestClaudeBypassRelaunches: bypassPermissions, which the protocol refuses
// to a process not launched for it, resumes the conversation in a process
// that is, carrying the model and effort the chat ran on, and is refused while
// a turn runs.
func TestClaudeBypassRelaunches(t *testing.T) {
	r := newRig(t, providers.Claude)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	r.prompt("hi")
	r.turnEnd(0)
	r.writeClaudeTranscript("claude-new", `{"type":"user","message":{"content":"hi"}}`)
	r.mustSet(claudeModelOption, "haiku")
	r.mustSet(claudeEffortOption, "low")

	r.term.ReportChatState("s1", stateBusy, "", "")
	if err := r.set(claudeModeOption, claudeBypass); err == nil {
		t.Error("a relaunch during a turn was accepted")
	}
	r.term.ReportChatState("s1", stateDone, "", "")

	first := r.svc.running("s1")
	r.mustSet(claudeModeOption, claudeBypass)
	if r.svc.running("s1") == first {
		t.Fatal("the chat was not relaunched")
	}
	if log := r.fakeLog(); !strings.Contains(log, "--allow-dangerously-skip-permissions") || !strings.Contains(log, "--resume claude-new") {
		t.Errorf("relaunch argv lacks the bypass flag or the conversation: %q", log)
	}
	if _, lost := r.item(api.ItemNotice, "did not come back"); lost {
		t.Error("the relaunch lost the conversation")
	}
	r.current(api.OptionMode, claudeBypass)
	r.current(api.OptionModel, "haiku")
	r.current(api.OptionEffort, "low")
	if r.db.SessionChatMode("s1") != claudeBypass {
		t.Errorf("chat mode recorded = %q", r.db.SessionChatMode("s1"))
	}
	viewed := false
	for _, e := range r.snapshot() {
		if v, ok := e.data.(api.SessionView); ok && v.View == api.ViewChat && e.name == api.SessionViewEvent.Name {
			viewed = true
		}
	}
	if !viewed {
		t.Error("the window was not told the chat came back")
	}
	r.mustSet(claudeModeOption, "default")
	if r.svc.running("s1") == nil {
		t.Fatal("the chat stopped")
	}
}

// TestChatOpensOnItsStoredMode: a chat comes back on the mode the person
// picked in it, launched for bypassPermissions when that was it, and says so
// when the provider refuses it.
func TestChatOpensOnItsStoredMode(t *testing.T) {
	for _, tc := range []struct{ mode, want string }{
		{"plan", "plan"}, {claudeBypass, claudeBypass}, {"dontAsk", "auto"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			r := newRig(t, providers.Claude)
			_ = r.db.SetSessionChatMode("s1", tc.mode)
			if err := r.setView(api.ViewChat); err != nil {
				t.Fatalf("SetView: %v", err)
			}
			r.current(api.OptionMode, tc.want)
			if tc.mode == claudeBypass && !strings.Contains(r.fakeLog(), "--allow-dangerously-skip-permissions") {
				t.Errorf("argv lacks the bypass flag: %q", r.fakeLog())
			}
			if tc.want != tc.mode {
				r.mustItem(api.ItemNotice, "was not set again")
			}
		})
	}
}

// TestChatOpensOnItsStoredModelAndEffort: a chat comes back on the model and
// effort the person last picked in it. Claude Code resumes the model with the
// conversation but takes the effort from its settings.json, so a chat set to
// medium came back on high after lich restarted.
func TestChatOpensOnItsStoredModelAndEffort(t *testing.T) {
	for _, tc := range []struct{ kind, model, effort string }{
		{providers.Claude, "haiku", "low"},
		{providers.Codex, "gpt-a", "low"},
		{providers.OMP, "m1", "low"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			r := newRig(t, tc.kind)
			_ = r.db.SetSessionModel("s1", tc.model)
			_ = r.db.SetSessionEffort("s1", tc.effort)
			if err := r.setView(api.ViewChat); err != nil {
				t.Fatalf("SetView: %v", err)
			}
			r.current(api.OptionModel, tc.model)
			r.current(api.OptionEffort, tc.effort)
		})
	}
}

// TestAnEffortTheProviderMovesIsRecorded: an effort the chat moves to without
// lich's picker, a /effort typed in it, is recorded for its next open, on
// each way a provider says so: Codex in thread/settings/updated, an ACP server
// in config_option_update, and Claude Code on no line at all, so lich asks
// get_settings again once one of its commands has printed.
func TestAnEffortTheProviderMovesIsRecorded(t *testing.T) {
	for _, kind := range []string{providers.Claude, providers.Codex, providers.OMP} {
		t.Run(kind, func(t *testing.T) {
			r := newRig(t, kind)
			if err := r.setView(api.ViewChat); err != nil {
				t.Fatalf("SetView: %v", err)
			}
			r.prompt("/effort low")
			r.current(api.OptionEffort, "low")
			r.eventually("the effort recorded", func() bool { return r.db.SessionEffort("s1") == "low" })
		})
	}
}

// TestACPConfigOptions: an ACP chat offers the config options of the three
// categories, follows the whole list a server sends back, and sets each over
// session/set_config_option.
func TestACPConfigOptions(t *testing.T) {
	r := newRig(t, providers.OMP)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	opts := r.options()
	checkCurrents(t, opts)
	if len(opts) != 3 {
		t.Errorf("options = %+v, want mode, model and thinking", opts)
	}
	if o, _ := r.option(api.OptionModel); o.Label != "Model" || o.Choices[0].Label != "M1" {
		t.Errorf("model option = %+v, want the server's words", o)
	}

	r.mustSet("model", "m2")
	r.current(api.OptionModel, "m2")
	if _, ok := r.option(api.OptionEffort); ok {
		t.Error("the effort of the old model is still offered")
	}
	if r.db.SessionModel("s1") != "m2" {
		t.Errorf("model recorded = %q", r.db.SessionModel("s1"))
	}
	r.mustSet("model", "m1")
	r.mustSet("thinking", "low")
	r.current(api.OptionEffort, "low")
	r.mustSet("mode", "plan")
	r.current(api.OptionMode, "plan")
	if r.db.SessionEffort("s1") != "low" || r.db.SessionChatMode("s1") != "plan" {
		t.Errorf("recorded effort %q, mode %q", r.db.SessionEffort("s1"), r.db.SessionChatMode("s1"))
	}
	r.mustSet("mode", "default")
	r.prompt("mode")
	r.turnEnd(0)
	r.current(api.OptionMode, "plan")
}

// TestACPLegacyModesAndModels: a server with only the older modes and models
// is offered them, set over session/set_mode and session/set_model, and its
// mode follows current_mode_update.
func TestACPLegacyModesAndModels(t *testing.T) {
	t.Setenv(fakeLegacyEnv, "1")
	r := newRig(t, providers.OMP)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	checkCurrents(t, r.options())
	if o, _ := r.option(api.OptionMode); o.ID != acpModesOption || o.Choices[1].Label != "planner" {
		t.Errorf("mode option = %+v", o)
	}
	r.mustSet(acpModesOption, "plan")
	r.mustSet(acpModelsOption, "m2")
	r.current(api.OptionModel, "m2")
	if log := r.fakeLog(); !strings.Contains(log, `"session/set_mode"`) || !strings.Contains(log, `"session/set_model"`) {
		t.Errorf("fake log = %q, want set_mode and set_model", log)
	}
	r.mustSet(acpModesOption, "default")
	r.prompt("mode")
	r.turnEnd(0)
	r.current(api.OptionMode, "plan")
}

// TestKiroModelRelaunches: Kiro's model, which its set_model never applies,
// is set by resuming in a process started with --model, the mode carried.
func TestKiroModelRelaunches(t *testing.T) {
	t.Setenv(fakeLegacyEnv, "1")
	r := newRig(t, providers.Kiro)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	dir := filepath.Join(r.home, ".kiro", "sessions", "cli")
	_ = os.MkdirAll(dir, 0o755)
	if err := os.WriteFile(filepath.Join(dir, "ses-new.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	r.mustSet(acpModesOption, "plan")
	r.mustSet(acpModelsOption, "m2")
	r.current(api.OptionModel, "m2")
	r.current(api.OptionMode, "plan")
	log := r.fakeLog()
	if !strings.Contains(log, "--model m2") || strings.Contains(log, `"session/set_model"`) {
		t.Errorf("fake log = %q, want a relaunch with --model and no set_model", log)
	}
	if !strings.Contains(log, `"session/load","params":{"cwd"`) || !strings.Contains(log, `"sessionId":"ses-new"}`) {
		t.Errorf("fake log = %q, want the relaunch to load the conversation", log)
	}
	for _, lost := range []string{"did not come back", "came back empty"} {
		if _, ok := r.item(api.ItemNotice, lost); ok {
			t.Errorf("the relaunch lost the conversation: %q", lost)
		}
	}
}

// TestClaudeSkipPermissionsOpensOnBypass: a checkout that skips permissions
// starts the chat on bypassPermissions, and leaving it and coming back needs
// no new process.
func TestClaudeSkipPermissionsOpensOnBypass(t *testing.T) {
	r := newRig(t, providers.Claude)
	_, _ = r.db.SetSetting(t.Context(), api.SetSettingOptions{Key: "provider.claude.skip-permissions", Value: "true"})
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	r.current(api.OptionMode, claudeBypass)
	first := r.svc.running("s1")
	r.mustSet(claudeModeOption, "default")
	r.mustSet(claudeModeOption, claudeBypass)
	r.current(api.OptionMode, claudeBypass)
	if r.svc.running("s1") != first {
		t.Error("a process launched skipping permissions was relaunched for bypassPermissions")
	}
}
