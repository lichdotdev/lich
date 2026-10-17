package terminal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/xdgprofile"
)

// xdgMachine is profileMachine for the link-home providers: Personal's base
// directories under a scratch home, with another tool's state in each.
func xdgMachine(t *testing.T) string {
	t.Helper()
	home := conversationHome(t)
	t.Setenv("LICH_DEV", "")
	plant(t, filepath.Join(home, ".config", "gh", "hosts.yml"), "x")
	plant(t, filepath.Join(home, ".local", "share", "mise", "installs"), "x")
	return home
}

// opencodeDBAt plants an opencode database holding one session.
func opencodeDBAt(t *testing.T, dataDir, id string) {
	t.Helper()
	plantDB(t, filepath.Join(dataDir, opencodeDBFile),
		`CREATE TABLE session (id TEXT PRIMARY KEY, parent_id TEXT, directory TEXT, title TEXT,
		   time_updated INTEGER, time_archived INTEGER)`,
		fmt.Sprintf(`INSERT INTO session VALUES ('%s', NULL, '/work', 't', 1, NULL)`, id))
}

// TestAnXDGSpawnRunsInItsLinkHome: the process gets the profile's link home as
// its base directory, through which every other tool still finds its state,
// and none of the variables that would send it to another account.
func TestAnXDGSpawnRunsInItsLinkHome(t *testing.T) {
	xdgMachine(t)
	work, err := xdgprofile.Cursor.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(t.TempDir(), "spawns")
	bin := filepath.Join(t.TempDir(), "fake-cursor")
	script := fmt.Sprintf("#!/bin/sh\necho \"$XDG_CONFIG_HOME|${CURSOR_API_KEY:-}|$(cat \"$XDG_CONFIG_HOME/gh/hosts.yml\")\" >> %q\nexec cat\n", log)
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	svc := New(stubBins{bin: bin, profile: work.Slug, movedProfiles: map[string]string{}},
		[]string{"CURSOR_API_KEY=key"}, events.New())
	t.Cleanup(func() { _ = svc.Close("s1") })

	if err := svc.Start("s1", "p1", t.TempDir(), providers.Cursor, "", "", false, false, 80, 24); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return len(spawnLines(t, log)) == 1 }, "the spawn")
	if got, want := spawnLines(t, log)[0], work.Dir+"||x"; got != want {
		t.Errorf("spawn %q, want %q", got, want)
	}
	if got := svc.XDGProfileSessions(providers.Cursor, work.Slug); got != 1 {
		t.Errorf("the profile counts %d sessions, want 1", got)
	}
	if got := svc.XDGProfileSessions(providers.Kiro, work.Slug); got != 0 {
		t.Errorf("a Cursor session counted against the Kiro profile of the same slug: %d", got)
	}
}

// TestXDGConversationsAreMarkedWithTheirProfile: History lists the opencode
// and Cursor CLI conversations of every profile, a resume of one runs in the
// profile holding it, and a Cursor chat continues in another profile.
func TestXDGConversationsAreMarkedWithTheirProfile(t *testing.T) {
	home := xdgMachine(t)
	oc, err := xdgprofile.OpenCode.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	cu, err := xdgprofile.Cursor.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	opencodeDBAt(t, filepath.Join(home, ".local", "share", "opencode"), "ses_p")
	opencodeDBAt(t, filepath.Join(oc.Dir, "opencode"), "ses_w")
	cwd := filepath.Join(home, "work")
	chat := filepath.Join(cu.Dir, "cursor", "chats", cursorWorkspaceKey(cwd), "chat_w")
	plant(t, filepath.Join(chat, "store.db"), "")
	plant(t, filepath.Join(chat, "meta.json"), fmt.Sprintf(`{"cwd":%q,"hasConversation":true}`, cwd))

	got := map[string]string{}
	for _, c := range append(opencodeConversations(), cursorConversations()...) {
		got[c.ID] = c.Profile
	}
	if len(got) != 3 || got["ses_p"] != "personal" || got["ses_w"] != oc.Slug || got["chat_w"] != cu.Slug {
		t.Errorf("conversations = %v", got)
	}
	svc := New(stubBins{}, nil, events.New())
	for kind, id := range map[string]string{providers.OpenCode: "ses_w", providers.Cursor: "chat_w"} {
		if !svc.ResumeAvailable(kind, id, cwd) {
			t.Errorf("%s: %s in a profile cannot be resumed", kind, id)
		}
	}

	moved := map[string]string{}
	svc = New(stubBins{providerSession: "chat_w", movedProfiles: moved, kinds: map[string]string{"s1": providers.Cursor}}, nil, events.New())
	if err := svc.ContinueIn("s1", "personal"); err != nil {
		t.Fatal(err)
	}
	// With no XDG_CONFIG_HOME, Personal's chats are in ~/.cursor.
	if _, err := os.Stat(filepath.Join(home, ".cursor", "chats", cursorWorkspaceKey(cwd), "chat_w", "store.db")); err != nil {
		t.Errorf("the chat did not move to Personal: %v", err)
	}
	if got, ok := moved["s1"]; !ok || got != "" {
		t.Errorf("row profile %q (set %v), want Personal", got, ok)
	}

	svc = New(stubBins{providerSession: "ses_w", kinds: map[string]string{"s2": providers.OpenCode}}, nil, events.New())
	if err := svc.ContinueIn("s2", "personal"); err == nil || !strings.Contains(err.Error(), "cannot move") {
		t.Errorf("an opencode conversation continued in another profile: %v", err)
	}
}

// TestAKiroResumeStaysInTheRowsProfile: Kiro's conversations are every
// profile's, so a resume runs where the session was born and Continue in only
// changes the row.
func TestAKiroResumeStaysInTheRowsProfile(t *testing.T) {
	home := xdgMachine(t)
	work, err := xdgprofile.Kiro.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	plant(t, filepath.Join(home, ".kiro", "sessions", "cli", "k1.json"), `{}`)
	moved := map[string]string{}
	svc := New(stubBins{profile: work.Slug, providerSession: "k1", movedProfiles: moved,
		kinds: map[string]string{"s1": providers.Kiro}}, nil, events.New())

	if dir, err := svc.profileDir("s1", providers.Kiro, "k1"); err != nil || dir != work.Dir {
		t.Errorf("resume: %q, %v, want %q", dir, err, work.Dir)
	}
	if err := svc.ContinueIn("s1", "personal"); err != nil {
		t.Fatal(err)
	}
	if got, ok := moved["s1"]; !ok || got != "" {
		t.Errorf("row profile %q (set %v), want Personal", got, ok)
	}
	if _, err := os.Stat(filepath.Join(home, ".kiro", "sessions", "cli", "k1.json")); err != nil {
		t.Errorf("the conversation moved: %v", err)
	}
}
