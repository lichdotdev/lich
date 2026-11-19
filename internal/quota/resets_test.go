package quota

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
)

var resetsNow = time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)

const scopedLimits = `"limits":[
	{"kind":"session","percent":2,"resets_at":"2026-11-19T17:50:00+00:00"},
	{"kind":"weekly_scoped","percent":6,"resets_at":"2026-11-23T15:00:00+00:00","scope":{"model":{"display_name":"Fable"}}}]`

const grantFull = `{"id":"full-reset-2611","label":"One full limit reset (November 2026)","resets_left":1,"ends_at":"2026-12-10T22:00:00+00:00",
	"clears":["five_hour","seven_day","seven_day_overage_included"],"paused":false,"usable_now":true}`
const grantFable = `{"id":"fable-reset-1","label":"New model launch: one more Fable reset","resets_left":2,"ends_at":"2027-03-09T07:59:00+00:00",
	"clears":["five_hour","seven_day_overage_included"],"paused":false,"usable_now":true}`

func resetsOf(t *testing.T, cedar string) []api.QuotaReset {
	t.Helper()
	writeCreds(t, claudeCredsJSON, "")
	url, _ := serve(t, http.StatusOK, `{`+scopedLimits+`,"cedar_ember":`+cedar+`}`)
	got := newService(url, "", resetsNow).claudePlan(lichEnv())
	if got.Status != StatusOK {
		t.Fatalf("status = %q, want ok", got.Status)
	}
	return got.Resets
}

func TestClaudeGrantsAreNamedByWhatTheyClear(t *testing.T) {
	got := resetsOf(t, `{"eligible":true,"grants":[`+grantFull+`,`+grantFable+`,
		{"label":"Something new","resets_left":1,"ends_at":"2026-12-01T00:00:00+00:00","clears":["five_hour"],"usable_now":true}]}`)
	want := []api.QuotaReset{
		{ID: "full-reset-2611", Kind: api.QuotaResetFull, Clears: []string{"session", "weekly", "model"}, Left: 1, EndsAt: "2026-12-10T22:00:00+00:00"},
		{ID: "fable-reset-1", Label: "Fable", Clears: []string{"session", "model"}, Left: 2, EndsAt: "2027-03-09T07:59:00+00:00"},
		{Label: "Something new", Clears: []string{"session"}, Left: 1, EndsAt: "2026-12-01T00:00:00+00:00"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resets = %+v, want %+v", got, want)
	}
}

func TestClaudeGrantsFallBackToTheServersLabelWithoutAScopedModel(t *testing.T) {
	writeCreds(t, claudeCredsJSON, "")
	url, _ := serve(t, http.StatusOK, `{"limits":[{"kind":"session","percent":2}],"cedar_ember":{"eligible":true,"grants":[`+grantFable+`]}}`)
	got := newService(url, "", resetsNow).claudePlan(lichEnv()).Resets
	if len(got) != 1 || got[0].Label != "New model launch: one more Fable reset" {
		t.Fatalf("resets = %+v, want the server's own label", got)
	}
}

func TestClaudeGrantsLeaveOutWhatIsNotLive(t *testing.T) {
	cases := map[string]string{
		"not eligible": `{"eligible":false,"ineligible_reason":"surface","grants":[` + grantFull + `]}`,
		"expired":      `{"eligible":true,"grants":[{"resets_left":1,"ends_at":"2026-11-19T11:59:59+00:00","clears":["seven_day"],"usable_now":true}]}`,
		"ends now":     `{"eligible":true,"grants":[{"resets_left":1,"ends_at":"2026-11-19T12:00:00+00:00","clears":["seven_day"],"usable_now":true}]}`,
		"none left":    `{"eligible":true,"grants":[{"resets_left":0,"ends_at":"2026-12-10T22:00:00+00:00","clears":["seven_day"],"usable_now":false}]}`,
		"no end date":  `{"eligible":true,"grants":[{"resets_left":1,"clears":["seven_day"],"usable_now":true}]}`,
		"no grants":    `{"eligible":true,"grants":[]}`,
		"null block":   `null`,
	}
	for name, cedar := range cases {
		if got := resetsOf(t, cedar); len(got) != 0 {
			t.Errorf("%s: resets = %+v, want none", name, got)
		}
	}
}

func TestClaudeGrantsPausedOrNotUsableAreHeld(t *testing.T) {
	paused := `{"resets_left":1,"ends_at":"2026-12-10T22:00:00+00:00","clears":["seven_day"],"paused":true,"usable_now":true}`
	idle := `{"resets_left":1,"ends_at":"2026-12-10T22:00:00+00:00","clears":["seven_day"],"paused":false,"usable_now":false}`
	got := resetsOf(t, `{"eligible":true,"grants":[`+paused+`,`+idle+`,`+grantFull+`]}`)
	if len(got) != 3 || !got[0].Held || !got[1].Held || got[2].Held {
		t.Fatalf("resets = %+v, want the first two held and the last spendable", got)
	}
}

func TestClaudeAsksForGrantsInTheSameRequestAsTheWindows(t *testing.T) {
	writeCreds(t, claudeCredsJSON, "")
	var query, agent string
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		query, agent = r.URL.RawQuery, r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(claudeLimitsBody))
	}))
	defer server.Close()
	s := New()
	s.claudeURL, s.profileURL = server.URL+"?"+strings.SplitN(claudeUsageURL, "?", 2)[1], ""
	s.claudeLogin = readClaudeFile
	s.claudeVersion = func() string { return "" }
	s.claudePlan(lichEnv())
	if calls != 1 || query != "cedar_ember=1" || agent != "claude-cli/"+claudeFallbackVersion+" (external, cli)" {
		t.Errorf("calls = %d, query = %q, agent = %q", calls, query, agent)
	}
}
