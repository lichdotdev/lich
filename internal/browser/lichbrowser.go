package browser

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/lichdotdev/lich/browserwire"
	"github.com/lichdotdev/lich/internal/logging"
)

// BrowserEnv pins the lich-browser to start, by path, as restart.NativeEnv
// pins the window.
const BrowserEnv = "LICH_BROWSER"

// helloWait bounds lich-browser's start, up to its answer to hello: a GTK
// and WebKit cold off the disk, well under pageWait.
const helloWait = 15 * time.Second

// lichBrowser is the lich-browser process every page lives in, started at the
// first page and again at the first page after it ended. A page belongs to the
// process it opened in and dies with it (docs/lich-browser.md).
type lichBrowser struct {
	// spawn starts a process and connects to it; tests hand a pipe.
	spawn func() (*wireConn, error)
	// gone runs, on a goroutine of its own, when pages died without a Close:
	// the process ended, or a page's web process crashed.
	gone func()

	mu     sync.Mutex
	conn   *wireConn
	window uint64
}

// connect is the running process, started first when there is none.
func (b *lichBrowser) connect() (*wireConn, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.conn != nil && b.conn.alive() {
		return b.conn, nil
	}
	c, err := b.spawn()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), helloWait)
	defer cancel()
	if err := hello(ctx, c); err != nil {
		c.fail(err)
		return nil, err
	}
	if b.window != 0 {
		if err := c.call(ctx, browserwire.Window, browserwire.WindowParams{XID: b.window}, nil); err != nil {
			slog.Warn("browser: tell lich-browser the window", "err", err)
		}
	}
	b.conn = c
	return c, nil
}

func hello(ctx context.Context, c *wireConn) error {
	var h browserwire.HelloResult
	if err := c.call(ctx, browserwire.Hello, nil, &h); err != nil {
		return fmt.Errorf("start lich-browser: %w", err)
	}
	if h.Version != browserwire.Version {
		return fmt.Errorf("lich-browser speaks version %d of the wire and this lich %d: install the lich-browser built with this lich", h.Version, browserwire.Version)
	}
	return nil
}

// setWindow keeps the window's X window, 0 once it closed, for this process
// and the next.
func (b *lichBrowser) setWindow(ctx context.Context, xid uint64) error {
	b.mu.Lock()
	b.window = xid
	c := b.conn
	b.mu.Unlock()
	if c == nil || !c.alive() {
		return nil
	}
	return c.call(ctx, browserwire.Window, browserwire.WindowParams{XID: xid}, nil)
}

// stop ends the process, once Cleanup closed every page.
func (b *lichBrowser) stop() {
	b.mu.Lock()
	c := b.conn
	b.conn = nil
	b.mu.Unlock()
	if c != nil {
		c.fail(io.EOF)
	}
}

// launch is the Launcher of a page in lich-browser, in a window of its own
// when headed. The profile dir is the service's to delete: each page's
// context is lich-browser's own and ends with it.
func (b *lichBrowser) launch(_ string, headed bool, scale float64) (Page, error) {
	c, err := b.connect()
	if err != nil {
		return nil, err
	}
	p := newWebkitPage(c, newID(), scale, b.gone)
	ctx, cancel := context.WithTimeout(context.Background(), pageWait)
	defer cancel()
	c.add(p)
	open := browserwire.OpenParams{Page: p.id, URL: "about:blank", Scale: scale, W: layoutW, H: layoutH, Downloads: downloadsFor()}
	if err := c.call(ctx, browserwire.Open, open, nil); err != nil {
		c.remove(p)
		return nil, err
	}
	if headed {
		if err := p.PopOut(ctx); err != nil {
			return nil, errors.Join(err, p.Close())
		}
	}
	return p, nil
}

// downloadsFor is the folder a page's downloads land in, the one Chrome's
// do; "" when there is none, which leaves them to lich-browser.
func downloadsFor() string {
	dir, err := downloadFolder()
	if err == nil {
		err = os.MkdirAll(dir, 0o755)
	}
	if err != nil {
		slog.Warn("browser: downloads stay off", "err", err)
		return ""
	}
	return dir
}

// layoutW and layoutH are the size a page opens at before the service lays it
// out at the window's, which it does before loading anything.
const (
	layoutW = 1280
	layoutH = 800
)

// spawnProcess starts the lich-browser at path, its stderr appended to its
// log under logDir.
func spawnProcess(path, logDir string) (*wireConn, error) {
	logFile, err := logging.OpenBrowser(logDir)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(path)
	cmd.Stderr = logFile
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, errors.Join(err, logFile.Close())
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Join(err, logFile.Close())
	}
	if err := cmd.Start(); err != nil {
		return nil, errors.Join(fmt.Errorf("start lich-browser: %w", err), logFile.Close())
	}
	var once sync.Once
	stop := func() {
		once.Do(func() {
			_ = stdin.Close()
			_ = cmd.Process.Kill()
			if err := cmd.Wait(); err != nil {
				slog.Info("browser: lich-browser ended", "err", err)
			}
			_ = logFile.Close()
		})
	}
	return newWireConn(stdout, stdin, func() { go stop() }), nil
}
