package chat

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/prompt"
)

// claudeQuestionTool is Claude Code's tool for asking the person. It reaches
// lich as a can_use_tool like any other, and its answers go back in the
// allowed input (measured on 2.1.328).
const claudeQuestionTool = "AskUserQuestion"

// claudeAnswerJoin joins the options picked for one question into its answer,
// the Agent SDK's own reading of a multiple choice.
const claudeAnswerJoin = ", "

type claudeQuestionInput struct {
	Questions []struct {
		Question string `json:"question"`
		Header   string `json:"header"`
		Options  []struct {
			Label       string `json:"label"`
			Description string `json:"description"`
			Preview     string `json:"preview"`
		} `json:"options"`
		MultiSelect bool `json:"multiSelect"`
	} `json:"questions"`
	Answers map[string]string `json:"answers"`
}

// readClaudeQuestion reads AskUserQuestion's input.
func readClaudeQuestion(raw json.RawMessage) (api.ChatQuestion, claudeQuestionInput) {
	var in claudeQuestionInput
	_ = json.Unmarshal(raw, &in)
	q := api.ChatQuestion{Questions: []api.ChatAsk{}}
	for _, ask := range in.Questions {
		a := api.ChatAsk{Header: ask.Header, Text: ask.Question, Multi: ask.MultiSelect}
		for _, o := range ask.Options {
			a.Options = append(a.Options, api.ChatOffer{Label: o.Label, Description: o.Description, Preview: o.Preview})
		}
		q.Questions = append(q.Questions, a)
	}
	return q, in
}

// claudeAnswers is the answers map AskUserQuestion takes: each question's text
// to the options picked and the person's own words, joined.
func claudeAnswers(asks []api.ChatAsk, r api.ChatQReplied) map[string]string {
	answers := map[string]string{}
	for i, ask := range asks {
		parts := slices.Clone(r.Answers[i].Picked)
		if other := strings.TrimSpace(r.Answers[i].Other); other != "" {
			parts = append(parts, other)
		}
		answers[ask.Text] = strings.Join(parts, claudeAnswerJoin)
	}
	return answers
}

// claudeReplied reads the answers map back into a reply: a whole answer that
// is an option's label is that option, else each part that is one is picked
// and the rest is the person's own words.
func claudeReplied(asks []api.ChatAsk, answers map[string]string) api.ChatQReplied {
	r := api.ChatQReplied{Answers: make([]api.ChatAskAnswer, len(asks))}
	for i, ask := range asks {
		answer := answers[ask.Text]
		labels := make([]string, len(ask.Options))
		for j, o := range ask.Options {
			labels[j] = o.Label
		}
		if slices.Contains(labels, answer) {
			r.Answers[i].Picked = []string{answer}
			continue
		}
		var other []string
		for part := range strings.SplitSeq(answer, claudeAnswerJoin) {
			if slices.Contains(labels, part) {
				r.Answers[i].Picked = append(r.Answers[i].Picked, part)
			} else if part != "" {
				other = append(other, part)
			}
		}
		r.Answers[i].Other = strings.Join(other, claudeAnswerJoin)
	}
	return r
}

// askQuestion turns AskUserQuestion's can_use_tool into a question item.
// Answered, the tool is allowed with the answers in its input; declined, it is
// denied with a word to the agent that nobody answered.
func (d *claudeDriver) askQuestion(ask claudeAsk, respond func(map[string]any) error) {
	q, _ := readClaudeQuestion(ask.Input)
	var input map[string]any
	_ = json.Unmarshal(ask.Input, &input)
	d.c.question(ask.ToolUseID, q, func(r api.ChatQReplied) error {
		if r.Declined {
			return respond(map[string]any{"behavior": "deny", "message": prompt.For(d.lang).ChatQuestionDeclined})
		}
		updated := map[string]any{}
		maps.Copy(updated, input)
		updated["answers"] = claudeAnswers(q.Questions, r)
		return respond(map[string]any{"behavior": "allow", "updatedInput": updated})
	})
}

// readClaudeQuestionResult records how a question item was answered, from its
// tool result: the answers the tool echoes back, or none when it was denied.
// Live, the item already holds the reply; this is for the transcript.
func readClaudeQuestionResult(c *chatSession, it api.ChatItem, b claudeBlock, result json.RawMessage) {
	if b.IsError {
		c.replied(it.ID, api.ChatQReplied{Declined: true})
		return
	}
	_, in := readClaudeQuestion(result)
	c.replied(it.ID, claudeReplied(it.Question.Questions, in.Answers))
}
