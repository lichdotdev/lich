package main

import (
	"testing"

	"github.com/lichdotdev/lich/native/i18n"
)

func TestNotificationSwitchesTurnInTheFrameThatTookTheClick(t *testing.T) {
	for _, title := range []string{"settings.notificationsSettings.needsInputTitle", "settings.notificationsSettings.finishedTitle"} {
		t.Run(title, func(t *testing.T) {
			_, r := settingsFrames(t, sectionGeneral)
			r.settle(t)
			r.click(t, r.controlOf(t, i18n.T(title)))
		})
	}
}

func sandboxFrames(t *testing.T) (*settingsScreen, *frameRig) {
	t.Helper()
	s, r := settingsFrames(t, sectionSandbox)
	s.sandbox.machine = sandboxMachine{loaded: true, backend: "bwrap", globals: map[string]string{}}
	return s, r
}

func TestSandboxGrantSwitchTurnsAProjectsFirstOverrideInTheFrameThatTookTheClick(t *testing.T) {
	s, r := sandboxFrames(t)
	s.scope = "pg"
	r.settle(t)
	r.click(t, r.controlOf(t, i18n.T("settings.sandboxSettings.sshTitle")))
}

func TestSandboxRungTightensInTheFrameThatTookTheClick(t *testing.T) {
	_, r := sandboxFrames(t)
	r.clickLabel(t, rungLabel(sandboxRungs[len(sandboxRungs)-1]))
}

// claudeScreen is Settings on Claude Code's own provider screen.
func claudeScreen(t *testing.T) (*settingsScreen, *frameRig) {
	t.Helper()
	s, r := settingsFrames(t, sectionProviders)
	s.providers.open = "claude"
	r.settle(t)
	return s, r
}

func TestProviderSwitchTurnsInTheFrameThatTookTheClick(t *testing.T) {
	_, r := claudeScreen(t)
	r.click(t, r.controlOf(t, i18n.T("settings.ultracodeSetting.title")))
}

func TestProviderLadderStepsDownInTheFrameThatTookTheClick(t *testing.T) {
	s, r := settingsFrames(t, sectionProviders)
	s.providers.open = "claude"
	s.providers.book.setGlobal(providerSkipKey("claude", false), "true")
	r.clickLabel(t, i18n.T(skipRungs[0].labelKey))
	if got := s.providers.snapshot().skipLevel("claude"); got != skipNever {
		t.Fatalf("skip is %q, want never", got)
	}
}

func TestProvidersPaneOpensAProviderInTheFrameAfterTheClick(t *testing.T) {
	s, r := settingsFrames(t, sectionProviders)
	r.clickLabel(t, "Codex")
	if s.providers.open != "codex" {
		t.Fatalf("on %q, want Codex's screen", s.providers.open)
	}
}

func TestProviderScreenBackInTheFrameAfterTheClick(t *testing.T) {
	s, r := claudeScreen(t)
	r.clickLabel(t, i18n.T("settings.providerDetail.back"))
	if s.providers.open != "" {
		t.Fatalf("still on %q", s.providers.open)
	}
}

func TestProvidersAddFoldOpensInTheFrameAfterTheClick(t *testing.T) {
	s, r := settingsFrames(t, sectionProviders)
	s.setProviderEnabled("codex", false)
	r.clickLabel(t, i18n.T("settings.providersPane.addProvider"))
	if !s.providers.adding {
		t.Fatal("the fold stayed shut")
	}
}
