//go:build !windows

package terminal

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
)

// A payload with a "\n" at every offset 1..600 apart, written while nobody
// reads the pty for 300ms so the tty output queue fills; counts the "\r\r\n"
// a bare "\n" should never turn into.
func TestZZDiagSlowReader(t *testing.T) {
	var b strings.Builder
	for n := 1; n <= 600; n++ {
		b.WriteString(strings.Repeat("x", n))
		b.WriteString("\n")
	}
	payload := b.String()
	for _, pre := range []string{"", "stty -opost; "} {
		for round := 0; round < 3; round++ {
			cmd := exec.Command("/bin/sh", "-c", pre+`printf '%s' "$P"`)
			cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "P=" + payload}
			ptmx, err := pty.Start(cmd)
			if err != nil {
				t.Fatal(err)
			}
			time.Sleep(300 * time.Millisecond)
			var out bytes.Buffer
			_, _ = io.Copy(&out, ptmx)
			_ = cmd.Wait()
			ptmx.Close()
			s := out.String()
			t.Logf("pre=%q round=%d bytes=%d lf=%d crlf=%d dup=%d", pre, round, len(s), strings.Count(s, "\n"), strings.Count(s, "\r\n"), strings.Count(s, "\r\r\n"))
		}
	}
}
