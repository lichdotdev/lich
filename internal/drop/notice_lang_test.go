package drop

import (
	"strings"
	"testing"

	"github.com/lichdotdev/lich/internal/prompt"
)

func TestCopyNoticeRendersInEveryLanguage(t *testing.T) {
	for _, lang := range prompt.Langs {
		got := copyNotice(lang, "shot.png")
		if strings.Contains(got, "%!") || !strings.HasPrefix(got, "[lich] ") || !strings.Contains(got, "shot.png") {
			t.Errorf("%s: notice = %q", lang, got)
		}
	}
}

// The language is read when the notice is composed, so a change in Settings
// reaches the next drop.
func TestTheCopyNoticeFollowsThePromptLanguageLive(t *testing.T) {
	svc := New(t.TempDir(), nil)
	if got := svc.lang(); got != prompt.English {
		t.Fatalf("unwired language = %q, want English", got)
	}
	lang := prompt.English
	svc.SetPromptLanguage(func() prompt.Lang { return lang })
	lang = prompt.PortugueseBR

	if got := copyNotice(svc.lang(), "a.png"); got != copyNotice(prompt.PortugueseBR, "a.png") {
		t.Errorf("notice = %q, want it in the language set after startup", got)
	}
}

func TestTheCopyNoticeIsSpanishWhenChosen(t *testing.T) {
	got := copyNotice(prompt.Spanish, "a.png")
	if got == copyNotice(prompt.English, "a.png") || !strings.Contains(got, "copia de a.png") {
		t.Errorf("notice = %q, want the Spanish text", got)
	}
}

func TestTheCopyNoticeIsJapaneseWhenChosen(t *testing.T) {
	got := copyNotice(prompt.Japanese, "a.png")
	if got == copyNotice(prompt.English, "a.png") || !strings.Contains(got, "a.png のコピーです") {
		t.Errorf("notice = %q, want the Japanese text", got)
	}
}

func TestTheCopyNoticeIsKoreanWhenChosen(t *testing.T) {
	got := copyNotice(prompt.Korean, "a.png")
	if got == copyNotice(prompt.English, "a.png") || !strings.Contains(got, "a.png의 사본") {
		t.Errorf("notice = %q, want the Korean text", got)
	}
}

func TestTheCopyNoticeIsRussianWhenChosen(t *testing.T) {
	got := copyNotice(prompt.Russian, "a.png")
	if got == copyNotice(prompt.English, "a.png") || !strings.Contains(got, "копия a.png") {
		t.Errorf("notice = %q, want the Russian text", got)
	}
}
