package backend

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

// rawArgs joins the argument list as the backend received it.
func rawArgs(args []json.RawMessage) string {
	s := ""
	for i, a := range args {
		if i > 0 {
			s += ","
		}
		s += string(a)
	}
	return s
}

func TestFooterCallsSendTheirOptions(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name, method, args string
		result             any
		call               func(c *Client) error
	}{
		{"QuotaPlans", "quota.Plans", `{"sessionId":"s1"}`, []QuotaPlan{}, func(c *Client) error {
			_, err := c.QuotaPlans(ctx, "s1")
			return err
		}},
		{"HandsOn", "terminal.HandsOn", `{"id":"s1"}`, 0, func(c *Client) error {
			_, err := c.HandsOn(ctx, "s1")
			return err
		}},
		{"Attach", "drop.Attach", `{"sessionId":"s1","root":"/repo","confined":true}`, Attachment{}, func(c *Client) error {
			_, err := c.Attach(ctx, "s1", "/repo", true)
			return err
		}},
		{"Write", "terminal.Write", `{"id":"s1","data":"hi"}`, nil, func(c *Client) error {
			return c.Write(ctx, "s1", "hi")
		}},
		{"SetSetting", "store.SetSetting", `{"key":"footer.layout","projectId":"","value":"{}"}`, nil, func(c *Client) error {
			return c.SetSetting(ctx, "footer.layout", "", "{}")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := fakeBackend(t, func(method string, args []json.RawMessage) (any, int) {
				if method != tc.method || rawArgs(args) != tc.args {
					t.Errorf("got %s(%s), want %s(%s)", method, rawArgs(args), tc.method, tc.args)
				}
				return tc.result, http.StatusOK
			})
			if err := tc.call(c); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestQuotaPlansDecodesWindows(t *testing.T) {
	c := fakeBackend(t, func(string, []json.RawMessage) (any, int) {
		return json.RawMessage(`[{"provider":"claude","name":"Claude","status":"ok","noAccount":"token-login","resetCredits":2,
			"extra":{"used":1250,"limit":5000,"currency":"USD","decimals":2},
			"windows":[{"label":"Session","seconds":18000,"percent":42,"resetsAt":"2026-10-11T15:00:00Z","active":true,"lockedReason":"x","runsOutAt":"2026-10-12T00:00:00Z"}]}]`), http.StatusOK
	})
	plans, err := c.QuotaPlans(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	want := QuotaWindow{Label: "Session", Seconds: 18000, Percent: 42, ResetsAt: "2026-10-11T15:00:00Z", Active: true, LockedReason: "x", RunsOutAt: "2026-10-12T00:00:00Z"}
	if len(plans) != 1 || plans[0].NoAccount != "token-login" || len(plans[0].Windows) != 1 || plans[0].Windows[0] != want {
		t.Fatalf("got %+v", plans)
	}
	if e := plans[0].Extra; plans[0].ResetCredits != 2 || e == nil || e.Used != 1250 || e.Limit == nil || *e.Limit != 5000 || e.Currency != "USD" {
		t.Fatalf("extra usage and reset credits: %+v", plans[0])
	}
}

func TestQuotaUseResetSendsTheProfileAndGrantAndDecodesTheOutcome(t *testing.T) {
	var method string
	var args []json.RawMessage
	c := fakeBackend(t, func(m string, a []json.RawMessage) (any, int) {
		method, args = m, a
		return json.RawMessage(`{"status":"cooldown","cooldownUntil":"2026-11-20T10:00:00Z"}`), http.StatusOK
	})
	got, err := c.QuotaUseReset(context.Background(), "work", "full-reset-2611")
	if err != nil {
		t.Fatal(err)
	}
	if method != "quota.UseReset" || len(args) != 1 || string(args[0]) != `{"profile":"work","grantId":"full-reset-2611"}` {
		t.Errorf("sent %s %s", method, args)
	}
	if got.Status != "cooldown" || got.CooldownUntil != "2026-11-20T10:00:00Z" {
		t.Errorf("got %+v", got)
	}
}

func TestHandsOnAndAttachDecode(t *testing.T) {
	c := fakeBackend(t, func(method string, _ []json.RawMessage) (any, int) {
		if method == "terminal.HandsOn" {
			return 3900, http.StatusOK
		}
		return Attachment{Path: "/tmp/a b", Notice: "copied"}, http.StatusOK
	})
	ctx := context.Background()
	if s, err := c.HandsOn(ctx, "s1"); err != nil || s != 3900 {
		t.Fatalf("HandsOn = %d, %v", s, err)
	}
	if a, err := c.Attach(ctx, "s1", "/repo", false); err != nil || a.Path != "/tmp/a b" || a.Notice != "copied" {
		t.Fatalf("Attach = %+v, %v", a, err)
	}
}
