package main

// Installing a missing language server, in the background, into a folder of
// lich's own (lspInstallRoot), from mason-registry's recipe as godemirror's
// catalog carries it.

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/godemirror/lsp/servers"

	"github.com/lichdotdev/lich/native/backend"
)

// lspInstallable is in, when lich knows how to install it and the tool the
// recipe needs is on the PATH; nil otherwise.
func lspInstallable(in *servers.Install) *servers.Install {
	if in == nil {
		return nil
	}
	if _, err := exec.LookPath(lspInstallTool(in.Kind)); err != nil {
		return nil
	}
	return in
}

// lspInstallTool is the program a recipe of kind runs, "" for a kind lich
// does not install. A GitHub release, which needs a download and unpacking
// per platform, is not one yet.
func lspInstallTool(kind servers.Kind) string {
	switch kind {
	case servers.KindGolang:
		return "go"
	case servers.KindNpm:
		return "npm"
	case servers.KindPypi:
		return "python3"
	case servers.KindCargo:
		return "cargo"
	}
	return ""
}

// lspInstallCommands are the commands that install in into dir.
func lspInstallCommands(in *servers.Install, dir string) [][]string {
	pkg := in.Package + "@" + in.Version
	switch in.Kind {
	case servers.KindGolang:
		return [][]string{{"go", "install", pkg}}
	case servers.KindNpm:
		return [][]string{append([]string{"npm", "install", "--prefix", dir, pkg}, in.ExtraPackages...)}
	case servers.KindPypi:
		venv := filepath.Join(dir, "venv")
		return [][]string{{"python3", "-m", "venv", venv}, {filepath.Join(venv, "bin", "pip"), "install", in.Package + "==" + in.Version}}
	case servers.KindCargo:
		return [][]string{{"cargo", "install", "--root", dir, "--version", in.Version, in.Package}}
	}
	return nil
}

// lspInstall installs the editor's missing server in the background; the
// header says so until it ends.
func (f *fileEditor) lspInstall() {
	in, name := f.lsp.install, f.lsp.name
	root, err := lspInstallRoot()
	if err != nil || in == nil {
		return
	}
	done := make(chan error, 1)
	f.lsp.installing, f.lsp.installErr = done, ""
	dir := filepath.Join(root, name)
	go func() {
		done <- runLspInstall(in, dir)
		f.invalidate()
	}()
}

// lspInstalled chooses the server again once an install ended well, and
// keeps the failure of one that did not.
func (f *fileEditor) lspInstalled() {
	select {
	case err := <-f.lsp.installing:
		f.lsp.installing = nil
		if err != nil {
			f.lsp.installErr = err.Error()
			return
		}
		f.lsp.gen = 0
	default:
	}
}

func runLspInstall(in *servers.Install, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, args := range lspInstallCommands(in, dir) {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOBIN="+filepath.Join(dir, "bin"))
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("lsp: install %s: %s: %v\n%s", in.Mason, strings.Join(args, " "), err, out)
			return lastLineOr(out, err)
		}
	}
	return nil
}

// lastLineOr is the output's last line as an error, err when it is empty.
func lastLineOr(out []byte, err error) error {
	lines := strings.Split(string(bytes.TrimSpace(out)), "\n")
	if last := strings.TrimSpace(lines[len(lines)-1]); last != "" {
		return errors.New(last)
	}
	return err
}

// lspApplyElsewhere writes a rename's or a code action's edits to the
// checkout's other files, as the disk holds them.
func (f *fileEditor) lspApplyElsewhere(edits []godemirror.DocumentEdit) {
	client, checkout := f.client, f.checkout
	go func() {
		for _, e := range edits {
			if err := applyFileEdits(client, checkout, e); err != nil {
				log.Printf("lsp: %s: %v", e.URI, err)
			}
		}
		f.invalidate()
	}()
}

func applyFileEdits(client *backend.Client, checkout string, e godemirror.DocumentEdit) error {
	rel, err := filepath.Rel(checkout, lspPathOf(e.URI))
	if err != nil || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("outside the checkout")
	}
	ctx := context.Background()
	text, err := client.ReadFile(ctx, checkout, filepath.ToSlash(rel))
	if err != nil {
		return err
	}
	return client.WriteFile(ctx, checkout, filepath.ToSlash(rel), applyTextEdits(text, e.Edits))
}

// applyTextEdits is text with edits made, their spans all in text as it is;
// a span past a line's end or the text's is held inside it.
func applyTextEdits(text string, edits []godemirror.TextEdit) string {
	starts := []int{0}
	for i, c := range text {
		if c == '\n' {
			starts = append(starts, i+1)
		}
	}
	offset := func(p godemirror.LineColumn) int {
		if p.Line >= len(starts) {
			return len(text)
		}
		end := len(text)
		if p.Line+1 < len(starts) {
			end = starts[p.Line+1] - 1
		}
		return min(starts[p.Line]+p.Column, end)
	}
	sorted := slices.Clone(edits)
	slices.SortStableFunc(sorted, func(a, b godemirror.TextEdit) int { return cmp.Compare(offset(b.Start), offset(a.Start)) })
	for _, e := range sorted {
		from, to := offset(e.Start), offset(e.End)
		text = text[:from] + e.Text + text[max(from, to):]
	}
	return text
}
