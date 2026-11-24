package main

// The window's one embedded Neovim, behind every editor while Settings ›
// Editor › Vim is on: Neovim runs the keys, so every command is Vim's own,
// and lich draws what it reports. Calls run in order on one goroutine, off
// the UI's; what Neovim reports waits per buffer for the editor's next frame.

import (
	_ "embed"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"sync"

	"github.com/neovim/go-client/nvim"
)

//go:embed vim.lua
var vimLua string

// vimMinVersion is the oldest Neovim the editor runs: vim.lua reads the
// visual region with getregionpos, which came in 0.10.
const vimMinVersion = "nvim-0.10"

// errVimTooOld is a Neovim older than vimMinVersion on the PATH.
var errVimTooOld = errors.New("Neovim 0.10 or newer is needed")

// vimRowsFloor is the grid's height before an editor has measured its own.
const vimRowsFloor, vimColsFloor = 24, 80

// vimQueue is how many calls wait for Neovim before more are dropped: a
// Neovim that stops answering must not stop the window.
const vimQueue = 1024

type vimHost struct {
	invalidate func()
	jobs       chan func()
	// args start Neovim; tests add --clean to leave the person's config out.
	args []string

	mu sync.Mutex
	nv *nvim.Nvim
	// gen counts the processes started: a buffer opened in an older one is
	// gone, and its editor opens it again.
	gen    int
	failed error
	// told is whether the editors already said that Neovim failed to start.
	told   bool
	events map[int][]vimEvent
	ui     vimUI
	rows   int
	cols   int
}

// vimEvent is what Neovim reported of one buffer: its lines from first to
// last replaced by lines, at tick; its cursor and mode; a :w; or a :q.
type vimEvent struct {
	lines *vimLines
	state *vimState
	write bool
	// quit is a :q and its kin; discard drops what is not saved.
	quit, discard bool
}

type vimLines struct {
	tick, first, last int
	lines             []string
}

// vimState is the cursor, 1-based row and byte column, the mode as
// nvim_get_mode spells it, the window's top line, and the visual region as
// rows and 1-based byte columns, both ends inside it.
type vimState struct {
	mode     string
	row, col int
	top      int
	regions  [][4]int
	// matches are the highlighted search's visible matches: row, byte
	// column, byte length.
	matches [][3]int
}

func newVimHost(invalidate func()) *vimHost {
	h := &vimHost{invalidate: invalidate, jobs: make(chan func(), vimQueue), args: []string{"--embed"}, events: map[int][]vimEvent{},
		rows: vimRowsFloor, cols: vimColsFloor}
	go func() {
		for job := range h.jobs {
			job()
		}
	}()
	return h
}

// vimAvailable is whether a Neovim the editor can run is on the PATH.
func vimAvailable() error {
	if _, err := exec.LookPath(nvim.BinaryName); err != nil {
		return err
	}
	out, err := exec.Command(nvim.BinaryName, "--headless", "--clean", "-c", fmt.Sprintf("echo has('%s')", vimMinVersion), "-c", "qa!").CombinedOutput()
	if err != nil {
		return err
	}
	if len(out) == 0 || out[0] != '1' {
		return errVimTooOld
	}
	return nil
}

// do runs call against the running Neovim, starting one first if none is.
// A call that fails because Neovim went away drops it; the next starts anew.
func (h *vimHost) do(call func(v *nvim.Nvim, gen int) error) {
	job := func() {
		v, gen, err := h.running()
		if err != nil {
			return
		}
		if err := call(v, gen); err != nil {
			log.Printf("vim: %v", err)
			h.drop(v)
		}
	}
	select {
	case h.jobs <- job:
	default:
		log.Printf("vim: Neovim is %d calls behind, dropping one", vimQueue)
	}
}

func (h *vimHost) running() (*nvim.Nvim, int, error) {
	h.mu.Lock()
	v, gen := h.nv, h.gen
	h.mu.Unlock()
	if v != nil {
		return v, gen, nil
	}
	v, err := h.start()
	h.mu.Lock()
	defer h.mu.Unlock()
	h.failed = err
	if err != nil {
		return nil, 0, err
	}
	h.nv = v
	h.gen++
	return v, h.gen, nil
}

