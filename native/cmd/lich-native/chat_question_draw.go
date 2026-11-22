package main

import (
	"image"
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// chatPreviewShare is how much of the card the option list takes beside a
// preview, the mockup's 1fr 1fr.
const chatPreviewShare = 0.5

// chatAnswerJoin joins the options picked for one ask in the answered card.
const chatAnswerJoin = ", "

// chatQuestion is the agent asking the person: a box with the ask on show and
// its options, or, answered, a muted line per ask.
func chatQuestion(gtx C, th *ui.Theme, p *chatPane, it backend.ChatItem) D {
	if it.Question.Proposal != nil {
		it = proposalShown(it)
	}
	if it.Question.Reply != nil {
		return chatQuestionReply(gtx, th, p, it)
	}
	q := p.quiz(it.ID, it.Question.Questions)
	q.proposal = it.Question.Proposal != nil
	p.cards[it.ID] = chatCard{item: it, quiz: q}
	return chatQuestionCard(gtx, th, q, it.Question.Questions)
}

// chatQuestionCard is the box of a question waiting, its card q.
func chatQuestionCard(gtx C, th *ui.Theme, q *chatQuiz, asks []api.ChatAsk) D {
	box := th.On(th.Sidebar)
	return layout.Background{}.Layout(gtx,
		func(gtx C) D { return ui.Fill(gtx, th.Sidebar, ui.RadiusMD) },
		func(gtx C) D {
			return layout.UniformInset(ui.Space(3)).Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return chatQuestionHead(gtx, box, q, asks) }),
					ui.VGap(2.5),
					layout.Rigid(func(gtx C) D { return chatQuestionBody(gtx, box, q, asks) }),
				)
			})
		})
}

// chatQuestionHead is the ask's header and text, or the steps of several.
func chatQuestionHead(gtx C, th *ui.Theme, q *chatQuiz, asks []api.ChatAsk) D {
	if len(asks) == 1 {
		a := asks[0]
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return chatAskHeader(gtx, th, a.Header, false) }),
			ui.Gap(2),
			layout.Flexed(1, th.Text(ui.TextSM, a.Text).Weight(font.Medium).Wrapped().Layout),
			layout.Rigid(func(gtx C) D {
				if !a.Multi {
					return D{}
				}
				return th.Text(ui.TextXS, i18n.T("chat.question.multi")).Muted().Layout(gtx)
			}),
		)
	}
	steps := make([]layout.FlexChild, 0, 2*len(asks)+1)
	for i, a := range asks {
		steps = append(steps, layout.Rigid(func(gtx C) D {
			return q.steps[i].Layout(gtx, func(gtx C) D {
				label := a.Header
				if label == "" {
					label = strconv.Itoa(i + 1)
				}
				if q.done[i] {
					label = "✓ " + label
				}
				return chatAskHeader(gtx, th, label, i == q.step)
			})
		}), ui.Gap(1.5))
	}
	shown := min(q.step+1, len(asks))
	steps = append(steps, layout.Flexed(1, func(gtx C) D {
		return layout.E.Layout(gtx, th.Text(ui.TextXS, i18n.T("chat.question.step", i18n.Args{"n": shown, "total": len(asks)})).Muted().Layout)
	}))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return layout.Flex{Alignment: layout.Middle}.Layout(gtx, steps...) }),
		layout.Rigid(func(gtx C) D {
			if q.step == len(asks) {
				return D{}
			}
			return layout.Inset{Top: ui.Space(2)}.Layout(gtx, th.Text(ui.TextSM, asks[q.step].Text).Weight(font.Medium).Wrapped().Layout)
		}),
	)
}

// chatAskHeader is an ask's short label, in small capitals on a muted fill;
// lit, it is the step on show.
func chatAskHeader(gtx C, th *ui.Theme, label string, lit bool) D {
	if label == "" {
		return D{}
	}
	fill, text := th.Over(th.Muted), th.Text(ui.TextXS, strings.ToUpper(label)).Muted()
	if lit {
		fill, text = th.Over(th.SidebarAccent), text.In(th.Foreground).Weight(font.Medium)
	}
	return layout.Background{}.Layout(gtx,
		func(gtx C) D { return ui.Fill(gtx, fill, ui.RadiusSM) },
		func(gtx C) D {
			return layout.Inset{Left: ui.Space(1.5), Right: ui.Space(1.5), Top: ui.Space(0.5), Bottom: ui.Space(0.5)}.Layout(gtx, text.Layout)
		})
}

