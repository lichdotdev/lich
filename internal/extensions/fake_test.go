package extensions

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
)

// fakeEnv turns the test binary into a fake extension; the file named "mode"
// in its working directory says which one.
const fakeEnv = "LICH_EXTENSIONS_FAKE"

const (
	modeFile   = "mode"
	runsFile   = "runs"
	scriptFile = "script"
	eofFile    = "eof"
	childFile  = "child"

	modeEcho       = "echo"
	modeCrash      = "crash"
	modeCrashTwice = "crash-twice"
	modeStubborn   = "stubborn"
	modeScript     = "script"
	modeStderr     = "stderr"
	modeDataDir    = "data-dir"
	modeLanguage   = "language"
	modeAPI        = "api"
	modeParent     = "parent"

	// fakeEnv set to fakeChild makes the test binary a process that only waits.
	fakeChild = "child"

	readyTitle = "ready"
)

func TestMain(m *testing.M) {
	if os.Getenv(fakeEnv) == fakeChild {
		time.Sleep(time.Hour)
		return
	}
	if os.Getenv(fakeEnv) != "" {
		fakeExtension()
		return
	}
	os.Exit(m.Run())
}

func fakeExtension() {
	runs := recordRun()
	mode, _ := os.ReadFile(modeFile)
	switch string(mode) {
	case modeCrash:
		os.Exit(3)
	case modeCrashTwice:
		if runs <= 2 {
			os.Exit(3)
		}
	case modeStubborn:
		fmt.Println(toastLine(readyTitle))
		time.Sleep(time.Hour)
	case modeParent:
		startChild()
		fmt.Println(toastLine(readyTitle))
		time.Sleep(time.Hour)
	case modeScript:
		script, err := os.Open(scriptFile)
		if err != nil {
			os.Exit(4)
		}
		_, _ = io.Copy(os.Stdout, script)
	case modeDataDir:
		fmt.Println(toastLine(os.Getenv(extension.DataDirEnv)))
	case modeLanguage:
		fmt.Println(toastLine("language " + os.Getenv(extension.LanguageEnv)))
	case modeAPI:
		apiExtension()
		return
	case modeStderr:
		fmt.Fprintln(os.Stderr, strings.Repeat("x", maxLogLineBytes+1))
		for i := range 3 * maxLogLines {
			fmt.Fprintf(os.Stderr, "noise %d\n", i)
		}
	}
	echo()
}

// echo answers each line with a toast titled after its kind, and leaves a mark
// when stdin ends: the difference between being asked to stop and being killed.
func echo() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(nil, maxLineBytes)
	for in.Scan() {
		var msg struct {
			Kind string `json:"kind"`
		}
		_ = json.Unmarshal(in.Bytes(), &msg)
		fmt.Println(toastLine("got " + msg.Kind))
	}
	_ = os.WriteFile(eofFile, nil, 0o644)
}

// startChild starts a process that outlives this one unless something ends it,
// and leaves its pid in childFile.
func startChild() {
	self, err := os.Executable()
	if err != nil {
		os.Exit(6)
	}
	child := exec.Command(self)
	child.Env = append(os.Environ(), fakeEnv+"="+fakeChild)
	if err := child.Start(); err != nil {
		os.Exit(6)
	}
	if err := os.WriteFile(childFile, []byte(strconv.Itoa(child.Process.Pid)), 0o644); err != nil {
		os.Exit(6)
	}
}

func toastLine(title string) string {
	raw, err := json.Marshal(extension.ShowToast.New(extension.ToastData{Title: title}))
	if err != nil {
		panic(err)
	}
	return string(raw)
}

func recordRun() int {
	f, err := os.OpenFile(runsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		os.Exit(5)
	}
	_, _ = f.WriteString(".")
	_ = f.Close()
	return runCount(".")
}

func runCount(root string) int {
	raw, _ := os.ReadFile(filepath.Join(root, runsFile))
	return len(raw)
}

