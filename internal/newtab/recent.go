package newtab

import (
	"encoding/json"
	"log/slog"
	"slices"
)

// maxRecent is how many pages a session's browser remembers.
const maxRecent = 8

// Recent is a page the session's browser visited.
type Recent struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

// Visit remembers that owner's browser is on rawURL, newest first, once each,
// and keeps the title the page gets after its URL. Only web pages count.
func (s *Service) Visit(owner, rawURL, title string) {
	if !isWeb(rawURL) || s.src.Skip(rawURL) {
		return
	}
	key := s.src.Key(owner)
	if key == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.loadLocked(key)
	if len(list) > 0 && list[0].URL == rawURL && (title == "" || list[0].Title == title) {
		return
	}
	if title == "" {
		if i := slices.IndexFunc(list, func(r Recent) bool { return r.URL == rawURL }); i >= 0 {
			title = list[i].Title
		}
	}
	list = slices.DeleteFunc(slices.Clone(list), func(r Recent) bool { return r.URL == rawURL })
	list = append([]Recent{{URL: rawURL, Title: title}}, list...)
	if len(list) > maxRecent {
		list = list[:maxRecent]
	}
	s.recents[key] = list
	raw, err := json.Marshal(list)
	if err == nil {
		err = s.src.Save(recentKey(key), string(raw))
	}
	if err != nil {
		slog.Warn("newtab: save recent pages", "err", err)
	}
}

// Forget drops the pages kept under a files key, its session being gone for good.
func (s *Service) Forget(key string) {
	s.mu.Lock()
	delete(s.recents, key)
	s.mu.Unlock()
	if err := s.src.Drop(recentKey(key)); err != nil {
		slog.Warn("newtab: drop recent pages", "err", err)
	}
}

func (s *Service) recent(session string) []Recent {
	key := s.src.Key(session)
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.loadLocked(key))
}

// loadLocked is key's list, read from the store the first time.
func (s *Service) loadLocked(key string) []Recent {
	if list, ok := s.recents[key]; ok {
		return list
	}
	var list []Recent
	if raw := s.src.Load(recentKey(key)); raw != "" {
		if err := json.Unmarshal([]byte(raw), &list); err != nil {
			slog.Warn("newtab: read recent pages", "err", err)
			list = nil
		}
	}
	s.recents[key] = list
	return list
}

func recentKey(filesKey string) string { return "browser.recent." + filesKey }
