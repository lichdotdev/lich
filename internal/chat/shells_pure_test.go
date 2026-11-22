package chat

import (
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
)

// The three sentences 2.1.330's Bash result names a background command's
// output file in.
func TestClaudeOutputPathReadsEveryBackgroundResult(t *testing.T) {
	const path = "/tmp/claude 1000/-a-b. c/s/tasks/b1.output"
	for _, text := range []string{
		"Command running in background with ID: b1. Output is being written to: " + path + ". You will be notified when it completes.",
		"Command was moved to the background (ID: b1) after running for 12s. Output is being written to: " + path + ".",
		"Command did not complete within its 120s timeout and was moved to the background (ID: b1). Output is being written to: " + path + ".",
	} {
		if got := claudeOutputPath(text, "b1"); got != path {
			t.Errorf("claudeOutputPath(%q) = %q, want %q", text, got, path)
		}
	}
	if got := claudeOutputPath("hello", "b1"); got != "" {
		t.Errorf("a plain result named %q", got)
	}
}

func TestClaudeShellMovedToTheBackgroundIsShownThen(t *testing.T) {
	var s claudeShells
	now := time.UnixMilli(1000)
	cmd := func(string) string { return "make" }
	if s.task("task_started", []byte(`{"task_id":"b1","tool_use_id":"t1","task_type":"local_bash","is_backgrounded":false}`), cmd, now) {
		t.Error("a foreground command was shown")
	}
	if !s.task("task_updated", []byte(`{"task_id":"b1","patch":{"is_backgrounded":true}}`), cmd, now) {
		t.Error("the move to the background was not shown")
	}
	if got := s.shown(); len(got) != 1 || got[0].Command != "make" || got[0].State != api.ShellRunning {
		t.Errorf("shown = %+v", got)
	}
	s.task("task_notification", []byte(`{"task_id":"b1","status":"failed"}`), cmd, now)
	if got := s.shown(); got[0].State != api.ShellFailed || got[0].Ended != 1000 {
		t.Errorf("after its end = %+v", got)
	}
}

func TestClaudeShellCommandFallsBackToItsDescription(t *testing.T) {
	var s claudeShells
	s.task("task_started", []byte(`{"task_id":"b1","task_type":"local_bash","description":"Watch","is_backgrounded":true}`), func(string) string { return "" }, time.Now())
	if got := s.shown(); got[0].Command != "Watch" {
		t.Errorf("command = %q, want the description", got[0].Command)
	}
}

func TestOutputTailIsPlainText(t *testing.T) {
	got := outputTail("cut\n\x1b]0;title\x07\x1b[1;31mred\x1b[0m\r\n10%\r50%\r100%\n", true)
	if want := "red\n100%\n"; got != want {
		t.Errorf("outputTail = %q, want %q", got, want)
	}
	if got := outputTail("whole\n", false); got != "whole\n" {
		t.Errorf("a read from the start lost its first line: %q", got)
	}
}
