package appkit

import (
	"log/slog"
	"os"
	"runtime"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// AppKit runs the main thread's loop only if the main goroutine still holds
// that thread when Run is called, and an init is the one place to pin it.
func init() {
	runtime.LockOSThread()
}

const appKitPath = "/System/Library/Frameworks/AppKit.framework/AppKit"

// NSApplicationActivationPolicyAccessory keeps the backend off the Dock and
// out of Cmd-Tab: the window, lich-native, is the app a person sees there.
const activationPolicyAccessory = 1

// NSTerminateLater holds the logout until lich has quit on its own, through
// the same quit `lich quit` runs, instead of AppKit's exit skipping it.
const terminateLater = 2

var (
	mu        sync.Mutex
	reopen    func()
	terminate func()
	queue     []func()
	// runner is the object Do hands its queue to on the main thread; zero
	// until Run brought AppKit up.
	runner objc.ID
)

// Handle sets what a reopen (lich opened again while it runs) and a terminate
// (logout, shutdown) do. Either one arriving before Handle is dropped.
func Handle(onReopen, onTerminate func()) {
	mu.Lock()
	defer mu.Unlock()
	reopen, terminate = onReopen, onTerminate
}

// Run runs app beside AppKit's loop and exits when app returns. A Mac where
// AppKit cannot load runs app alone, with no reopen and no menu-bar item.
func Run(app func()) {
	nsApp, err := start()
	if err != nil {
		slog.Warn("appkit unavailable: no reopen and no menu-bar item", "err", err)
		app()
		return
	}
	go func() {
		app()
		os.Exit(0)
	}()
	nsApp.Send(objc.RegisterName("run"))
}

// Do runs fn on the main thread, where every AppKit call has to be made. It
// does not wait for fn.
func Do(fn func()) {
	mu.Lock()
	r := runner
	if r != 0 {
		queue = append(queue, fn)
	}
	mu.Unlock()
	if r == 0 {
		slog.Warn("appkit: not running, dropped a main-thread call")
		return
	}
	r.Send(objc.RegisterName("performSelectorOnMainThread:withObject:waitUntilDone:"),
		objc.RegisterName("drain:"), objc.ID(0), false)
}

func start() (objc.ID, error) {
	if _, err := purego.Dlopen(appKitPath, purego.RTLD_GLOBAL|purego.RTLD_LAZY); err != nil {
		return 0, err
	}
	class, err := objc.RegisterClass("LichAppDelegate", objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{
		{Cmd: objc.RegisterName("applicationShouldHandleReopen:hasVisibleWindows:"), Fn: handleReopen},
		{Cmd: objc.RegisterName("applicationShouldTerminate:"), Fn: shouldTerminate},
		{Cmd: objc.RegisterName("drain:"), Fn: drain},
	})
	if err != nil {
		return 0, err
	}
	delegate := objc.ID(class).Send(objc.RegisterName("new"))
	nsApp := objc.ID(objc.GetClass("NSApplication")).Send(objc.RegisterName("sharedApplication"))
	nsApp.Send(objc.RegisterName("setActivationPolicy:"), activationPolicyAccessory)
	nsApp.Send(objc.RegisterName("setDelegate:"), delegate)
	mu.Lock()
	runner = delegate
	mu.Unlock()
	return nsApp, nil
}

// handleReopen answers NO: lich opens its window itself, off the main thread,
// since opening one may start a process.
func handleReopen(_ objc.ID, _ objc.SEL, _ objc.ID, _ bool) bool {
	mu.Lock()
	fn := reopen
	mu.Unlock()
	if fn != nil {
		go fn()
	}
	return false
}

func shouldTerminate(_ objc.ID, _ objc.SEL, _ objc.ID) uint {
	mu.Lock()
	fn := terminate
	mu.Unlock()
	if fn != nil {
		go fn()
	}
	return terminateLater
}

func drain(_ objc.ID, _ objc.SEL, _ objc.ID) {
	mu.Lock()
	fns := queue
	queue = nil
	mu.Unlock()
	for _, fn := range fns {
		fn()
	}
}
