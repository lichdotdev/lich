package ui

import (
	"image/color"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/omartelo/lich/native/ui/icons"
)

// ToastVariant is which sonner call raised the toast: toast(),
// toast.success(), toast.info(), toast.warning() or toast.error().
type ToastVariant uint8

const (
	ToastDefault ToastVariant = iota
	ToastSuccess
	ToastInfo
	ToastWarning
	ToastError
)

// ToastButton is a toast's action or cancel button. OnClick runs on the
// frame's goroutine after the toast has been dismissed; nil only dismisses.
type ToastButton struct {
	Label   string
	OnClick func()
}

// ToastLine is one muted line under a toast's title, with an optional glyph
// before it.
type ToastLine struct {
	Icon *icons.Icon
	Text string
}

// ToastForever is a Duration that never runs out, sonner's Infinity.
const ToastForever time.Duration = math.MaxInt64

// Toast is what sonner's toast() takes: a title, the optional description
// line under it, the action and cancel buttons, and how long it stays.
type Toast struct {
	// ID names the toast; showing one whose ID is up replaces it in place.
	// Empty draws a fresh one.
	ID          string
	Variant     ToastVariant
	Title       string
	Description string
	Action      *ToastButton
	Cancel      *ToastButton
	// Secondary sits between Cancel and Action, drawn as Cancel is: the
	// web's custom update toast, whose third choice sonner has no slot for.
	Secondary *ToastButton
	// Icon replaces the variant's glyph, drawn in the color IconColor picks
	// from the theme the toast is drawn with, the foreground when nil:
	// sonner's icon option.
	Icon      *icons.Icon
	IconColor func(*Theme) color.NRGBA
	// Lines sit under the title in the muted foreground, one truncated line
	// each, as a toast whose title is a block of its own draws them.
	Lines []ToastLine
	// Duration is how long the toast stays while the pointer is away; 0 is
	// sonner's default.
	Duration time.Duration
}

// sonner 2.0.7's constants (dist/index.mjs).
const (
	toastLifetime      = 4000 * time.Millisecond // TOAST_LIFETIME
	toastUnmountDelay  = 200 * time.Millisecond  // TIME_BEFORE_UNMOUNT
	toastVisibleAmount = 3                       // VISIBLE_TOASTS_AMOUNT
)

// toastEntry is one toast on the stack: what it says, its timer, and the
// state its drawing keeps from frame to frame.
type toastEntry struct {
	Toast
	// remaining is how long the timer has left as of runningSince, or as of
	// now while paused (runningSince zero).
	remaining    time.Duration
	runningSince time.Time
	// removedAt is when it was dismissed; it stays on the stack for its
	// exit until toastUnmountDelay later.
	removedAt time.Time
	view      toastView
}

func (e *toastEntry) removed() bool { return !e.removedAt.IsZero() }

func (e *toastEntry) duration() time.Duration {
	if e.Duration == 0 {
		return toastLifetime
	}
	return e.Duration
}

// startTimer runs the full duration from now, or holds it while paused.
func (e *toastEntry) startTimer(now time.Time, paused bool) {
	e.remaining, e.runningSince = e.duration(), time.Time{}
	if !paused {
		e.runningSince = now
	}
}

// deadline is when the timer runs out; zero while paused or forever.
func (e *toastEntry) deadline() time.Time {
	if e.runningSince.IsZero() || e.remaining == ToastForever {
		return time.Time{}
	}
	return e.runningSince.Add(e.remaining)
}

// toastStack is sonner's toast list, front (newest) first, with its timers.
// It holds no lock; Toaster does.
type toastStack struct {
	entries []*toastEntry
	paused  bool
	seq     int
}

// show puts t at the front, or over the live toast with its ID, where it
// keeps its place and restarts its timer. It returns the toast's ID.
func (s *toastStack) show(t Toast, now time.Time) string {
	if t.ID == "" {
		s.seq++
		t.ID = "toast-" + strconv.Itoa(s.seq)
	}
	if e := s.live(t.ID); e != nil {
		e.Toast = t
		e.startTimer(now, s.paused)
		return t.ID
	}
	e := &toastEntry{Toast: t}
	e.startTimer(now, s.paused)
	s.entries = append([]*toastEntry{e}, s.entries...)
	return t.ID
}

// live is the toast with id that is not on its way out, or nil.
func (s *toastStack) live(id string) *toastEntry {
	for _, e := range s.entries {
		if e.ID == id && !e.removed() {
			return e
		}
	}
	return nil
}

// dismiss starts the exit of the live toast with id, if there is one.
func (s *toastStack) dismiss(id string, now time.Time) {
	if e := s.live(id); e != nil {
		e.removedAt = now
	}
}

// pause holds or resumes every timer: sonner pauses them all while the
// stack is hovered or pressed, or the page is hidden.
func (s *toastStack) pause(paused bool, now time.Time) {
	if paused == s.paused {
		return
	}
	s.paused = paused
	for _, e := range s.entries {
		switch {
		case e.remaining == ToastForever:
		case paused && !e.runningSince.IsZero():
			e.remaining -= now.Sub(e.runningSince)
			e.runningSince = time.Time{}
		case !paused:
			e.runningSince = now
		}
	}
}

// advance dismisses the toasts whose timer ran out by now and drops the
// ones whose exit is over. It returns when it next has something to do,
// zero for never.
func (s *toastStack) advance(now time.Time) time.Time {
	var wake time.Time
	soonest := func(t time.Time) {
		if !t.IsZero() && (wake.IsZero() || t.Before(wake)) {
			wake = t
		}
	}
	kept := s.entries[:0]
	for _, e := range s.entries {
		if d := e.deadline(); !e.removed() && !d.IsZero() && !now.Before(d) {
			e.removedAt = now
		}
		if e.removed() {
			gone := e.removedAt.Add(toastUnmountDelay)
			if !now.Before(gone) {
				continue
			}
			soonest(gone)
		} else {
			soonest(e.deadline())
		}
		kept = append(kept, e)
	}
	clear(s.entries[len(kept):])
	s.entries = kept
	return wake
}

// visible is sonner's isVisible: only the front visibleToasts are drawn,
// counting the ones on their way out.
func visible(index int) bool { return index < toastVisibleAmount }

// Toaster is sonner's Toaster: the stack of toasts the window shows. Show
// and Dismiss are safe from any goroutine; draw it with Theme.Toaster.
type Toaster struct {
	// Invalidate asks the window for a frame; Show and Dismiss call it.
	Invalidate func()

	mu    sync.Mutex
	stack toastStack
	// hidden is the window being unfocused or minimized, sonner's
	// document.hidden.
	hidden  bool
	hovered bool
	pressed bool
}

// Show raises t, or replaces the toast with its ID, and returns that ID.
func (t *Toaster) Show(toast Toast) string {
	t.mu.Lock()
	id := t.stack.show(toast, time.Now())
	t.mu.Unlock()
	t.invalidate()
	return id
}

// Dismiss sends the toast with id out, as its close button would.
func (t *Toaster) Dismiss(id string) {
	t.mu.Lock()
	t.stack.dismiss(id, time.Now())
	t.mu.Unlock()
	t.invalidate()
}

// SetHidden holds the timers while the window is out of sight or focus,
// so a toast is not spent on nobody.
func (t *Toaster) SetHidden(hidden bool) {
	t.mu.Lock()
	t.hidden = hidden
	t.mu.Unlock()
	t.invalidate()
}

func (t *Toaster) invalidate() {
	if t.Invalidate != nil {
		t.Invalidate()
	}
}