func (h *vimHost) start() (*nvim.Nvim, error) {
	v, err := nvim.NewChildProcess(nvim.ChildProcessArgs(h.args...))
	if err != nil {
		return nil, err
	}
	handlers := map[string]any{
		"redraw":     h.redraw,
		"lich_lines": h.onLines,
		"lich_state": h.onState,
		"lich_write": func(buf int) { h.push(buf, vimEvent{write: true}) },
		"lich_quit":  func(buf int, discard bool) { h.push(buf, vimEvent{quit: true, discard: discard}) },
	}
	for name, fn := range handlers {
		if err := v.RegisterHandler(name, fn); err != nil {
			_ = v.Close()
			return nil, err
		}
	}
	h.mu.Lock()
	rows, cols := h.rows, h.cols
	h.mu.Unlock()
	if err := v.AttachUI(cols, rows, map[string]any{"ext_linegrid": true, "ext_messages": true, "ext_cmdline": true, "rgb": true}); err != nil {
		_ = v.Close()
		return nil, err
	}
	if err := v.ExecLua(vimLua, nil, v.ChannelID()); err != nil {
		_ = v.Close()
		return nil, err
	}
	if err := fitUI(v, rows, cols); err != nil {
		_ = v.Close()
		return nil, err
	}
	return v, nil
}

// drop forgets v once a call on it failed, and every buffer it held.
func (h *vimHost) drop(v *nvim.Nvim) {
	h.mu.Lock()
	if h.nv == v {
		h.nv, h.events, h.ui = nil, map[int][]vimEvent{}, vimUI{}
	}
	h.mu.Unlock()
	_ = v.Close()
	h.invalidate()
}

// close ends the Neovim, as the window closes or Vim is turned off.
func (h *vimHost) close() {
	h.mu.Lock()
	v := h.nv
	h.failed, h.told = nil, false
	h.mu.Unlock()
	if v != nil {
		h.drop(v)
	}
}

func (h *vimHost) push(buf int, ev vimEvent) {
	h.mu.Lock()
	h.events[buf] = append(h.events[buf], ev)
	h.mu.Unlock()
	h.invalidate()
}

func (h *vimHost) onLines(buf int, tick, first, last int, lines []string) {
	h.push(buf, vimEvent{lines: &vimLines{tick: tick, first: first, last: last, lines: lines}})
}

func (h *vimHost) onState(buf int, mode string, row, col, top int, regions, matches [][]int) {
	st := &vimState{mode: mode, row: row, col: col, top: top}
	for _, r := range regions {
		if len(r) == 4 {
			st.regions = append(st.regions, [4]int{r[0], r[1], r[2], r[3]})
		}
	}
	for _, m := range matches {
		if len(m) == 3 {
			st.matches = append(st.matches, [3]int{m[0], m[1], m[2]})
		}
	}
	h.mu.Lock()
	h.ui.buf = buf
	h.mu.Unlock()
	h.push(buf, vimEvent{state: st})
}

// take is what Neovim reported of buf since the last take, in order, and
// the process generation it came from.
func (h *vimHost) take(buf int) ([]vimEvent, int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	evs := h.events[buf]
	delete(h.events, buf)
	return evs, h.gen
}

// status is the strip under the editor holding buf: Neovim's mode, partial
// command, command line and last message while buf is the one it shows.
func (h *vimHost) status(buf int) (vimUI, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.ui, h.nv != nil && h.ui.buf == buf
}

// startErr is why Neovim last failed to start, nil once one runs; tell is
// set the first time it is asked about a failure, for the one toast.
func (h *vimHost) startErr() (err error, tell bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	tell = h.failed != nil && !h.told
	h.told = h.told || h.failed != nil
	return h.failed, tell
}

// resize fits the grid to an editor of rows by cols cells, the status and
// command lines added, so Vim's window is the editor's.
func (h *vimHost) resize(rows, cols int) {
	h.mu.Lock()
	same := rows == h.rows && cols == h.cols
	h.rows, h.cols = rows, cols
	h.mu.Unlock()
	if same {
		return
	}
	h.do(func(v *nvim.Nvim, _ int) error { return fitUI(v, rows, cols) })
}

func fitUI(v *nvim.Nvim, rows, cols int) error {
	var extra int
	if err := v.ExecLua("return lich_editor.extra()", &extra); err != nil {
		return err
	}
	return v.TryResizeUI(cols, rows+extra)
}
