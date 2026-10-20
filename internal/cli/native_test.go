package cli

import (
	"bytes"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/internal/restart"
	"github.com/lichdotdev/lich/internal/singleton"
)

// launched is one process `lich native` started.
type launched struct {
	exe  string
	env  []string
	args []string
}

// listeningLich is a lich listening on its port, answering nothing.
func listeningLich(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)
	return srv
}

func portOf(srv *httptest.Server) string {
	return strconv.Itoa(srv.Listener.Addr().(*net.TCPAddr).Port)
}

// nativeBuild is a lich-native on disk for LICH_NATIVE to point at.
func nativeBuild(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), restart.NativeBinary)
	if err := os.WriteFile(path, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func runNative(t *testing.T, env map[string]string, running func() (*singleton.Info, error),
	onLaunch func(launched)) (int, string, string, []launched) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	var starts []launched
	code := dispatch([]string{"native"}, &client{
		env:    func(key string) string { return env[key] },
		stdout: &stdout, stderr: &stderr, running: running,
		launch: func(exe string, env, args []string) error {
			l := launched{exe: exe, env: env, args: args}
			starts = append(starts, l)
			if onLaunch != nil {
				onLaunch(l)
			}
			return nil
		},
	})
	return code, stdout.String(), stderr.String(), starts
}

func TestNativeOpensOnTheRunningLich(t *testing.T) {
	t.Parallel()
	srv := listeningLich(t)
	native := nativeBuild(t)
	env := map[string]string{"LICH_PORT": portOf(srv), "LICH_TOKEN": "tok", restart.NativeEnv: native}

	code, stdout, stderr, starts := runNative(t, env, noInstance, nil)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if len(starts) != 1 || starts[0].exe != native {
		t.Fatalf("launched %+v, want only %s", starts, native)
	}
	for _, want := range []string{"LICH_PORT=" + portOf(srv), "LICH_TOKEN=tok"} {
		if !slices.Contains(starts[0].env, want) {
			t.Errorf("native window env lacks %s", want)
		}
	}
	if !strings.Contains(stdout, portOf(srv)) {
		t.Errorf("stdout = %q, want the port it opened on", stdout)
	}
}

// With no lich running, one is started without a window, and the native window
// opens on it once its runtime file names a port that answers.
func TestNativeStartsLichWithNoWindowWhenNoneRuns(t *testing.T) {
	t.Parallel()
	srv := listeningLich(t)
	native := nativeBuild(t)
	var started *singleton.Info
	running := func() (*singleton.Info, error) { return started, nil }
	port, _ := strconv.Atoi(portOf(srv))

	code, _, stderr, starts := runNative(t, map[string]string{restart.NativeEnv: native}, running, func(l launched) {
		if l.exe != native {
			started = &singleton.Info{Port: port, Token: "tok"}
		}
	})
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if len(starts) != 2 || !slices.Equal(starts[0].args, []string{"--no-window"}) || starts[1].exe != native {
		t.Fatalf("launched %+v, want lich --no-window and then %s", starts, native)
	}
}

func TestNativeWithoutABuildStartsNothing(t *testing.T) {
	t.Parallel()
	srv := listeningLich(t)
	missing := filepath.Join(t.TempDir(), restart.NativeBinary)
	env := map[string]string{"LICH_PORT": portOf(srv), "LICH_TOKEN": "tok", restart.NativeEnv: missing}

	code, _, stderr, starts := runNative(t, env, noInstance, nil)
	if code != 1 || !strings.Contains(stderr, restart.NativeEnv) {
		t.Fatalf("exit = %d, stderr = %q, want the pin named", code, stderr)
	}
	if len(starts) != 0 {
		t.Errorf("launched %+v without a native window to open", starts)
	}
}

func TestNativeTakesNoArguments(t *testing.T) {
	t.Parallel()
	f := newFakeLich(t, `null`)
	code, _, stderr := run(t, f, "native", "now")
	if code != 1 || !strings.Contains(stderr, "usage: lich native") {
		t.Fatalf("exit = %d, stderr = %q, want the usage line", code, stderr)
	}
}

// With no pin, the native window is looked for beside the lich binary, which in
// a test is the test binary, with no lich-native beside it.
func TestNativeWithNothingBesideLichSaysWhereToGetIt(t *testing.T) {
	t.Parallel()
	srv := listeningLich(t)
	env := map[string]string{"LICH_PORT": portOf(srv), "LICH_TOKEN": "tok"}

	code, _, stderr, starts := runNative(t, env, noInstance, nil)
	if code != 1 || !strings.Contains(stderr, "no "+restart.NativeBinary+" beside") || !strings.Contains(stderr, restart.NativeEnv) {
		t.Fatalf("exit = %d, stderr = %q, want where to build it and the pin", code, stderr)
	}
	if len(starts) != 0 {
		t.Errorf("launched %+v", starts)
	}
}

func TestAwaitBackendGivesUpOnALichThatNeverAnswers(t *testing.T) {
	t.Parallel()
	c := &client{env: func(string) string { return "" }, running: noInstance}
	if _, _, err := c.awaitBackend(3 * quitPoll); err == nil || !strings.Contains(err.Error(), "did not answer") {
		t.Fatalf("err = %v, want the timeout", err)
	}
}