// recorder is the window: what the backend pushed to it, in order.
type recorder struct {
	mu       sync.Mutex
	statuses []api.Extension
	messages []api.ExtensionMessage
	arrived  []time.Time
	removed  []string
	updates  []api.ExtensionUpdate
}

func (r *recorder) emit(name string, data any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch v := data.(type) {
	case api.Extension:
		if name != api.ExtensionStatusEvent.Name {
			panic("status under " + name)
		}
		r.statuses = append(r.statuses, v)
	case api.ExtensionMessage:
		if name != api.ExtensionMessageEvent.Name {
			panic("message under " + name)
		}
		r.messages = append(r.messages, v)
		r.arrived = append(r.arrived, time.Now())
	case api.ExtensionIDOptions:
		if name != api.ExtensionRemovedEvent.Name {
			panic("removal under " + name)
		}
		r.removed = append(r.removed, v.ID)
	case api.ExtensionUpdate:
		if name != api.ExtensionUpdateEvent.Name {
			panic("update under " + name)
		}
		r.updates = append(r.updates, v)
	}
}

func (r *recorder) statusNames() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	names := make([]string, 0, len(r.statuses))
	for _, status := range r.statuses {
		names = append(names, status.Status)
	}
	return names
}

func (r *recorder) lastStatus() api.Extension {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.statuses) == 0 {
		return api.Extension{}
	}
	return r.statuses[len(r.statuses)-1]
}

// titles are the toast titles delivered so far, in order.
func (r *recorder) titles(t *testing.T) []string {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	titles := make([]string, 0, len(r.messages))
	for _, m := range r.messages {
		var msg extension.Message
		if err := json.Unmarshal(m.Message, &msg); err != nil {
			t.Fatalf("a delivered message does not decode: %v", err)
		}
		if toast, ok := msg.Data.(extension.ToastData); ok {
			titles = append(titles, toast.Title)
		}
	}
	return titles
}

func waitUntil(t *testing.T, what string, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func waitStatus(t *testing.T, r *recorder, status string) {
	t.Helper()
	waitUntil(t, "status "+status, func() bool { return r.lastStatus().Status == status })
}

func validManifest(id, executable string) extension.Manifest {
	return extension.Manifest{
		FormatVersion: extension.ManifestFormat, ID: id, Name: "Fake " + id, Version: "1.0.0", Executable: executable,
		Surfaces: []extension.Surface{{ID: "main", Title: "Main", Icon: "box"}},
	}
}

// install lays out an extension whose executable is the test binary, adjusted
// by mutate before the manifest is written.
func install(t *testing.T, dir, id, mode string, mutate ...func(*extension.Manifest)) string {
	t.Helper()
	root := filepath.Join(dir, id)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	executable := "ext" + filepath.Ext(self)
	linkOrCopy(t, self, filepath.Join(root, executable))
	writeFile(t, filepath.Join(root, modeFile), mode)
	m := validManifest(id, executable)
	for _, change := range mutate {
		change(&m)
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, extension.ManifestFile), string(raw))
	return root
}

func linkOrCopy(t *testing.T, from, to string) {
	t.Helper()
	if os.Link(from, to) == nil {
		return
	}
	src, err := os.Open(from)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	dst, err := os.OpenFile(to, os.O_CREATE|os.O_WRONLY, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// boot runs the extensions under dir on a clock fit for tests.
func boot(t *testing.T, dir string, adjust ...func(*timing)) (*Service, *recorder) {
	t.Helper()
	r := &recorder{}
	s := New(dir, filepath.Join(t.TempDir(), "data"), "1.2.3", append(os.Environ(), fakeEnv+"=1"), r.emit)
	s.timing = timing{firstBackoff: time.Millisecond, maxBackoff: 4 * time.Millisecond, stableRun: time.Hour, stopGrace: 5 * time.Second, killWait: 10 * time.Second, callTimeout: callTimeout, stagedTTL: time.Hour}
	for _, change := range adjust {
		change(&s.timing)
	}
	s.catalogURL = (&fakeCatalog{status: http.StatusNotFound}).serve(t)
	t.Cleanup(s.Shutdown)
	s.Boot()
	return s, r
}
