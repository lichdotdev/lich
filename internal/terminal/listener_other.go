//go:build !linux

package terminal

import "net"

type stopListening struct{ *net.TCPListener }
