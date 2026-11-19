package quota

import (
	"net/http"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
)

const limitsOnly = `"limits":[{"kind":"session","percent":2,"resets_at":"2026-08-17T23:20:00+00:00"}]`

func claudeExtraOf(t *testing.T, extra string) *api.QuotaExtra {
	t.Helper()
	writeCreds(t, claudeCredsJSON, "")
	url, _ := serve(t, http.StatusOK, `{`+limitsOnly+`,"extra_usage":`+extra+`}`)
	got := newService(url, "", time.Now()).claudePlan(lichEnv())
	if got.Status != StatusOK {
		t.Fatalf("status = %q, want ok", got.Status)
	}
	return got.Extra
}

func TestClaudeExtraUsageReadsCentsAndACap(t *testing.T) {
	got := claudeExtraOf(t, `{"is_enabled":true,"monthly_limit":5000,"used_credits":1250,"currency":"USD","decimal_places":2}`)
	limit := int64(5000)
	if got == nil || got.Used != 1250 || got.Limit == nil || *got.Limit != limit || got.Currency != "USD" || got.Decimals != 2 {
		t.Fatalf("extra = %+v", got)
	}
}

func TestClaudeExtraUsageNullLimitIsNoCap(t *testing.T) {
	got := claudeExtraOf(t, `{"is_enabled":true,"monthly_limit":null,"used_credits":300,"currency":"USD","decimal_places":2}`)
	if got == nil || got.Limit != nil || got.Used != 300 {
		t.Fatalf("extra = %+v, want 300 with no cap", got)
	}
}

func TestClaudeExtraUsageReadsFloatCredits(t *testing.T) {
	got := claudeExtraOf(t, `{"is_enabled":true,"monthly_limit":50000.0,"used_credits":14157.0,"currency":"USD","decimal_places":2}`)
	if got == nil || got.Used != 14157 || got.Limit == nil || *got.Limit != 50000 {
		t.Fatalf("extra = %+v, want 14157 of 50000", got)
	}
}

func TestClaudeExtraUsageDefaultsWhatItIsNotTold(t *testing.T) {
	got := claudeExtraOf(t, `{"is_enabled":true,"monthly_limit":null,"used_credits":null,"currency":null,"decimal_places":null}`)
	if got == nil || got.Used != 0 || got.Currency != "" || got.Decimals != 2 {
		t.Fatalf("extra = %+v, want zero spend in cents with no currency", got)
	}
	if got := claudeExtraOf(t, `{"is_enabled":true,"used_credits":500,"currency":"JPY","decimal_places":0}`); got.Decimals != 0 {
		t.Fatalf("decimals = %d, want the payload's 0", got.Decimals)
	}
	if got := claudeExtraOf(t, `{"is_enabled":true,"used_credits":500,"decimal_places":99}`); got.Decimals != 4 {
		t.Fatalf("decimals = %d, want the cap of 4", got.Decimals)
	}
}

// The payload a plan without extra usage returns: every amount null.
func TestClaudeExtraUsageDisabledIsAbsent(t *testing.T) {
	if got := claudeExtraOf(t, `{"is_enabled":false,"monthly_limit":null,"used_credits":null,"utilization":null,"currency":null,"decimal_places":null}`); got != nil {
		t.Fatalf("extra = %+v, want none while disabled", got)
	}
	if got := claudeExtraOf(t, `null`); got != nil {
		t.Fatalf("extra = %+v, want none for a null block", got)
	}
}

func TestClaudeExtraUsageAbsentFromAnOlderPayload(t *testing.T) {
	writeCreds(t, claudeCredsJSON, "")
	url, _ := serve(t, http.StatusOK, `{`+limitsOnly+`}`)
	if got := newService(url, "", time.Now()).claudePlan(lichEnv()); got.Extra != nil || got.Status != StatusOK {
		t.Fatalf("plan = %+v", got)
	}
}

const codexWindows = `"rate_limit":{"primary_window":{"used_percent":4,"limit_window_seconds":18000,"reset_at":1790000000}}`

func codexCreditsOf(t *testing.T, tail string) int {
	t.Helper()
	writeCreds(t, "", codexCredsJSON)
	url, _ := serve(t, http.StatusOK, `{"plan_type":"plus",`+codexWindows+tail+`}`)
	got := newService("", url, time.Now()).codexPlan(lichEnv())
	if got.Status != StatusOK {
		t.Fatalf("status = %q, want ok", got.Status)
	}
	return got.ResetCredits
}

func TestCodexReadsTheResetCreditsItHolds(t *testing.T) {
	got := codexCreditsOf(t, `,"rate_limit_reset_credits":{"available_count":2,"applicable_available_count":1}`)
	if got != 2 {
		t.Fatalf("reset credits = %d, want 2", got)
	}
}

func TestCodexResetCreditsAreNoneWhenAbsentZeroOrNegative(t *testing.T) {
	for name, tail := range map[string]string{
		"absent":   ``,
		"null":     `,"rate_limit_reset_credits":null`,
		"zero":     `,"rate_limit_reset_credits":{"available_count":0}`,
		"negative": `,"rate_limit_reset_credits":{"available_count":-1}`,
	} {
		if got := codexCreditsOf(t, tail); got != 0 {
			t.Errorf("%s: reset credits = %d, want 0", name, got)
		}
	}
}
