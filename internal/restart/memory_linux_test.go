package restart

import (
	"os"
	"testing"
)

func TestParseMemoryReadsResidentAndPeak(t *testing.T) {
	status := []byte("Name:\tlich-native\nVmHWM:\t  812340 kB\nVmRSS:\t  604112 kB\nThreads:\t22\n")
	got, ok := parseMemory(status)
	if !ok || got != (memory{rss: "604112 kB", hwm: "812340 kB"}) {
		t.Errorf("parseMemory = %+v, %v", got, ok)
	}
}

// TestParseMemoryOfAZombieHasNone proves an exited, unreaped process, whose
// status carries no Vm lines, does not replace the last real reading.
func TestParseMemoryOfAZombieHasNone(t *testing.T) {
	if _, ok := parseMemory([]byte("Name:\tlich-native\nState:\tZ (zombie)\n")); ok {
		t.Error("a zombie's status read as a memory sample")
	}
}

func TestReadMemoryOfThisProcess(t *testing.T) {
	if m, ok := readMemory(os.Getpid()); !ok || m.rss == "" {
		t.Errorf("readMemory(self) = %+v, %v", m, ok)
	}
}
