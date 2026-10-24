package terminal

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"runtime"
	"slices"
	"time"
	"unicode/utf16"
)

// KindService marks a PTY that runs a project service's command
// (internal/services) instead of a session: it has no row, no provider and no
// hooks, and it is not counted as a session anywhere.
const KindService = "service"

// interruptByte is what Ctrl+C types: the PTY's line discipline turns it into
// SIGINT for the foreground process group, and ConPTY into CTRL_C_EVENT.
const interruptByte = 0x03

// ServiceCommand is one run of a service's command on a PTY of its own.
// Interrupted is told when the user types Ctrl+C at its console, so the exit
// that follows reads as a stop they asked for. Exited is told once the process
// is reaped, with its exit status (negative when it left none) and the tail of
// what it printed, which is all that is left of its console afterwards.
type ServiceCommand struct {
	ID          string
	ProjectID   string
	Cwd         string
	Line        string
	Interrupted func()
	Exited      func(code int, tail []byte)
}

// StartService runs c.Line in the user's shell on a new PTY registered under
// c.ID, which the window reaches through the same Write, Resize, Replay and
// SetVisible a session's terminal takes. The command runs on its own: no rc
// file is loaded and no shell is left behind it, so its exit is the service's
// exit. Starting an id already running is an error.
func (s *Service) StartService(c ServiceCommand) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, running := s.sessions[c.ID]; running {
		return fmt.Errorf("service %q is already running", c.ID)
	}
	cols, rows := s.sizeFor(0, 0)
	shell := resolveCommand(KindShell, "", userShell())
	env := slices.Clip(s.env)
	if dir := s.store.ProjectPath(c.ProjectID); dir != "" {
		env = append(env, "LICH_PROJECT_DIR="+dir)
	}
	spec := ptySpec{bin: shell, args: serviceArgs(c.Line, runtime.GOOS), dir: c.Cwd, env: env, cols: cols, rows: rows}
	header, err := encodeFrame(c.ID, nil)
	if err != nil {
		return fmt.Errorf("start service %q: %w", c.ID, err)
	}
	p, err := startPTY(spec)
	if err != nil {
		return fmt.Errorf("start service %q: %w", c.ID, err)
	}
	box := newOutbox(header, func(frame []byte) {
		if s.ws != nil && s.ws.send(frame) {
			return
		}
		s.emitData(c.ID, frame[len(header):])
	}, func(tail []byte) {
		if s.ws != nil {
			s.ws.sendReplay(c.ID, tail)
		}
	}, outboxDepth)
	sess := &session{
		pty:     p,
		out:     newCoalescer(box.push, visibleFlushInterval, hiddenFlushInterval),
		done:    make(chan struct{}),
		replay:  newReplayBuffer(replayCapBytes),
		outbox:  box,
		lastOut: time.Now(),
		service: &c,
	}
	s.sessions[c.ID] = sess
	s.spawns.Store(c.ID, spawn{kind: KindService, cwd: c.Cwd, projectID: c.ProjectID})
	go s.stream(c.ID, sess)
	return nil
}

// InterruptService types Ctrl+C at service id's console, the way the user
// would: the command is asked to stop and decides how long that takes. A
// service that is not running is a no-op.
func (s *Service) InterruptService(id string) error {
	return s.writeBytes(id, []byte{interruptByte})
}

// KillService ends service id's process without waiting for it to agree:
// SIGTERM, then SIGKILL past closeGrace (ptyHandle.Close). It returns at once;
// the exit reaches ServiceCommand.Exited like any other. A service that is not
// running is a no-op.
func (s *Service) KillService(id string) {
	if p := s.ptyOf(id); p != nil {
		go func() { _ = p.Close() }()
	}
}

// serviceInput is onInput for a service's console: typed bytes go straight to
// the PTY, none of a session's bookkeeping applies, and a Ctrl+C among them
// tells the service it was asked to stop. It reports whether id is a service.
func (s *Service) serviceInput(id string, data []byte) bool {
	s.mu.Lock()
	sess, ok := s.sessions[id]
	s.mu.Unlock()
	if !ok || sess.service == nil {
		return false
	}
	if bytes.IndexByte(data, interruptByte) >= 0 && sess.service.Interrupted != nil {
		sess.service.Interrupted()
	}
	_ = s.writeBytes(id, data)
	return true
}

// serviceArgs is the argv after the shell that runs a service's line. goos is
// runtime.GOOS, passed in so both branches are testable anywhere. A POSIX
// shell's -c loads no interactive rc; PowerShell would load $PROFILE unless
// told not to, and its command goes base64 so a quote the user typed survives
// windows.ComposeCommandLine, which escapes it as \" and PowerShell does not
// read that back.
func serviceArgs(line, goos string) []string {
	if goos == "windows" {
		return []string{"-NoProfile", "-EncodedCommand", encodePwshCommand(line)}
	}
	return []string{"-c", line}
}

// encodePwshCommand renders a script as the base64 of its UTF-16LE bytes, which
// is what PowerShell's -EncodedCommand takes.
func encodePwshCommand(script string) string {
	units := utf16.Encode([]rune(script))
	buf := make([]byte, 2*len(units))
	for i, unit := range units {
		binary.LittleEndian.PutUint16(buf[2*i:], unit)
	}
	return base64.StdEncoding.EncodeToString(buf)
}
