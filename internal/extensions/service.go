// Package extensions runs what the user installed under lich's extensions
// directory: separate programs that speak the JSON lines of the extension
// contract over stdin and stdout. Each is started at boot, restarted when it
// crashes, and stopped with the backend. An extension is unsandboxed code, but
// what it writes is input the window draws, so every line is vetted here.
package extensions

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
	exti18n "github.com/lichdotdev/lich/extension/i18n"
	"github.com/lichdotdev/lich/internal/events"
)

const (
	statusInvalid    = api.ExtensionInvalid
	statusRunning    = api.ExtensionRunning
	statusRestarting = api.ExtensionRestarting
	statusFailed     = api.ExtensionFailed
	statusStopped    = api.ExtensionStopped
)

const (
	// maxQuickCrashes in a row is when lich stops restarting an extension.
	maxQuickCrashes = 5
	// stableRun is how long a run has to last for its crash not to count as a
	// quick one, which also starts the backoff over.
	stableRun    = 30 * time.Second
	firstBackoff = time.Second
	maxBackoff   = 30 * time.Second
	// stopGrace is the time from closing stdin to the kill.
	stopGrace = 3 * time.Second
	// killWait bounds the wait for a killed process, which may be stuck in the
	// kernel and is not worth holding up lich's exit for.
	killWait = 5 * time.Second
	// sendQueue is how many messages can wait for an extension to read its
	// stdin before Send refuses more, rather than block its caller on a full
	// pipe.
	sendQueue = 64
)

// timing is apart from the constants so tests can run a crash loop in
// milliseconds.
type timing struct {
	firstBackoff time.Duration
	maxBackoff   time.Duration
	stableRun    time.Duration
	stopGrace    time.Duration
	killWait     time.Duration
	callTimeout  time.Duration
	stagedTTL    time.Duration
}

// Service is every installed extension and its process.
type Service struct {
	dir         string
	dataDir     string
	lichVersion string
	env         []string
	// language is the tag an extension starts with in extension.LanguageEnv.
	language func() string
	emit     func(name string, data any)
	timing   timing
	lich     Lich
	calls    map[string]route

	// The installer's GitHub reads, fields so tests can serve releases locally.
	metadata   *http.Client
	download   *http.Client
	releaseAPI string
	goos       string
	goarch     string
	// installMu serialises installs, so two of one id never race to its
	// directory.
	installMu sync.Mutex
	// stagedMu guards staged, the prepared installs by token.
	stagedMu sync.Mutex
	staged   map[string]stagedInstall

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu  sync.Mutex
	all []*installed
	// serviceStates is each service's state as the last services event had it.
	serviceStates map[string]string
}

// installed is one extension directory. The fields below mu are Service.mu's;
// status changes only on the extension's supervise goroutine, which is what
// keeps its status events in order.
type installed struct {
	id       string
	root     string
	exe      string
	manifest extension.Manifest
	text     map[string]api.ExtensionText
	source   source
	invalid  error
	stderr   logBudget
	refused  logBudget
	renders  renderGate
	// restartMu serialises Restart calls on this extension.
	restartMu sync.Mutex
	// inFlight holds a token for each call running, up to maxCallsInFlight.
	inFlight chan struct{}

	// mu: Service.mu
	status  string
	errText string
	queue   chan []byte
	// subscribed is the events the running process asked for, by name.
	subscribed map[string]bool
	// tickets is the pending sends this extension may collect. They outlive a
	// restart, as the sends do.
	tickets map[string]bool
	// notified is when its last notifications went up, the newest last.
	notified []time.Time
	// stop and done belong to the current supervise goroutine; done closes
	// when it has returned.
	stop context.CancelFunc
	done chan struct{}
}

// New returns the extensions under dir, none discovered or running until Boot.
// env is what their processes get, and each one's data directory is under
// dataDir.
func New(dir, dataDir, lichVersion string, env []string, emit func(name string, data any)) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{
		dir: dir, dataDir: dataDir, lichVersion: lichVersion, env: env, emit: emit, ctx: ctx, cancel: cancel,
		timing: timing{
			firstBackoff: firstBackoff, maxBackoff: maxBackoff, stableRun: stableRun, stopGrace: stopGrace,
			killWait: killWait, callTimeout: callTimeout, stagedTTL: stagedTTL,
		},
		metadata: &http.Client{Timeout: metadataTimeout}, download: &http.Client{Timeout: downloadTimeout},
		releaseAPI: releaseAPI, goos: runtime.GOOS, goarch: runtime.GOARCH,
	}
	s.calls = s.routes()
	return s
}

// SetLanguage names where the interface language an extension starts with
// comes from, before Boot. Without one it starts with English.
func (s *Service) SetLanguage(language func() string) { s.language = language }

func (s *Service) startLanguage() string {
	if s.language == nil {
		return exti18n.Source
	}
	return s.language()
}

