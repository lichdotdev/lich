//go:build unix

package terminal

import (
	"errors"
	"net"
	"syscall"
)

// stopListening takes the socket out of LISTEN before closing its fd. A
// fork/exec running anywhere in this process copies the fd table and holds the
// fd until its exec, so a bare close leaves the port taken for that window and
// a relaunch's bind fails; shutdown acts on the socket, whoever holds an fd.
type stopListening struct{ *net.TCPListener }

func (l stopListening) Close() error {
	var shutErr error
	rc, err := l.SyscallConn()
	if err == nil {
		err = rc.Control(func(fd uintptr) { shutErr = syscall.Shutdown(int(fd), syscall.SHUT_RD) })
	}
	return errors.Join(err, shutErr, l.TCPListener.Close())
}
