package providers

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/winexec"
)

// versionTimeout bounds one `--version`. The slowest of the eight measured
// under 0.7s warm (oh-my-pi and Cursor CLI, both booting a JS runtime); a cold
// cache or a version-manager shim resolving first can take several times that.
const versionTimeout = 10 * time.Second

// versionPattern finds the version in what a CLI prints, which no two of them
// spell alike: "2.1.299 (Claude Code)", "codex-cli 0.162.1", "omp/18.8.9",
// "crush version v0.98.1", "2026.10.01-e373342".
var versionPattern = regexp.MustCompile(`\d+\.\d+(?:\.\d+)?(?:-[0-9A-Za-z.]+)?`)

type Binary = api.ProviderBinary

// Version runs bin (resolved as Verify resolves it) with `--version` and
// returns the version it printed. It is the slow half of what Detect answers,
// kept out of it: a probe boots the whole CLI, and Detect runs on every launch
// menu.
//
// The probe inherits lich's own environment, PATH pinned from the login shell,
// so a shim that picks its version from the user's config answers as it would
// inside a session.
func (s *Service) Version(ctx context.Context, bin string) (string, error) {
	check := s.Verify(bin)
	if check.Status != CheckOK {
		return "", fmt.Errorf("resolve %q: %s", bin, check.Status)
	}
	return s.probe(ctx, check.Path)
}

// OnPath lists every distinct executable on $PATH that a provider's binary
// names resolve to, in $PATH order (so the first one is what Detect picks when
// no binary is configured), each with its version. Two entries that are the
// same file (a directory listed twice, two symlinks to one install) count once.
//
// A probe that fails leaves that entry's version empty rather than dropping
// it: the binary is still there for the spawn to find.
func (s *Service) OnPath(ctx context.Context, id string) ([]Binary, error) {
	names := binariesOf(id)
	if names == nil {
		return nil, fmt.Errorf("unknown provider %q", id)
	}
	out := []Binary{}
	seen := map[string]bool{}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		for _, name := range names {
			path, err := s.lookPath(filepath.Join(dir, name))
			if err != nil {
				continue
			}
			real, err := filepath.EvalSymlinks(path)
			if err != nil {
				real = path
			}
			if seen[real] {
				continue
			}
			seen[real] = true
			version, _ := s.probe(ctx, path)
			out = append(out, Binary{Path: path, Version: version})
		}
	}
	return out, nil
}

func binariesOf(id string) []string {
	for _, p := range Registry {
		if p.ID == id {
			return p.Binaries
		}
	}
	return nil
}

func (s *Service) probe(ctx context.Context, path string) (string, error) {
	run := s.runVersion
	if run == nil {
		run = runVersion
	}
	ctx, cancel := context.WithTimeout(ctx, versionTimeout)
	defer cancel()
	stdout, stderr, err := run(ctx, path)
	if err != nil {
		return "", fmt.Errorf("%s --version: %w", path, err)
	}
	if v := parseVersion(stdout); v != "" {
		return v, nil
	}
	if v := parseVersion(stderr); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("%s --version printed no version", path)
}

func runVersion(ctx context.Context, path string) (stdout, stderr []byte, err error) {
	cmd := exec.CommandContext(ctx, path, "--version")
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	// A CLI that forks a helper keeps the pipes open past the kill; WaitDelay
	// stops Wait from waiting on that helper.
	cmd.WaitDelay = time.Second
	winexec.Hide(cmd)
	err = cmd.Run()
	return out.Bytes(), errOut.Bytes(), err
}

// parseVersion is the first version-shaped token on the first line carrying
// one. Line by line, so a warning a CLI prints ahead of its version cannot lend
// it digits from a path.
func parseVersion(out []byte) string {
	lines := bufio.NewScanner(bytes.NewReader(out))
	for lines.Scan() {
		if v := versionPattern.FindString(strings.TrimSpace(lines.Text())); v != "" {
			return v
		}
	}
	return ""
}