// Boot deletes what earlier runs left staged, then discovers the extensions
// and starts every valid one.
func (s *Service) Boot() {
	sweepStaged(s.dir)
	found := discover(s.dir, s.lichVersion)
	s.mu.Lock()
	s.all = found
	s.mu.Unlock()
	for _, x := range found {
		if x.invalid != nil {
			continue
		}
		s.mu.Lock()
		s.launch(x)
		s.mu.Unlock()
	}
}

// launch starts x's supervise goroutine. Called with mu held, which is what
// keeps wg.Add from racing the wg.Wait in Shutdown.
func (s *Service) launch(x *installed) {
	ctx, stop := context.WithCancel(s.ctx)
	done := make(chan struct{})
	x.stop, x.done = stop, done
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer close(done)
		s.supervise(ctx, x)
	}()
}

// Shutdown stops every extension: stdin closes, each gets its grace, and what
// is still up is killed. Bounded, so nothing here can hold up lich's exit.
func (s *Service) Shutdown() {
	s.mu.Lock()
	s.cancel()
	s.mu.Unlock()
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(s.timing.stopGrace + s.timing.killWait):
		slog.Warn("extensions: still running after the kill")
	}
}

// List is every installed extension with its state, by directory name.
func (s *Service) List(_ context.Context, _ api.None) ([]api.Extension, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]api.Extension, 0, len(s.all))
	for _, x := range s.all {
		out = append(out, x.view())
	}
	return out, nil
}

// Send writes one message to an extension's process.
func (s *Service) Send(_ context.Context, opts api.ExtensionSendOptions) (api.None, error) {
	line, err := encodeOutbound(opts.Message)
	if err != nil {
		return api.None{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x := s.find(opts.ID)
	if x == nil {
		return api.None{}, fmt.Errorf("no extension %q", opts.ID)
	}
	if x.queue == nil {
		return api.None{}, fmt.Errorf("extension %q is not running", opts.ID)
	}
	select {
	case x.queue <- line:
		return api.None{}, nil
	default:
		return api.None{}, fmt.Errorf("extension %q is not reading its input", opts.ID)
	}
}

// Restart starts an extension again, which is the only way back from failed,
// and starts its count of quick crashes over. One that is running or waiting
// to restart is stopped first, as lich stops it on exit.
func (s *Service) Restart(ctx context.Context, opts api.ExtensionIDOptions) (api.None, error) {
	s.mu.Lock()
	x := s.find(opts.ID)
	s.mu.Unlock()
	if x == nil {
		return api.None{}, fmt.Errorf("no extension %q", opts.ID)
	}
	if x.invalid != nil {
		return api.None{}, fmt.Errorf("extension %q is invalid and cannot start: %w", opts.ID, x.invalid)
	}
	x.restartMu.Lock()
	defer x.restartMu.Unlock()
	s.mu.Lock()
	stop, done := x.stop, x.done
	s.mu.Unlock()
	if stop != nil {
		stop()
		select {
		case <-done:
		case <-ctx.Done():
			return api.None{}, ctx.Err()
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil {
		return api.None{}, errors.New("lich is shutting down")
	}
	if s.find(opts.ID) != x {
		return api.None{}, fmt.Errorf("extension %q was removed", opts.ID)
	}
	s.launch(x)
	return api.None{}, nil
}

// find returns the extension with id, or nil. Called with mu held.
func (s *Service) find(id string) *installed {
	for _, x := range s.all {
		if x.id == id {
			return x
		}
	}
	return nil
}

// view is the extension as the api shows it. Called with Service.mu held.
func (x *installed) view() api.Extension {
	surfaces := make([]api.ExtensionSurface, 0, len(x.manifest.Surfaces))
	for _, surface := range x.manifest.Surfaces {
		surfaces = append(surfaces, api.ExtensionSurface{ID: surface.ID, Title: surface.Title, Icon: string(surface.Icon)})
	}
	return api.Extension{
		ID: x.id, Name: cmp.Or(x.manifest.Name, x.id), Version: x.manifest.Version,
		Status: x.status, Error: x.errText, Dir: x.root, Surfaces: surfaces,
		Source: x.source.URL, Release: x.source.Release, Text: x.text, Permissions: permissionNames(x.manifest),
	}
}

func permissionNames(m extension.Manifest) []string {
	out := make([]string, 0, len(m.Permissions))
	for _, p := range m.Permissions {
		out = append(out, string(p))
	}
	return out
}

// setStatus records a state and tells the window, once it is true.
func (s *Service) setStatus(x *installed, status, errText string) {
	s.mu.Lock()
	x.status, x.errText = status, errText
	view := x.view()
	s.mu.Unlock()
	events.Send(s.emit, api.ExtensionStatusEvent, view)
}

func (s *Service) deliver(x *installed, message json.RawMessage) {
	events.Send(s.emit, api.ExtensionMessageEvent, api.ExtensionMessage{ExtensionID: x.id, Message: message})
}

// ExtensionsDir is the directory extensions are read from and installed into.
func (s *Service) ExtensionsDir(context.Context, api.None) (string, error) {
	return s.dir, nil
}
