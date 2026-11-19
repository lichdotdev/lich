package main

import (
	"slices"
	"sync"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// resetUse is the usage panel's spend of a limit reset: which grant waits for
// its confirmation, which one is out to the provider, and the buttons. A spend
// cannot be undone, so it is always asked twice, in the row itself.
type resetUse struct {
	mu   sync.Mutex
	busy string // the grant being spent, "" for none

	pending         string // the grant whose confirmation replaces its row
	profile, key    string // the Claude Code profile asked about, and its planQuota key
	act             *sidebarAction
	buttons         map[string]*widget.Clickable
	cancel, confirm widget.Clickable
}

// resetRequest is a confirmed spend, handed to the window to send.
type resetRequest struct{ profile, key, grantID string }

func (u *resetUse) busyNow() bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.busy != ""
}

func (u *resetUse) setBusy(id string) {
	u.mu.Lock()
	u.busy = id
	u.mu.Unlock()
}

// button is the Use button of grant id, with the press already taken: it asks
// for the confirmation, unless a spend is out.
func (u *resetUse) button(gtx C, th *ui.Theme, id string) layout.Widget {
	if u.buttons == nil {
		u.buttons = map[string]*widget.Clickable{}
	}
	click := clickableOf(u.buttons, id)
	if click.Clicked(gtx) && !u.busyNow() {
		u.pending = id
		gtx.Execute(op.InvalidateCmd{}) // the row is already drawn as it was
	}
	b := th.Button(click, i18n.T("shell.planQuota.resetUse"))
	b.Variant, b.Size, b.Disabled = ui.VariantOutline, ui.SizeXS, u.busyNow()
	return b.Layout
}

// confirmation is the row's replacement: what is about to be spent, what it
// clears, and Cancel beside the destructive Use.
func (u *resetUse) confirmation(th *ui.Theme, r backend.QuotaReset) layout.Widget {
	return func(gtx C) D {
		if u.cancel.Clicked(gtx) {
			u.pending = ""
			gtx.Execute(op.InvalidateCmd{})
		}
		if u.confirm.Clicked(gtx) && !u.busyNow() {
			u.pending = ""
			gtx.Execute(op.InvalidateCmd{})
			u.setBusy(r.ID)
			u.act.useReset = resetRequest{profile: u.profile, key: u.key, grantID: r.ID}
		}
		cancel := th.Button(&u.cancel, i18n.T("common.action.cancel"))
		cancel.Variant, cancel.Size = ui.VariantOutline, ui.SizeXS
		use := th.Button(&u.confirm, i18n.T("shell.planQuota.resetUse"))
		use.Variant, use.Size, use.Disabled = ui.VariantDestructive, ui.SizeXS, u.busyNow()
		rows := []layout.Widget{
			para(th.Text(ui.TextXS, i18n.T("shell.planQuota.resetConfirm", i18n.Args{"name": resetName(r)}))).Layout,
		}
		if clears := resetClears(r); clears != "" {
			rows = append(rows, para(th.Text(ui.TextXS, clears).Muted()).Layout)
		}
		rows = append(rows, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx C) D { return D{Size: gtx.Constraints.Min} }),
				layout.Rigid(cancel.Layout), ui.Gap(1.5), layout.Rigid(use.Layout))
		})
		return ui.Stack(usageLineGap, rows...)(gtx)
	}
}

// resetClears says which limits a reset of r clears, "" for a grant that names
// none lich knows.
func resetClears(r backend.QuotaReset) string {
	has := func(name string) bool {
		return slices.Contains(r.Clears, name)
	}
	switch {
	case has(api.QuotaClearsSession) && has(api.QuotaClearsWeekly):
		return i18n.T("shell.planQuota.resetClearsFull")
	case has(api.QuotaClearsSession) && has(api.QuotaClearsModel) && r.Label != "":
		return i18n.T("shell.planQuota.resetClearsModel", i18n.Args{"model": r.Label})
	case has(api.QuotaClearsSession):
		return i18n.T("shell.planQuota.resetClearsSession")
	}
	return ""
}

// applyUseReset sends a confirmed spend and tells the person how it went, then
// reads the usage again.
func (u *window) applyUseReset(r resetRequest) {
	if r.grantID == "" {
		return
	}
	go func() {
		defer func() {
			u.sidebar.profile.reset.setBusy("")
			u.model.invalidate()
		}()
		res, err := u.client.QuotaUseReset(u.ctx, r.profile, r.grantID)
		u.toaster.Show(useResetToast(res, err, time.Now()))
		// Whatever the answer, the reset may have gone through: the backend
		// dropped its cache, and the reading is taken again at once.
		u.model.quota.reread(r.key)
	}()
}

// useResetToast is what a spend's answer says: a success for a reset, a note
// for the answers that cleared nothing, the error as it came for a failure.
func useResetToast(res api.UseResetResult, err error, now time.Time) ui.Toast {
	if err != nil {
		return ui.Toast{Variant: ui.ToastError, Title: i18n.T("shell.planQuota.resetFailed", i18n.Args{"error": backend.ErrorText(err)})}
	}
	switch res.Status {
	case api.UseResetReset, api.UseResetAlreadyUsed:
		return ui.Toast{Variant: ui.ToastSuccess, Title: i18n.T("shell.planQuota.resetDone")}
	case api.UseResetNotLimited:
		return ui.Toast{Variant: ui.ToastInfo, Title: i18n.T("shell.planQuota.resetNothing"), Description: i18n.T("shell.planQuota.resetNothingDetail")}
	case api.UseResetCooldown:
		if left := timeLeft(res.CooldownUntil, now); left != "" {
			return ui.Toast{Variant: ui.ToastWarning, Title: i18n.T("shell.planQuota.resetCooldownAt", i18n.Args{"time": left})}
		}
		return ui.Toast{Variant: ui.ToastWarning, Title: i18n.T("shell.planQuota.resetCooldownLater")}
	}
	return ui.Toast{Variant: ui.ToastWarning, Title: i18n.T("shell.planQuota.resetIneligible")}
}