// chatQuestionBody is the ask on show: its options, a field when it has
// none, or the summary of several with Send all.
func chatQuestionBody(gtx C, th *ui.Theme, q *chatQuiz, asks []api.ChatAsk) D {
	if q.step == len(asks) {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return chatAnswerLines(gtx, th, asks, q.answers) }),
			ui.VGap(2.5),
			layout.Rigid(func(gtx C) D { return chatQuestionFoot(gtx, th, q, "", i18n.T("chat.question.sendAll"), true) }),
		)
	}
	a := asks[q.step]
	if len(a.Options) == 0 {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(th.Textarea(&q.others[q.step], "").Layout),
			ui.VGap(2),
			layout.Rigid(func(gtx C) D {
				return chatQuestionFoot(gtx, th, q, i18n.T("chat.question.freeHint"), i18n.T("chat.composer.send"), q.others[q.step].Len() > 0)
			}),
		)
	}
	list := func(gtx C) D { return chatOffers(gtx, th, q, a) }
	if hasPreview(a) {
		list = func(gtx C) D { return chatOptionsWithPreview(gtx, th, q, a) }
	}
	if !a.Multi {
		return list(gtx)
	}
	_, any := q.draft(a)
	marked := 0
	for _, m := range q.marks[q.step] {
		if m.Value {
			marked++
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(list),
		ui.VGap(2),
		layout.Rigid(func(gtx C) D {
			return chatQuestionFoot(gtx, th, q, i18n.T("chat.question.marked", i18n.Args{"count": marked}), i18n.T("chat.composer.send"), any)
		}),
	)
}

// chatQuestionFoot is a muted hint at the left and the card's send button at
// the right, dimmed until there is something to send.
func chatQuestionFoot(gtx C, th *ui.Theme, q *chatQuiz, hint, send string, ready bool) D {
	b := th.Button(&q.send, send)
	b.Size, b.Variant, b.Disabled = ui.SizeSM, ui.VariantSecondary, !ready
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, th.Text(ui.TextXS, hint).Muted().Layout),
		layout.Rigid(b.Layout),
	)
}

func hasPreview(a api.ChatAsk) bool {
	for _, o := range a.Options {
		if o.Preview != "" {
			return true
		}
	}
	return false
}

// chatOffers is an ask's options, a row each, then its "Other…" field.
func chatOffers(gtx C, th *ui.Theme, q *chatQuiz, a api.ChatAsk) D {
	rows := make([]layout.FlexChild, 0, len(a.Options)+1)
	for j, o := range a.Options {
		rows = append(rows, layout.Rigid(func(gtx C) D { return chatOptionRow(gtx, th, q, a, j, o, !hasPreview(a)) }))
	}
	rows = append(rows, layout.Rigid(func(gtx C) D { return chatOtherRow(gtx, th, q, a) }))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
}

// chatOptionsWithPreview is the options at the left and the preview of the
// one the keys stand on at the right, in mono.
func chatOptionsWithPreview(gtx C, th *ui.Theme, q *chatQuiz, a api.ChatAsk) D {
	preview := ""
	if q.hot < len(a.Options) {
		preview = a.Options[q.hot].Preview
	}
	return layout.Flex{}.Layout(gtx,
		layout.Flexed(chatPreviewShare, func(gtx C) D { return chatOffers(gtx, th, q, a) }),
		ui.Gap(2),
		layout.Flexed(1-chatPreviewShare, func(gtx C) D {
			return layout.Background{}.Layout(gtx,
				func(gtx C) D { return ui.Fill(gtx, th.Background, ui.RadiusSM) },
				func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.UniformInset(ui.Space(2)).Layout(gtx, th.On(th.Background).Text(ui.TextXS, expandTabs(preview)).Mono().Wrapped().Layout)
				})
		}),
	)
}

