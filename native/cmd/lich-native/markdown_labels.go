package main

import (
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui/markdown"
)

func init() {
	markdown.Words = markdown.Labels{
		Alert: markdownAlertName,
		More:  func(count int) string { return i18n.T("common.markdown.moreLines", i18n.Args{"count": count}) },
		Less:  func() string { return i18n.T("common.markdown.showLess") },
	}
}

func markdownAlertName(a markdown.Alert) string {
	switch a {
	case markdown.AlertNote:
		return i18n.T("common.markdown.alert.note")
	case markdown.AlertTip:
		return i18n.T("common.markdown.alert.tip")
	case markdown.AlertImportant:
		return i18n.T("common.markdown.alert.important")
	case markdown.AlertWarning:
		return i18n.T("common.markdown.alert.warning")
	case markdown.AlertCaution:
		return i18n.T("common.markdown.alert.caution")
	}
	return ""
}
