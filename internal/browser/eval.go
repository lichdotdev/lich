package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/lichdotdev/lich/api"
)

// evalMax caps a script's answer, which lands whole in the agent's context:
// a page's state serialized runs to tens of thousands of tokens, and the
// script can pick the part it wants instead.
const evalMax = 2000

var errNoScript = errors.New("eval needs a script")

func (s *Service) Eval(_ context.Context, opts api.BrowserEvalOptions) (string, error) {
	if strings.TrimSpace(opts.Script) == "" {
		return "", errNoScript
	}
	defer s.agentTurn(opts.ID)()
	var out string
	sess, err := s.drive(opts.ID, func(p Page, ctx context.Context) error {
		var err error
		out, err = p.Eval(ctx, opts.Script)
		return err
	})
	if err != nil {
		return "", err
	}
	s.agentActed(sess, api.BrowserAgentAct{})
	if r := []rune(out); len(r) > evalMax {
		return string(r[:evalMax]) + "…", nil
	}
	return out, nil
}

// Eval runs in REPL mode, as DevTools' console does: top-level await works
// and a let the agent's last script declared can be declared again.
func (p *chromePage) Eval(ctx context.Context, script string) (string, error) {
	var res *runtime.RemoteObject
	var exc *runtime.ExceptionDetails
	err := p.run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		res, exc, err = runtime.Evaluate(script).
			WithReturnByValue(true).WithAwaitPromise(true).WithReplMode(true).Do(ctx)
		return err
	}))
	if err != nil {
		return "", err
	}
	if exc != nil {
		return "", fmt.Errorf("the script threw: %s", exceptionText(exc))
	}
	return evalValue(res), nil
}

// evalValue is a script's value as the agent reads it. An object returned by
// value is its JSON; one JSON cannot hold (a DOM node, a function) comes back
// as {} or its description.
func evalValue(o *runtime.RemoteObject) string {
	switch {
	case o == nil || o.Type == runtime.TypeUndefined:
		return "undefined"
	case o.UnserializableValue != "":
		return string(o.UnserializableValue)
	case len(o.Value) > 0:
		var s string
		if o.Type == runtime.TypeString && json.Unmarshal(o.Value, &s) == nil {
			return s
		}
		return string(o.Value)
	case o.Description != "":
		return o.Description
	}
	return string(o.Type)
}
