package terminal

import (
	"bufio"
	"io"
	"os"
	"sync"
)

// rereadEntry is how far one transcript has been summed for cache-read tokens:
// where to resume and the total up to there.
type rereadEntry struct {
	offset      int64
	lastMessage string
	total       int
}

// rereadCounts totals the tokens a session's calls read back from the prompt
// cache, per transcript, in memory. It is not the cost ledger: that one only
// runs with the cost readout on, and the readout is off on a subscription,
// which is where this figure matters. A restart of lich rescans each transcript
// once from zero.
type rereadCounts struct {
	mu   sync.Mutex
	byID map[string]map[string]rereadEntry
}

// total is the cache-read tokens of session id across the conversation's
// transcript and its sub-agents', scanning only what each file gained since the
// last call.
func (r *rereadCounts) total(id string, paths []string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.byID == nil {
		r.byID = map[string]map[string]rereadEntry{}
	}
	if r.byID[id] == nil {
		r.byID[id] = map[string]rereadEntry{}
	}
	sum := 0
	for _, path := range paths {
		entry := scanCacheRead(path, r.byID[id][path])
		r.byID[id][path] = entry
		sum += entry.total
	}
	return sum
}

func (r *rereadCounts) forget(id string) {
	r.mu.Lock()
	delete(r.byID, id)
	r.mu.Unlock()
}

// scanCacheRead resumes a transcript's sum from entry. A file that cannot be
// read, or a read that stops mid-file, returns the entry as it was, so the next
// call resumes from the same line.
func scanCacheRead(path string, entry rereadEntry) rereadEntry {
	f, err := os.Open(path)
	if err != nil {
		return entry
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return entry
	}
	if info.Size() < entry.offset {
		entry = rereadEntry{}
	}
	if _, err := f.Seek(entry.offset, io.SeekStart); err != nil {
		return entry
	}
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			return entry
		}
		entry.offset += int64(len(line))
		parsed, ok := parseCostLine(line)
		if !ok || (parsed.messageID != "" && parsed.messageID == entry.lastMessage) {
			continue
		}
		entry.lastMessage = parsed.messageID
		entry.total += parsed.tokens.CacheRead
	}
}
