package main

import (
	"image/color"
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

// installWarning is the confirmation's body: that it runs as you, then what
// else it can do.
func installWarning(gtx C, th *ui.Theme, permissions []string) D {
	warning := th.Text(ui.TextSM, i18n.T("settings.extensionsSettings.installWarning")).Muted().Wrapped()
	if len(permissions) == 0 {
		return warning.Layout(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(warning.Layout),
		ui.VGap(4),
		layout.Rigid(func(gtx C) D { return permissionList(gtx, th, permissions) }),
	)
}

// permissionList is the uppercase label over one line per permission, the
// ones that act in the wait tone.
func permissionList(gtx C, th *ui.Theme, permissions []string) D {
	label := strings.ToUpper(i18n.T("settings.extensionsSettings.permissionsLabel"))
	children := []layout.FlexChild{
		layout.Rigid(th.Text(text2XS, label).Weight(font.SemiBold).Leading(text2XSLeading).Muted().Layout),
	}
	for _, p := range permissions {
		children = append(children, ui.VGap(1), layout.Rigid(th.Text(ui.TextSM, permissionText(p)).In(permissionTone(th, p)).Wrapped().Layout))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
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
