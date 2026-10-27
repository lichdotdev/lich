package terminal

import (
	"bytes"
	"database/sql"
	"encoding/json"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// lastTurnCache is the newest assistant turn's cache split for the two
// cost-only providers that record one, oh-my-pi and opencode. Both file the
// input with the cache already taken out of it, so the three counters add up
// to the turn's whole input as they stand (measured against omp 18.0.10's
// calculateOpenAIUsageAccounting and an opencode 1.18.23 row whose total is
// input + output + cache.read + cache.write). nil on any miss, which the
// footer reads as no cache line.
func lastTurnCache(src usageSource) *api.SessionCache {
	switch src.kind {
	case providers.OMP:
		return ompLastTurnCache(src.path)
	case providers.OpenCode:
		return opencodeLastTurnCache(src.path, src.id)
	}
	return nil
}

// ompLastTurnCache reads the newest assistant line of an oh-my-pi session
// file, from the end, so a long conversation costs no more than a short one.
func ompLastTurnCache(path string) *api.SessionCache {
	var cache *api.SessionCache
	codexReverseScan(path, func(line []byte) bool {
		if !bytes.Contains(line, ompAssistantMark) {
			return false
		}
		cache = parseOMPCacheLine(line)
		return cache != nil
	})
	return cache
}

func parseOMPCacheLine(line []byte) *api.SessionCache {
	var entry struct {
		Type    string `json:"type"`
		Message struct {
			Role  string `json:"role"`
			Usage *struct {
				Input      int `json:"input"`
				CacheRead  int `json:"cacheRead"`
				CacheWrite int `json:"cacheWrite"`
			} `json:"usage"`
		} `json:"message"`
	}
	if json.Unmarshal(line, &entry) != nil || entry.Type != "message" ||
		entry.Message.Role != "assistant" || entry.Message.Usage == nil {
		return nil
	}
	u := entry.Message.Usage
	return &api.SessionCache{Read: u.CacheRead, Write: u.CacheWrite, Uncached: u.Input}
}

// opencodeLastTurnCacheQuery is the newest finished assistant message of one
// session: a message still streaming carries zero tokens until it completes.
// A sub-agent is a session of its own, and its turns are not this window's.
const opencodeLastTurnCacheQuery = `SELECT json_extract(data, '$.tokens.input'),
	json_extract(data, '$.tokens.cache.read'), json_extract(data, '$.tokens.cache.write')
	FROM message WHERE session_id = ? AND json_extract(data, '$.role') = 'assistant'
	AND json_extract(data, '$.time.completed') IS NOT NULL
	ORDER BY time_created DESC LIMIT 1`

func opencodeLastTurnCache(path, id string) *api.SessionCache {
	if id == "" {
		return nil
	}
	db, ok := openSessionDB(path)
	if !ok {
		return nil
	}
	defer func() { _ = db.Close() }()

	var input, read, write sql.NullInt64
	if db.QueryRow(opencodeLastTurnCacheQuery, id).Scan(&input, &read, &write) != nil || !input.Valid {
		return nil
	}
	return &api.SessionCache{Read: int(read.Int64), Write: int(write.Int64), Uncached: int(input.Int64)}
}
