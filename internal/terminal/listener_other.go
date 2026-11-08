//go:build !unix

package terminal

import "net"

type stopListening struct{ *net.TCPListener }
