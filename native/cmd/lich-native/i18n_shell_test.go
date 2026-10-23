package main

import (
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
)

// inPortuguese puts the interface in pt-BR for the test and puts English, and
// the labels tables built from it, back after.
func inPortuguese(t *testing.T) {
	t.Helper()
	i18n.SetCurrent(i18n.PtBR)
	t.Cleanup(func() {
		i18n.SetCurrent(i18n.EN)
		localizeHotkeys()
	})
}

func TestTrayLabelsAreInTheInterfaceLanguageAndKeepTheCountPlaceholder(t *testing.T) {
	if got := trayLabels(); got.Show != "Show lich" || got.Running != "Sessions running: {count}" {
		t.Errorf("English labels: %+v", got)
	}
	inPortuguese(t)
	got := trayLabels()
	if got.Show != "Mostrar o lich" || got.Running != "Sessões rodando: {count}" {
		t.Errorf("pt-BR labels: %+v", got)
	}
}

func TestTheTrayHearsOfALanguageChangeOnTheNextFrame(t *testing.T) {
	u, b := newGateWindow(t)
	u.attachTray()
	if args := b.await(t, "tray.SetLabels"); !strings.Contains(args, "Show lich") {
		t.Fatalf("first labels %s", args)
	}
	u.followLanguage()
	select {
	case call := <-b.calls:
		t.Fatalf("a frame in the same language called %s", call)
	case <-time.After(100 * time.Millisecond):
	}

	inPortuguese(t)
	u.followLanguage()
	if args := b.await(t, "tray.SetLabels"); !strings.Contains(args, "Mostrar o lich") || !strings.Contains(args, "{count}") {
		t.Errorf("labels after the change %s", args)
	}
	u.followLanguage()
	select {
	case call := <-b.calls:
		t.Fatalf("the same change was sent twice: %s", call)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestTheTrayIsNotToldBeforeItIsUp(t *testing.T) {
	u, b := newGateWindow(t)
	inPortuguese(t)
	u.followLanguage()
	select {
	case call := <-b.calls:
		t.Fatalf("labels sent to a tray that is not up: %s", call)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestHotkeyTablesFollowTheLanguageOnTheNextFrame(t *testing.T) {
	u := &window{}
	u.followLanguage()
	if hotkeyActions[0].label != "New session" || hotkeyUnassigned != "Unassigned" || hotkeyGroups[0].label != "Sessions" {
		t.Fatalf("English tables: %q %q %q", hotkeyActions[0].label, hotkeyUnassigned, hotkeyGroups[0].label)
	}
	inPortuguese(t)
	u.followLanguage()
	if hotkeyActions[0].label != "Nova sessão" || hotkeyLabel(hotkeyNewSession) != "Nova sessão" {
		t.Errorf("action label %q, hotkeyLabel %q", hotkeyActions[0].label, hotkeyLabel(hotkeyNewSession))
	}
	if hotkeyUnassigned != "Sem atalho" || formatCombo(comboUnassigned, false) != hotkeyUnassigned {
		t.Errorf("unassigned %q, formatted %q", hotkeyUnassigned, formatCombo(comboUnassigned, false))
	}
	if hotkeyGroups[0].label != "Sessões" {
		t.Errorf("group label %q", hotkeyGroups[0].label)
	}
	if got := terminalCost(combo{Mod: true, Key: "c"}); !strings.HasPrefix(got, "Ctrl+C é o atalho do shell para interromper") {
		t.Errorf("terminal cost %q", got)
	}
}

func TestEveryKeyTheTablesNameResolvesInEveryLocale(t *testing.T) {
	var keys []string
	for _, a := range hotkeyActions {
		keys = append(keys, a.labelKey)
	}
	for _, g := range hotkeyGroups {
		keys = append(keys, g.labelKey)
	}
	for _, key := range terminalChordCosts {
		keys = append(keys, key)
	}
	for _, key := range costMissReasonKeys {
		keys = append(keys, key)
	}
	for _, item := range footerItems {
		if item.ExampleKey != "" {
			keys = append(keys, item.ExampleKey)
		}
	}
	for _, key := range footerZoneLabelKeys {
		keys = append(keys, key)
	}
	for _, l := range i18n.Locales {
		for _, key := range keys {
			if i18n.In(l, key, i18n.Args{"key": "C"}) == "" {
				t.Errorf("%s has no text for %s", l, key)
			}
		}
	}
}

func TestShellMessagesWithPlaceholdersAndPluralsInPortuguese(t *testing.T) {
	inPortuguese(t)
	if got := closeRunning(1); !strings.HasPrefix(got, "1 sessão está rodando.") {
		t.Errorf("one session: %q", got)
	}
	if got := closeRunning(3); !strings.HasPrefix(got, "3 sessões estão rodando.") {
		t.Errorf("three sessions: %q", got)
	}
	if got := notifySummary(statusWaiting, "Claude 3"); got != "Claude 3 precisa da sua atenção" {
		t.Errorf("needs input: %q", got)
	}
	if got := updateFailureText(&backend.AppUpdateProgress{Phase: backend.UpdatePhaseDownload, Received: 5, Total: 10}, "offline"); got != "Falha no download em 50%: offline" {
		t.Errorf("download failure: %q", got)
	}
	if got := skippedDrop("a.txt", "sem permissão"); got != "a.txt (sem permissão)" {
		t.Errorf("skipped drop: %q", got)
	}
	if before, after := attachToSplit(); before != "Anexar a " || after != "" {
		t.Errorf("attach hint split into %q and %q", before, after)
	}
	if got := footerExample("files"); got != "Arquivos" {
		t.Errorf("footer example %q", got)
	}
	if text, hint := pathText("/srv/app", "box"); text != "diretório desconhecido · dentro de box" || hint != text {
		t.Errorf("unknown cwd: %q, hint %q", text, hint)
	}
}

func TestPatchNotesGroupsAreNamedInTheInterfaceLanguage(t *testing.T) {
	groups := []backend.PatchNotesGroup{{Label: "Added", Items: []string{"a"}}, {Label: "Fixed", Items: []string{"a", "b"}}}
	if got := notesCounts(groups); got != "1 added"+notesCountSep+"2 fixed" {
		t.Errorf("English counts %q", got)
	}
	inPortuguese(t)
	if got := notesCounts(groups); got != "1 novidade"+notesCountSep+"2 correções" {
		t.Errorf("pt-BR counts %q", got)
	}
	if got := notesGroupLabel("Fixed"); got != "Correções" {
		t.Errorf("pt-BR label %q", got)
	}
	if got := notesGroupLabel("Highlights"); got != "Highlights" {
		t.Errorf("a heading outside Keep a Changelog's six is shown as written, got %q", got)
	}
}
