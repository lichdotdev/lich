package agentplugin

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tarballPath is the URL path fileServerAt serves a release's tarball at.
func tarballPath(version string) string { return "/tarball/v" + version }

// releaseFiles is what a release's tarball unpacks to in these tests: the
// manifest Claude Code and Cursor CLI read, and the modules opencode and
// oh-my-pi load.
func releaseFiles() map[string]string {
	return map[string]string{
		".claude-plugin/plugin.json": `{"name":"lich"}`,
		"hooks/report-state.sh":      "#!/bin/sh\n",
		OpencodeModule:               "export default { server: async () => ({}) }\n",
		OMPExtension:                 "export default function lichPlugin(pi) {}\n",
	}
}

// pluginTarball is a release as codeload serves it: every entry under one top
// directory, git's pax header first.
func pluginTarball(t *testing.T, version string, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	top := "lich-plugin-" + version + "/"
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(tw.WriteHeader(&tar.Header{Typeflag: tar.TypeXGlobalHeader, Name: "pax_global_header", PAXRecords: map[string]string{"comment": "x"}}))
	must(tw.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: top, Mode: 0o755}))
	for name, body := range files {
		mode := int64(0o644)
		if strings.HasSuffix(name, ".sh") {
			mode = 0o755
		}
		must(tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: top + name, Mode: mode, Size: int64(len(body))}))
		_, err := tw.Write([]byte(body))
		must(err)
	}
	must(tw.Close())
	must(gz.Close())
	return buf.Bytes()
}

// bundleHome points the releases at a temporary config directory and returns
// the directory they unpack under.
func bundleHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	prev := userConfigDir
	userConfigDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { userConfigDir = prev })
	return filepath.Join(dir, "lich", "plugin", "releases")
}

// plantRelease lays out an unpacked release as an earlier unpack left it.
func plantRelease(t *testing.T, root, version string) string {
	t.Helper()
	dir := filepath.Join(root, version)
	for name, body := range releaseFiles() {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestInstallUnpacksTheReleaseSessionsLoad(t *testing.T) {
	s, _ := fileServer(t, nil)
	root := bundleHome(t)
	if err := s.installBundle(); err != nil {
		t.Fatalf("installBundle: %v", err)
	}
	dir, version, ok := BundleDir()
	if !ok || version != testVersion || dir != filepath.Join(root, testVersion) {
		t.Fatalf("BundleDir = (%q,%q,%v), want the release just unpacked", dir, version, ok)
	}
	for name, body := range releaseFiles() {
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil || string(data) != body {
			t.Errorf("%s = %q (err %v), want the release's %q", name, data, err, body)
		}
	}
	if info, err := os.Stat(filepath.Join(dir, "hooks", "report-state.sh")); err != nil || info.Mode().Perm()&0o100 == 0 {
		t.Errorf("a hook script lost its exec bit: %v (err %v)", info, err)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 {
		t.Errorf("releases dir holds %v, want the release alone, no temporary left", entries)
	}
}

// A release already unpacked is never fetched or written again: a session
// started on it is reading those files.
func TestInstallLeavesAnUnpackedReleaseAlone(t *testing.T) {
	s, asked := fileServer(t, nil)
	plantRelease(t, bundleHome(t), testVersion)
	if err := s.installBundle(); err != nil {
		t.Fatalf("installBundle: %v", err)
	}
	for _, path := range asked() {
		if path == tarballPath(testVersion) {
			t.Errorf("fetched the tarball of a release already unpacked")
		}
	}
}

func TestBundleDirPicksTheNewestCompatibleRelease(t *testing.T) {
	root := bundleHome(t)
	for _, v := range []string{"0.13.0", "0.14.2", PluginVersionCeiling, "0.12.0"} {
		plantRelease(t, root, v)
	}
	if err := os.MkdirAll(filepath.Join(root, bundleTmpPrefix+"0.15.0-123"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, version, ok := BundleDir(); !ok || version != "0.14.2" {
		t.Fatalf("BundleDir version = (%q,%v), want 0.14.2: past the ceiling, below the floor and half-unpacked ones are not loaded", version, ok)
	}
}

func TestBundleDirWithNothingUnpacked(t *testing.T) {
	bundleHome(t)
	if dir, version, ok := BundleDir(); ok || dir != "" || version != "" {
		t.Fatalf("BundleDir = (%q,%q,%v), want nothing", dir, version, ok)
	}
}

// A release that is not the plugin, or that will not arrive, leaves nothing a
// session would load.
func TestInstallLeavesNothingWhenTheReleaseIsNotThePlugin(t *testing.T) {
	for name, serve := range map[string]func(*Service, *testing.T){
		"tarball missing": func(s *Service, _ *testing.T) { s.tarballBase += "/missing" },
		"no manifest": func(s *Service, t *testing.T) {
			files := releaseFiles()
			delete(files, ".claude-plugin/plugin.json")
			s.tarballBase = serveBytes(t, tarballPath(testVersion), pluginTarball(t, testVersion, files))
		},
		"not a tarball": func(s *Service, t *testing.T) {
			s.tarballBase = serveBytes(t, tarballPath(testVersion), []byte("<html>"))
		},
	} {
		t.Run(name, func(t *testing.T) {
			s, _ := fileServer(t, nil)
			serve(s, t)
			root := bundleHome(t)
			if err := s.installBundle(); err == nil {
				t.Fatal("installBundle: want an error, got nil")
			}
			if _, _, ok := BundleDir(); ok {
				t.Error("a failed unpack left a release sessions would load")
			}
			if entries, _ := os.ReadDir(root); len(entries) != 0 {
				t.Errorf("a failed unpack left %v behind", entries)
			}
		})
	}
}

func TestEntryPath(t *testing.T) {
	for name, want := range map[string]struct {
		rel string
		ok  bool
	}{
		"lich-plugin-0.23.0/":                  {"", true},
		"lich-plugin-0.23.0/hooks/a.sh":        {"hooks/a.sh", true},
		"./lich-plugin-0.23.0/omp/lich.js":     {"omp/lich.js", true},
		"lich-plugin-0.23.0/../../etc/passwd":  {"", false},
		"../outside":                           {"", false},
		"/etc/passwd":                          {"", false},
		"lich-plugin-0.23.0/a/../../../escape": {"", false},
	} {
		rel, ok := entryPath(name)
		if rel != want.rel || ok != want.ok {
			t.Errorf("entryPath(%q) = (%q,%v), want (%q,%v)", name, rel, ok, want.rel, want.ok)
		}
	}
}

func TestUntarRefusesAnEntryOutsideTheArchive(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: "../escape", Mode: 0o644, Size: 1})
	_, _ = tw.Write([]byte("x"))
	_ = tw.Close()
	_ = gz.Close()
	dest := t.TempDir()
	if err := untar(&buf, filepath.Join(dest, "in")); err == nil {
		t.Fatal("untar: want an error for an entry climbing out, got nil")
	}
	if _, err := os.Stat(filepath.Join(dest, "escape")); !os.IsNotExist(err) {
		t.Errorf("the entry was written outside the destination (stat err = %v)", err)
	}
}
