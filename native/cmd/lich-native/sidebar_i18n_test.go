package main

import (
	"testing"
	"time"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/lichclient"
)

func inLocale(t *testing.T, l i18n.Locale) {
	t.Helper()
	i18n.SetCurrent(l)
	t.Cleanup(func() { i18n.SetCurrent(i18n.EN) })
}

// The narrow units are what Intl.NumberFormat(locale, {style: "unit",
// unitDisplay: "narrow"}) writes.
func TestFormatAgeReadsLikeIntlInEveryLocale(t *testing.T) {
	cases := []struct {
		locale i18n.Locale
		want   [3]string
	}{
		{i18n.EN, [3]string{"3s", "3m", "3h"}},
		{i18n.PtBR, [3]string{"3 s", "3 min", "3 h"}},
		{i18n.ES, [3]string{"3s", "3min", "3h"}},
		{i18n.ZhCN, [3]string{"3秒", "3分钟", "3小时"}},
		{i18n.KO, [3]string{"3초", "3분", "3시간"}},
	}
	for _, c := range cases {
		inLocale(t, c.locale)
		for i, elapsed := range []time.Duration{3 * time.Second, 3 * time.Minute, 3 * time.Hour} {
			if got := formatAge(elapsed); got != c.want[i] {
				t.Errorf("%s: formatAge(%v) = %q, want %q", c.locale, elapsed, got, c.want[i])
			}
		}
	}
}

// The palette's phrase is Intl.RelativeTimeFormat(locale, {style: "narrow",
// numeric: "always"}) writes.
func TestAgoLabelReadsLikeIntlInEveryLocale(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	ago := func(d time.Duration) int64 { return now.Add(-d).Unix() }
	ats := []int64{ago(3 * time.Minute), ago(3 * time.Hour), ago(3 * 24 * time.Hour), ago(21 * 24 * time.Hour)}
	cases := []struct {
		locale i18n.Locale
		want   [4]string
	}{
		{i18n.PtBR, [4]string{"há 3 min.", "há 3 h", "há 3 dias", "há 3 sem."}},
		{i18n.ES, [4]string{"hace 3 min", "hace 3 h", "hace 3 d", "hace 3 sem."}},
		{i18n.ZhCN, [4]string{"3分钟前", "3小时前", "3天前", "3周前"}},
		{i18n.KO, [4]string{"3분 전", "3시간 전", "3일 전", "3주 전"}},
	}
	for _, c := range cases {
		inLocale(t, c.locale)
		for i, at := range ats {
			if got := agoLabel(at, now); got != c.want[i] {
				t.Errorf("%s: agoLabel #%d = %q, want %q", c.locale, i, got, c.want[i])
			}
		}
	}
	inLocale(t, i18n.PtBR)
	if got := agoLabel(ago(time.Second), now); got != "agora mesmo" {
		t.Errorf("under a minute = %q, want agora mesmo", got)
	}
}

func TestNoMatchNoticeJoinsThePhasesInTheLocale(t *testing.T) {
	inLocale(t, i18n.PtBR)
	got := noMatchNotice("webgl", picked(phaseUnread, phaseWaiting))
	want := "Nenhuma sessão aguardando ou não lidas corresponde a “webgl”. A sessão ativa continua."
	if got != want {
		t.Errorf("noMatchNotice = %q, want %q", got, want)
	}
}

func TestFreshnessLinePluralisesInTheLocale(t *testing.T) {
	inLocale(t, i18n.PtBR)
	one, _ := freshnessLine("main", doneCheck(lichclient.BaseFreshness{Upstream: "origin/main", Ahead: 1}, ""), false)
	many, _ := freshnessLine("main", doneCheck(lichclient.BaseFreshness{Upstream: "origin/main", Ahead: 2}, ""), false)
	if one != "1 commit não enviado · começa da main local" {
		t.Errorf("one = %q", one)
	}
	if many != "2 commits não enviados · começa da main local" {
		t.Errorf("many = %q", many)
	}
}

func TestCloseBodyPluralisesInTheLocale(t *testing.T) {
	inLocale(t, i18n.PtBR)
	if got := closeBody("lich", 1); got != "Fechar lich interrompe uma sessão no meio do turno. Ao reabrir o projeto, você pode retomar de onde cada uma parou; o turno em andamento é perdido." {
		t.Errorf("one = %q", got)
	}
	if got := closeBody("lich", 3); got == closeBody("lich", 1) {
		t.Errorf("3 sessions read like one: %q", got)
	}
}

// A value rendered in its own weight can sit anywhere in a sentence: the
// message splits around it, whichever side the language puts it on.
func TestAroundValueSplitsAtThePlaceholder(t *testing.T) {
	cases := []struct {
		locale        i18n.Locale
		before, after string
	}{
		{i18n.EN, "No matches for ", " in sessions"},
		{i18n.ZhCN, "在sessions中没有与 ", " 匹配的结果"},
		{i18n.KO, "sessions에서 ", "와(과) 일치하는 항목이 없습니다"},
	}
	for _, c := range cases {
		inLocale(t, c.locale)
		before, after := aroundValue("palette.commandPalette.noMatchesInTab", "query", i18n.Args{"tab": "sessions"})
		if before != c.before || after != c.after {
			t.Errorf("%s: (%q, %q), want (%q, %q)", c.locale, before, after, c.before, c.after)
		}
	}
}

func TestPaletteTabLabelsAreTranslatedButTheirValuesStay(t *testing.T) {
	inLocale(t, i18n.PtBR)
	if got := tabSessions.label(); got != "Sessões" {
		t.Errorf("label = %q, want Sessões", got)
	}
	if string(tabSessions) != "Sessions" {
		t.Errorf("value = %q: it is the -palette-tab flag and must stay", tabSessions)
	}
}
