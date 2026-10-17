//go:build !windows

package terminal

import (
	"slices"
	"testing"

	"github.com/lichdotdev/lich/internal/prompt"
)

// Text composed outside the lich process (the CLI, the MCP server, the plugin)
// has no way to read the setting, so every session carries the language it was
// spawned with. Unix-only for its stubBins store, which lives in the Unix suite.
func TestSessionEnvExportsThePromptLanguage(t *testing.T) {
	t.Parallel()
	s := &Service{env: []string{"A=1"}, store: stubBins{}, ws: &transport{port: 4321, token: "tok"}}
	if env := s.sessionEnv("sess", "p1", ""); !slices.Contains(env, "LICH_PROMPT_LANG=en") {
		t.Errorf("an unwired spawn did not export English: %v", env)
	}

	s.SetPromptLanguage(func() prompt.Lang { return prompt.PortugueseBR })
	if env := s.sessionEnv("sess", "p1", ""); !slices.Contains(env, "LICH_PROMPT_LANG=pt-BR") {
		t.Errorf("the chosen language was not exported: %v", env)
	}
}

func TestSessionEnvExportsSpanish(t *testing.T) {
	t.Parallel()
	s := &Service{env: []string{"A=1"}, store: stubBins{}, ws: &transport{port: 4321, token: "tok"}}
	s.SetPromptLanguage(func() prompt.Lang { return prompt.Spanish })
	if env := s.sessionEnv("sess", "p1", ""); !slices.Contains(env, "LICH_PROMPT_LANG=es") {
		t.Errorf("the chosen language was not exported: %v", env)
	}
}

func TestSessionEnvExportsJapanese(t *testing.T) {
	t.Parallel()
	s := &Service{env: []string{"A=1"}, store: stubBins{}, ws: &transport{port: 4321, token: "tok"}}
	s.SetPromptLanguage(func() prompt.Lang { return prompt.Japanese })
	if env := s.sessionEnv("sess", "p1", ""); !slices.Contains(env, "LICH_PROMPT_LANG=ja") {
		t.Errorf("the chosen language was not exported: %v", env)
	}
}

func TestSessionEnvExportsKorean(t *testing.T) {
	t.Parallel()
	s := &Service{env: []string{"A=1"}, store: stubBins{}, ws: &transport{port: 4321, token: "tok"}}
	s.SetPromptLanguage(func() prompt.Lang { return prompt.Korean })
	if env := s.sessionEnv("sess", "p1", ""); !slices.Contains(env, "LICH_PROMPT_LANG=ko") {
		t.Errorf("the chosen language was not exported: %v", env)
	}
}

func TestSessionEnvExportsRussian(t *testing.T) {
	t.Parallel()
	s := &Service{env: []string{"A=1"}, store: stubBins{}, ws: &transport{port: 4321, token: "tok"}}
	s.SetPromptLanguage(func() prompt.Lang { return prompt.Russian })
	if env := s.sessionEnv("sess", "p1", ""); !slices.Contains(env, "LICH_PROMPT_LANG=ru") {
		t.Errorf("the chosen language was not exported: %v", env)
	}
}