// chatOptionRow is option j: its key (a box to mark, for several), its label
// and its description; the one the keys stand on is lit.
func chatOptionRow(gtx C, th *ui.Theme, q *chatQuiz, a api.ChatAsk, j int, o api.ChatOffer, described bool) D {
	click := &q.rows[q.step][j]
	return click.Layout(gtx, func(gtx C) D {
		lead := func(gtx C) D { return th.KeyCap(strconv.Itoa(j + 1)).Layout(gtx) }
		if a.Multi {
			// The row is the one target: a box taking the click too would mark
			// the option and the row's click unmark it, in the same press.
			lead = func(gtx C) D { return th.Checkbox(&q.marks[q.step][j]).Layout(gtx.Disabled()) }
		}
		return chatQuizRow(gtx, th, q.hot == j || click.Hovered(), lead, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(th.Text(ui.TextSM, o.Label).Weight(font.Medium).Layout),
				layout.Rigid(func(gtx C) D {
					if !described || o.Description == "" {
						return D{}
					}
					return th.Text(ui.TextXS, o.Description).Muted().Wrapped().Layout(gtx)
				}),
			)
		})
	})
}

// chatOtherRow is the person's own answer, a field after the options.
func chatOtherRow(gtx C, th *ui.Theme, q *chatQuiz, a api.ChatAsk) D {
	n := len(a.Options)
	lead := func(gtx C) D { return th.KeyCap(strconv.Itoa(n + 1)).Layout(gtx) }
	if a.Multi {
		lead = func(gtx C) D { return D{Size: image.Pt(gtx.Dp(ui.Space(4)), 0)} }
	}
	hint := i18n.T("chat.question.write")
	if q.proposal {
		hint = i18n.T("chat.proposal.change")
	}
	return chatQuizRow(gtx, th, q.hot == n, lead, func(gtx C) D {
		in := th.Input(&q.others[q.step], hint)
		in.Height = ui.Space(8)
		return in.Layout(gtx)
	})
}

// chatQuizRow is one row of options: lead, then content, on SidebarAccent
// while lit.
func chatQuizRow(gtx C, th *ui.Theme, lit bool, lead, content layout.Widget) D {
	return layout.Background{}.Layout(gtx,
		func(gtx C) D {
			if !lit {
				return D{Size: gtx.Constraints.Min}
			}
			return ui.Fill(gtx, th.Over(th.SidebarAccent), ui.RadiusSM)
		},
		func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.UniformInset(ui.Space(2)).Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Start}.Layout(gtx,
					layout.Rigid(lead),
					ui.Gap(2.5),
					layout.Flexed(1, content),
				)
			})
		})
}

// chatQuestionAnswered is a question once answered: a line per ask, its
// header and its answer, after a hairline; declined, a muted word.
func chatQuestionAnswered(gtx C, th *ui.Theme, asks []api.ChatAsk, r *api.ChatQReplied) D {
	if r.Declined {
		return th.Text(ui.TextXS, i18n.T("chat.question.declined")).Muted().Layout(gtx)
	}
	d := layout.Inset{Left: ui.Space(3)}.Layout(gtx, func(gtx C) D { return chatAnswerLines(gtx, th, asks, r.Answers) })
	paint.FillShape(gtx.Ops, th.Border, clip.Rect{Max: image.Pt(gtx.Dp(1), d.Size.Y)}.Op())
	return d
}

// chatAnswerLines is a muted header and its answer, a line per ask.
func chatAnswerLines(gtx C, th *ui.Theme, asks []api.ChatAsk, answers []api.ChatAskAnswer) D {
	lines := make([]layout.FlexChild, 0, len(asks))
	for i, a := range asks {
		if i >= len(answers) {
			break
		}
		label := a.Header
		if label == "" {
			label = a.Text
		}
		lines = append(lines, layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
				layout.Rigid(th.Text(ui.TextXS, i18n.T("chat.question.label", i18n.Args{"header": label})).Muted().Layout),
				ui.Gap(1),
				layout.Flexed(1, th.Text(ui.TextXS, answerText(answers[i])).Weight(font.Medium).Wrapped().Layout),
			)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, lines...)
}

// answerText is an ask's answer as the card reads it: the options picked, and
// the person's own words in quotes.
func answerText(a api.ChatAskAnswer) string {
	parts := append([]string(nil), a.Picked...)
	if a.Other != "" {
		parts = append(parts, i18n.T("chat.question.own", i18n.Args{"text": a.Other}))
	}
	return strings.Join(parts, chatAnswerJoin)
}
