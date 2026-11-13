package ompprofile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lichdotdev/lich/internal/profilestore"
)

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestAProfileKeepsItsOwnModelsAndTakesPersonalsSettings(t *testing.T) {
	personal := machine(t)
	shared := filepath.Join(personal, "config.yml")
	mustWrite(t, shared, "modelRoles:\n  default: a/one:high\ntheme:\n  dark: nord\n")
	p, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(p.Dir, "config.yml")
	// What `/model` run in the profile leaves there.
	mustWrite(t, config, "modelRoles:\n  default: b/two:low\n  smol: b/mini\ntheme:\n  dark: nord\ndefaultThinkingLevel: low\n")
	mustWrite(t, shared, "# mine\nmodelRoles:\n  default: c/three:max\nmodelRolesNote: kept\nstatusLine:\n  text: |\n    modelRoles: in a string\n\ntheme:\n  dark: obsidian\n")

	if err := EnsureShared(p.Slug); err != nil {
		t.Fatal(err)
	}
	want := "# mine\nmodelRolesNote: kept\nstatusLine:\n  text: |\n    modelRoles: in a string\n\ntheme:\n  dark: obsidian\n" +
		"modelRoles:\n  default: b/two:low\n  smol: b/mini\ndefaultThinkingLevel: low\n"
	if got := readFile(t, config); got != want {
		t.Errorf("config.yml =\n%s\nwant\n%s", got, want)
	}
	if err := EnsureShared(p.Slug); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, config); got != want {
		t.Errorf("a second spawn changed config.yml again:\n%s", got)
	}
	if got := readFile(t, shared); got[:7] != "# mine\n" {
		t.Errorf("syncing the profile changed Personal's config.yml:\n%s", got)
	}
}

func TestAConfigLinkedToPersonalsBecomesTheProfilesOwn(t *testing.T) {
	personal := machine(t)
	p, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	shared := filepath.Join(personal, "config.yml")
	mustWrite(t, shared, "modelRoles:\n  default: a/one\n")
	config := filepath.Join(p.Dir, "config.yml")
	if err := os.Symlink(shared, config); err != nil {
		t.Skipf("no file links here: %v", err)
	}

	if err := EnsureShared(p.Slug); err != nil {
		t.Fatal(err)
	}
	if profilestore.IsLink(config) {
		t.Fatal("config.yml is still linked to Personal's")
	}
	if got := readFile(t, config); got != "modelRoles:\n  default: a/one\n" {
		t.Errorf("a profile moved off the shared file lost the model it ran on: %q", got)
	}
}

func TestWithoutPersonalsConfigAProfileKeepsItsOwn(t *testing.T) {
	machine(t)
	p, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(p.Dir, "config.yml")
	mustWrite(t, config, "theme: mine\n")
	if err := EnsureShared(p.Slug); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, config); got != "theme: mine\n" {
		t.Errorf("the profile's config.yml was rewritten: %q", got)
	}
}
