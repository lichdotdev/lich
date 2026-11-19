package chat

import (
	"encoding/json"
	"log/slog"
	"slices"
	"strings"

	"github.com/lichdotdev/lich/api"
)

// acpElicit is the ACP client method an agent asks the person through, MCP's
// elicitation in form mode: a JSON schema whose properties are the questions.
// Antigravity and oh-my-pi send it for their question tools once the client
// declares the capability (measured on agy 1.5.3 and omp 18.8.9).
const acpElicit = "elicitation/create"

// acpOtherSuffixes name the property that holds an ask's own words beside it:
// oh-my-pi writes q0__other, Antigravity q1_other.
var acpOtherSuffixes = []string{"__other", "_other"}

type acpChoice struct {
	Const string `json:"const"`
	Title string `json:"title"`
}

type acpProperty struct {
	Type  string `json:"type"`
	Title string `json:"title"`
	// Description is oh-my-pi's short label of the question, its header.
	Description string      `json:"description"`
	OneOf       []acpChoice `json:"oneOf"`
	Enum        []string    `json:"enum"`
	Items       struct {
		AnyOf []acpChoice `json:"anyOf"`
		OneOf []acpChoice `json:"oneOf"`
		Enum  []string    `json:"enum"`
	} `json:"items"`
}

// acpAsk is one question of an elicitation: the property that answers it,
// its choices, and the property for the person's own words, "" for none.
type acpAsk struct {
	key, other string
	choices    []acpChoice
	multi      bool
}

func (p acpProperty) choices() []acpChoice {
	if p.Type == "array" {
		return slices.Concat(p.Items.AnyOf, p.Items.OneOf, enumChoices(p.Items.Enum))
	}
	return slices.Concat(p.OneOf, enumChoices(p.Enum))
}

func enumChoices(values []string) []acpChoice {
	out := make([]acpChoice, len(values))
	for i, v := range values {
		out[i] = acpChoice{Const: v, Title: v}
	}
	return out
}

func (c acpChoice) label() string {
	if c.Title != "" {
		return c.Title
	}
	return c.Const
}

// readElicitation reads a form's schema into a question, its properties
// sorted by name, which is the order both agents number them in.
func readElicitation(props map[string]acpProperty) (api.ChatQuestion, []acpAsk) {
	keys := slices.Sorted(func(yield func(string) bool) {
		for k := range props {
			if otherOf(props, k) == "" && !yield(k) {
				return
			}
		}
	})
	q := api.ChatQuestion{Questions: []api.ChatAsk{}}
	asks := make([]acpAsk, 0, len(keys))
	for _, k := range keys {
		p := props[k]
		a := acpAsk{key: k, choices: p.choices(), multi: p.Type == "array"}
		for _, s := range acpOtherSuffixes {
			if _, ok := props[k+s]; ok {
				a.other = k + s
			}
		}
		ask := api.ChatAsk{Header: p.Description, Text: p.Title, Multi: a.multi}
		for _, c := range a.choices {
			ask.Options = append(ask.Options, api.ChatOffer{Label: c.label()})
		}
		q.Questions = append(q.Questions, ask)
		asks = append(asks, a)
	}
	return q, asks
}

// otherOf is the ask property k holds the own words of, "" when k is a
// question of its own.
func otherOf(props map[string]acpProperty, k string) string {
	for _, s := range acpOtherSuffixes {
		if base, ok := strings.CutSuffix(k, s); ok {
			if _, exists := props[base]; exists {
				return base
			}
		}
	}
	return ""
}

// elicitContent is the form's answer: each ask's choices by their const, and
// its own words in its other property, or in itself when it has no choices.
func elicitContent(asks []acpAsk, r api.ChatQReplied) map[string]any {
	content := map[string]any{}
	for i, a := range asks {
		ans := r.Answers[i]
		var picked []string
		for _, c := range a.choices {
			if slices.Contains(ans.Picked, c.label()) {
				picked = append(picked, c.Const)
			}
		}
		switch {
		case len(a.choices) == 0:
			content[a.key] = ans.Other
		case a.multi && len(picked) > 0:
			content[a.key] = picked
		case len(picked) > 0:
			content[a.key] = picked[0]
		}
		if a.other != "" && ans.Other != "" {
			content[a.other] = ans.Other
		}
	}
	return content
}

// elicit turns an elicitation into a question item. A form is all lich
// serves: any other mode is declined.
func (d *acpDriver) elicit(m acpMessage) {
	var req struct {
		Mode   string `json:"mode"`
		Schema struct {
			Properties map[string]acpProperty `json:"properties"`
		} `json:"requestedSchema"`
	}
	_ = json.Unmarshal(m.Params, &req)
	respond := func(result map[string]any) error {
		return d.c.out.send(map[string]any{"jsonrpc": "2.0", "id": m.ID, "result": result})
	}
	q, asks := readElicitation(req.Schema.Properties)
	if req.Mode != "form" || len(asks) == 0 {
		if err := respond(map[string]any{"action": "decline"}); err != nil {
			slog.Debug("chat: decline an elicitation", "session", d.c.id, "err", err)
		}
		return
	}
	d.c.question("elicit-"+strings.Trim(string(m.ID), `"`), q, func(r api.ChatQReplied) error {
		if r.Declined {
			return respond(map[string]any{"action": "decline"})
		}
		return respond(map[string]any{"action": "accept", "content": elicitContent(asks, r)})
	})
}
