package main

import (
	"bytes"
	"errors"
	"log"
	"os/exec"
	"strings"
)

// vimInstall runs command, a language server's install vim.lua offers, at
// the project in the background; the strip says so until it ends.
func (f *fileEditor) vimInstall(command string) {
	done := make(chan error, 1)
	f.vim.installing, f.vim.installErr = done, ""
	dir := f.root()
	go func() {
		done <- runInstall(dir, command)
		f.invalidate()
	}()
}

// vimInstalled starts the server again once an Install ended well, and
// keeps the failure of one that did not.
func (f *fileEditor) vimInstalled() {
	select {
	case err := <-f.vim.installing:
		f.vim.installing = nil
		if err != nil {
			f.vim.installErr = err.Error()
			return
		}
		f.vimFeed("lich_editor.retry(...)")
	default:
	}
}

// runInstall runs command at dir; its error is the output's last line, the
// whole output going to the log. vim.lua's commands quote nothing, so
// splitting on spaces is enough.
func runInstall(dir, command string) error {
	args := strings.Fields(command)
	if len(args) == 0 {
		return errors.New("no install command")
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	log.Printf("vim: %s: %v\n%s", command, err, out)
	lines := strings.Split(string(bytes.TrimSpace(out)), "\n")
	if last := strings.TrimSpace(lines[len(lines)-1]); last != "" {
		return errors.New(last)
	}
	return err
}
