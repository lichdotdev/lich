package main

import (
	"image/color"
	"slices"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"

	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// installTitle names the staged extension and its version, when it has one.
func installTitle(p backend.ExtensionInstallPreview) string {
	if p.Version == "" {
		return i18n.T("settings.extensionsSettings.installTitleNoVersion", i18n.Args{"name": p.Name})
	}
	return i18n.T("settings.extensionsSettings.installTitle", i18n.Args{"name": p.Name, "version": p.Version})
}

// previewSourceLine is where a staged extension comes from, in the words the
// installed row will use.
func previewSourceLine(p backend.ExtensionInstallPreview) string {
	return extensionSourceLine(backend.Extension{Source: p.Source, Release: p.Release})
}

// previewSource is the confirmation's description: where the staged
// extension comes from and, from GitHub, who publishes it.
func previewSource(p backend.ExtensionInstallPreview) string {
	if p.Publisher == "" {
		return i18n.T("settings.extensionsSettings.installSource", i18n.Args{"source": previewSourceLine(p)})
	}
	return i18n.T("settings.extensionsSettings.installSourcePublisher", i18n.Args{"source": previewSourceLine(p), "publisher": p.Publisher})
}

// previewWarning is installWarning, with the catalog's word on a listed
// release under its first paragraph.
func previewWarning(gtx C, th *ui.Theme, p backend.ExtensionInstallPreview, added []string) D {
	if !p.Listed {
		return installWarning(gtx, th, p.Permissions, added)
	}
	return installBody(gtx, th, []string{i18n.T("settings.extensionsSettings.installListed")}, p.Permissions, added)
}

// installWarning is the confirmation's body: that it runs as you, then what
// else it can do, the permissions in added marked as new.
func installWarning(gtx C, th *ui.Theme, permissions, added []string) D {
	return installBody(gtx, th, nil, permissions, added)
}

// installBody is the warning, then each note, then the permission list, a
// paragraph's space apart.
func installBody(gtx C, th *ui.Theme, notes, permissions, added []string) D {
	children := []layout.FlexChild{
		layout.Rigid(th.Text(ui.TextSM, i18n.T("settings.extensionsSettings.installWarning")).Muted().Wrapped().Layout),
	}
	for _, note := range notes {
		children = append(children, ui.VGap(4), layout.Rigid(th.Text(ui.TextSM, note).Muted().Wrapped().Layout))
	}
	if len(permissions) > 0 {
		children = append(children, ui.VGap(4), layout.Rigid(func(gtx C) D { return permissionList(gtx, th, permissions, added) }))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// newPermissions is what wanted lists that granted does not.
func newPermissions(granted, wanted []string) []string {
	var added []string
	for _, p := range wanted {
		if !slices.Contains(granted, p) {
			added = append(added, p)
		}
	}
	return added
}

// permissionList is the uppercase label over one line per permission, the
// ones that act in the wait tone and the ones in added marked as new.
func permissionList(gtx C, th *ui.Theme, permissions, added []string) D {
	label := strings.ToUpper(i18n.T("settings.extensionsSettings.permissionsLabel"))
	children := []layout.FlexChild{
		layout.Rigid(th.Text(ui.Text2XS, label).Weight(font.SemiBold).Leading(text2XSLeading).Muted().Layout),
	}
	for _, p := range permissions {
		line := th.Text(ui.TextSM, permissionText(p)).In(permissionTone(th, p)).Wrapped().Layout
		if slices.Contains(added, p) {
			line = markedNew(th, line)
		}
		children = append(children, ui.VGap(1), layout.Rigid(line))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// markedNew is a permission's line with the mark of one the installed
// version does not have.
func markedNew(th *ui.Theme, line layout.Widget) layout.Widget {
	mark := th.Text(ui.TextXS, i18n.T("settings.extensionsSettings.permissionNew")).Muted()
	return func(gtx C) D {
		return layout.Flex{Alignment: layout.Baseline}.Layout(gtx, layout.Flexed(1, func(gtx C) D {
			gtx.Constraints.Min.X = 0
			return line(gtx)
		}), ui.Gap(2), layout.Rigid(mark.Layout))
	}
}

func permissionTone(th *ui.Theme, p string) color.NRGBA {
	switch extension.Permission(p) {
	case extension.PermissionSessionsWrite, extension.PermissionSessionsAsk, extension.PermissionServicesWrite, extension.PermissionWindow:
		return th.ToneWait
	}
	return th.Foreground
}

// permissionText is lich's own sentence for a permission. A name this window
// does not know, from a newer backend, shows as itself.
func permissionText(p string) string {
	switch extension.Permission(p) {
	case extension.PermissionProjectsRead:
		return i18n.T("settings.extensionsSettings.permission.projectsRead")
	case extension.PermissionSessionsRead:
		return i18n.T("settings.extensionsSettings.permission.sessionsRead")
	case extension.PermissionSessionsWrite:
		return i18n.T("settings.extensionsSettings.permission.sessionsWrite")
	case extension.PermissionSessionEvents:
		return i18n.T("settings.extensionsSettings.permission.sessionEvents")
	case extension.PermissionSessionsAsk:
		return i18n.T("settings.extensionsSettings.permission.sessionsAsk")
	case extension.PermissionServicesRead:
		return i18n.T("settings.extensionsSettings.permission.servicesRead")
	case extension.PermissionServicesWrite:
		return i18n.T("settings.extensionsSettings.permission.servicesWrite")
	case extension.PermissionGitRead:
		return i18n.T("settings.extensionsSettings.permission.gitRead")
	case extension.PermissionNotify:
		return i18n.T("settings.extensionsSettings.permission.notify")
	case extension.PermissionWindow:
		return i18n.T("settings.extensionsSettings.permission.window")
	}
	return p
}
