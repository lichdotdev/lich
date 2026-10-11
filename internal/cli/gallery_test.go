package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runGallery(t *testing.T, env map[string]string, args ...string) (int, string, string, []launched) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	var starts []launched
	code := dispatch(append([]string{"gallery"}, args...), &client{
		env:    func(key string) string { return env[key] },
		stdout: &stdout, stderr: &stderr, running: noInstance,
		launch: func(exe string, env, args []string) error {
			starts = append(starts, launched{exe: exe, env: env, args: args})
			return nil
		},
	})
	return code, stdout.String(), stderr.String(), starts
}

func TestGalleryLaunchesThePinnedBuildWithNoBackend(t *testing.T) {
	pinned := filepath.Join(t.TempDir(), galleryBinary)
	if err := os.WriteFile(pinned, nil, 0o755); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr, starts := runGallery(t, map[string]string{galleryEnv: pinned})
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if len(starts) != 1 || starts[0].exe != pinned {
		t.Fatalf("launched %+v, want only %s", starts, pinned)
	}
	if stdout == "" {
		t.Error("stdout is empty, want one line saying what opened")
	}
}

func TestGalleryWithAMissingPinStartsNothing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), galleryBinary)

	code, _, stderr, starts := runGallery(t, map[string]string{galleryEnv: missing})
	if code != 1 || !strings.Contains(stderr, galleryEnv) {
		t.Fatalf("exit = %d, stderr = %q, want the pin named", code, stderr)
	}
	if len(starts) != 0 {
		t.Errorf("launched %+v without a gallery to open", starts)
	}
}

// With no pin, the gallery is looked for beside the lich binary, which in a
// test is the test binary, with no lich-gallery beside it.
func TestGalleryWithNothingBesideLichSaysHowToBuildIt(t *testing.T) {
	code, _, stderr, starts := runGallery(t, nil)
	for _, want := range []string{"no " + galleryBinary + " beside", "./cmd/ui-gallery", galleryEnv} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr = %q, want %q", stderr, want)
		}
	}
	if code != 1 || len(starts) != 0 {
		t.Errorf("exit = %d, launched %+v", code, starts)
	}
}

func TestGalleryTakesNoArguments(t *testing.T) {
	code, _, stderr, starts := runGallery(t, nil, "now")
	if code != 1 || !strings.Contains(stderr, "usage: lich gallery") {
		t.Fatalf("exit = %d, stderr = %q, want the usage line", code, stderr)
	}
	if len(starts) != 0 {
		t.Errorf("launched %+v", starts)
	}
}
