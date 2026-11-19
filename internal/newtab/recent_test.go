package newtab

import (
	"fmt"
	"testing"
)

func titles(s *Service, session string) []string {
	var out []string
	for _, r := range s.recent(session) {
		out = append(out, r.URL+"|"+r.Title)
	}
	return out
}

func TestVisitKeepsNewestFirstOncePerPage(t *testing.T) {
	s := New(newFixture().src)
	s.Visit("s1", "https://a.test/", "A")
	s.Visit("s1", "https://b.test/", "B")
	s.Visit("s1", "https://a.test/", "A")
	if got := fmt.Sprint(titles(s, "s1")); got != "[https://a.test/|A https://b.test/|B]" {
		t.Fatalf("recent = %s", got)
	}
}

func TestVisitTakesTheTitleThatArrivesAfterTheURL(t *testing.T) {
	f := newFixture()
	s := New(f.src)
	s.Visit("s1", "https://a.test/", "")
	s.Visit("s1", "https://a.test/", "A")
	s.Visit("s1", "https://a.test/", "")
	if got := fmt.Sprint(titles(s, "s1")); got != "[https://a.test/|A]" {
		t.Fatalf("recent = %s", got)
	}
}

func TestVisitWritesNothingForAnUnchangedPage(t *testing.T) {
	f := newFixture()
	s := New(f.src)
	s.Visit("s1", "https://a.test/", "A")
	saves := f.kept.saves
	s.Visit("s1", "https://a.test/", "A")
	s.Visit("s1", "https://a.test/", "")
	if f.kept.saves != saves {
		t.Fatalf("%d saves for a page already first", f.kept.saves-saves)
	}
}

func TestVisitKeepsTheLastEight(t *testing.T) {
	s := New(newFixture().src)
	for i := range 12 {
		s.Visit("s1", fmt.Sprintf("https://%d.test/", i), "")
	}
	got := s.recent("s1")
	if len(got) != maxRecent || got[0].URL != "https://11.test/" || got[maxRecent-1].URL != "https://4.test/" {
		t.Fatalf("recent = %v", got)
	}
}

func TestVisitSkipsWhatIsNoWebPage(t *testing.T) {
	s := New(newFixture().src)
	for _, raw := range []string{"about:blank", "", "file:///etc/passwd", "javascript:1", "http://127.0.0.1:27000/new-tab/s1", "https://"} {
		s.Visit("s1", raw, "x")
	}
	if got := s.recent("s1"); len(got) != 0 {
		t.Fatalf("remembered %v", got)
	}
}

func TestRecentPagesBelongToTheirSession(t *testing.T) {
	s := New(newFixture().src)
	s.Visit("s1", "https://a.test/", "")
	if got := s.recent("s2"); len(got) != 0 {
		t.Fatalf("session s2 sees %v", got)
	}
}

func TestRecentPagesSurviveARestartAndGoWithTheSession(t *testing.T) {
	f := newFixture()
	New(f.src).Visit("s1", "https://a.test/", "A")
	again := New(f.src)
	if got := fmt.Sprint(titles(again, "s1")); got != "[https://a.test/|A]" {
		t.Fatalf("after a restart: %s", got)
	}
	again.Forget("key-s1")
	if len(f.kept.values) != 0 || len(again.recent("s1")) != 0 {
		t.Fatalf("kept after Forget: %v", f.kept.values)
	}
	if len(New(f.src).recent("s1")) != 0 {
		t.Fatal("a fresh service reads what was dropped")
	}
}

func TestAnUnreadableStoredListIsAnEmptyOne(t *testing.T) {
	f := newFixture()
	f.kept.values["browser.recent.key-s1"] = "{not json"
	if got := New(f.src).recent("s1"); len(got) != 0 {
		t.Fatalf("recent = %v", got)
	}
}
