package browser

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/lichdotdev/lich/api"
)

// beforeunload is Chrome's type for the dialog that asks to leave a page.
const beforeunload = string(page.DialogTypeBeforeunload)

// answeredMax caps the dialogs the person answered that wait for the agent's
// next page answer, as consoleErrorMax caps errors.
const answeredMax = 20

// beforeunloadSettle is how long after a beforeunload opens accepting it
// waits for. Accepted sooner, the navigation it held is aborted about a second
// in (net::ERR_ABORTED) and Chrome reloads the page, which asks again: 3 of 3
// at 300 ms and 3 of 6 at 400 ms aborted, none of 30 from 500 ms on (Chrome
// for Testing 155 and Helium 0.19.2.1, measured 2026-10-28).
const beforeunloadSettle = time.Second

var errNoDialog = errors.New("the page has no JavaScript dialog open")

// dialogSink keeps the dialog the page holds open, nil once it closes, and
// tells the window, whose pane shows it for the person to answer.
func (s *Service) dialogSink(sess *session) func(*api.BrowserDialog) {
	return func(d *api.BrowserDialog) {
		s.mu.Lock()
		sess.dialog = d
		sess.dialogAt = time.Now()
		s.mu.Unlock()
		s.emitState(sess, true)
	}
}

// AnswerDialog is the agent answering the page's open dialog, which the pane
// plays as its cursor pressing the dialog's button. It answers with the page
// as it stands after.
func (s *Service) AnswerDialog(_ context.Context, opts api.BrowserDialogOptions) (api.BrowserPageInfo, error) {
	sess, d, err := s.openDialog(opts.ID)
	if err != nil {
		return api.BrowserPageInfo{}, err
	}
	action := "dismiss"
	if opts.Accept {
		action = "accept"
	}
	s.agentActed(sess, api.BrowserAgentAct{Action: action})
	if err := s.answerDialog(sess, d, opts); err != nil {
		return api.BrowserPageInfo{}, err
	}
	return s.pageAfter(opts.ID)
}

// Answer is the person answering the page's open dialog in the pane. The
// agent hears of it in its next page answer.
func (s *Service) Answer(_ context.Context, opts api.BrowserDialogOptions) (api.None, error) {
	sess, d, err := s.openDialog(opts.ID)
	if err != nil {
		return api.None{}, err
	}
	if err := s.answerDialog(sess, d, opts); err != nil {
		return api.None{}, err
	}
	d.Answer = api.DialogDismissed
	if opts.Accept {
		d.Answer = api.DialogAccepted
	}
	s.mu.Lock()
	if len(sess.answered) < answeredMax {
		sess.answered = append(sess.answered, d)
	}
	s.mu.Unlock()
	return api.None{}, nil
}

func (s *Service) openDialog(id string) (*session, api.BrowserDialog, error) {
	sess, err := s.live(id)
	if err != nil {
		return nil, api.BrowserDialog{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess.dialog == nil {
		return nil, api.BrowserDialog{}, errNoDialog
	}
	return sess, *sess.dialog, nil
}

// settleBeforeunload holds an accept of a beforeunload until
// beforeunloadSettle after it opened.
func (s *Service) settleBeforeunload(sess *session, d api.BrowserDialog, accept bool) {
	if !accept || d.Type != beforeunload {
		return
	}
	s.mu.Lock()
	at := sess.dialogAt
	s.mu.Unlock()
	time.Sleep(time.Until(at.Add(beforeunloadSettle)))
}

// answerDialog answers d, a prompt with opts.Text or else its default text.
func (s *Service) answerDialog(sess *session, d api.BrowserDialog, opts api.BrowserDialogOptions) error {
	text := d.Default
	if opts.Text != nil {
		text = *opts.Text
	}
	s.settleBeforeunload(sess, d, opts.Accept)
	err := runPage(func(ctx context.Context) error {
		return sess.page.AnswerDialog(ctx, opts.Accept, text)
	})
	if err == nil {
		s.touch(sess)
	}
	return err
}

// failed is what a page call that failed tells its caller: that the page
// holds a dialog open, which leaves the browser as it is, or the error, after
// dropping a browser whose Chrome is gone.
func (s *Service) failed(sess *session, err error) error {
	if !errors.Is(err, errDialogOpen) {
		s.drop(sess)
		return err
	}
	s.mu.Lock()
	d := sess.dialog
	s.mu.Unlock()
	if d == nil {
		return err
	}
	return fmt.Errorf("%w: a %s saying %q; answer it with browser_dialog first", errDialogOpen, d.Type, d.Message)
}

// heldPage is the agent's page answer while the page holds a dialog open:
// the page as lich last saw it, and the dialog.
func (s *Service) heldPage(id string) (api.BrowserPageInfo, error) {
	sess, err := s.live(id)
	if err != nil {
		return api.BrowserPageInfo{}, err
	}
	s.mu.Lock()
	info := api.BrowserPageInfo{URL: sess.url, Title: sess.title}
	if sess.dialog != nil {
		d := *sess.dialog
		info.Dialog = &d
	}
	s.mu.Unlock()
	info.Downloads = s.untold(sess)
	info.ConsoleErrors, info.ConsoleErrorsMore = s.consoleUntold(sess)
	info.Dialogs = s.answeredUntold(sess)
	return info, nil
}

// answeredUntold is the dialogs the person answered that the agent has not
// heard of, which it now has.
func (s *Service) answeredUntold(sess *session) []api.BrowserDialog {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := sess.answered
	sess.answered = nil
	return out
}
