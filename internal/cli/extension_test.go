package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/extension"
)

func extensionClient() (*client, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	return &client{env: func(string) string { return "" }, stdout: &stdout, stderr: &stderr}, &stdout, &stderr
}

// extensionFolder lays out an extension with description and returns its
// folder.
func extensionFolder(t *testing.T, description string) string {
	t.Helper()
	dir := t.TempDir()
	m := extension.Manifest{FormatVersion: extension.ManifestFormat, ID: "hello", Name: "Hello", Description: description, Version: "0.1.0", Executable: "hello"}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, extension.ManifestFile), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hello"), []byte("bin"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestExtensionCheckPassesAValidFolder(t *testing.T) {
	c, stdout, stderr := extensionClient()
	dir := extensionFolder(t, "Says hello")
	if code := dispatch([]string{"extension", "check", dir}, c); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "passes: hello 0.1.0") || stderr.Len() != 0 {
		t.Errorf("stdout = %q, stderr = %q, want a pass and no warning", stdout.String(), stderr.String())
	}
}

func TestExtensionCheckWarnsOfNoDescriptionAndStillPasses(t *testing.T) {
	c, _, stderr := extensionClient()
	if code := dispatch([]string{"extension", "check", extensionFolder(t, "")}, c); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "warning") || !strings.Contains(stderr.String(), "catalog will not list it") {
		t.Errorf("stderr = %q, want the catalog warning", stderr.String())
	}
}

func TestExtensionCheckFailsNamingWhatFails(t *testing.T) {
	c, _, stderr := extensionClient()
	dir := extensionFolder(t, "Says hello")
	if err := os.Remove(filepath.Join(dir, "hello")); err != nil {
		t.Fatal(err)
	}
	if code := dispatch([]string{"extension", "check", dir}, c); code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "fails") || !strings.Contains(stderr.String(), `executable "hello"`) {
		t.Errorf("stderr = %q, want it to name the executable", stderr.String())
	}
}

func TestExtensionWithoutCheckIsAUsageError(t *testing.T) {
	c, _, stderr := extensionClient()
	if code := dispatch([]string{"extension", "lint", "x"}, c); code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "usage: lich extension check <path>") {
		t.Errorf("stderr = %q, want the usage line", stderr.String())
	}
}
