package main

import (
	"testing"

	"gioui.org/layout"

	"github.com/lichdotdev/lich/native/ui"
)

// gitFrames is the stage column's two bottom rows, the Git panel over the
// footer that opens it, in the order body lays them out.
func gitFrames(t *testing.T) (*window, *frameRig) {
	t.Helper()
	r := newFrameRig(rigWindow, nil)
	_, client := newSidebarBackend(t)
	u := &window{ctx: t.Context(), client: client, model: newModel(nil, r.invalidate), settings: new(settingsScreen)}
	u.git = newGitPanel(u.ctx, client, &u.root, r.invalidate, func(ui.Toast) {}, "monospace", 12)
	u.footer = newFooter(u.ctx, client, &u.root, newFooterUsage(), newPlanQuota(quotaReader(client), r.invalidate), r.invalidate)
	u.footer.git, u.footer.dock = u.git, new(dock)
	v := stageView{in: busySession()}
	r.draw = func(gtx C, th *ui.Theme) {
		u.theme = th
		u.root.Layout(gtx, func(gtx C) D {
			u.actGit(gtx)
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Flexed(1, func(gtx C) D { return D{Size: gtx.Constraints.Max} }),
				layout.Rigid(func(gtx C) D { return u.gitPanelRow(gtx, v) }),
				layout.Rigid(func(gtx C) D { return u.footer.layoutBar(gtx, th, v.in) }),
			)
		})
	}
	return u, r
}

func TestFooterBranchOpensTheGitPanelInTheFrameThatTookTheClick(t *testing.T) {
	u, r := gitFrames(t)
	r.clickLabel(t, "main")
	if !u.git.open {
		t.Fatal("the click missed the branch")
	}
}
