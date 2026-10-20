package main

import (
	"slices"
	"strings"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/lichclient"
)

const defaultGitHubHost = "github.com"

// splitAccount splits "<host>/<login>", the host "" for the
// host-less form stored before accounts carried one.
func splitAccount(account string) (host, login string) {
	host, login, ok := strings.Cut(account, "/")
	if !ok {
		return "", account
	}
	return host, login
}

// upgradeAccount is the host-carrying form of a host-less stored account,
// "" when it cannot be had without guessing: gh knows no such login, or
// knows it on more than one host.
func upgradeAccount(stored string, accounts []string) string {
	if stored == "" || strings.Contains(stored, "/") {
		return ""
	}
	var match string
	for _, a := range accounts {
		if _, login := splitAccount(a); login == stored {
			if match != "" {
				return ""
			}
			match = a
		}
	}
	return match
}

// accountLabel is how an account reads in the picker: the host only when
// there is more than one to tell apart, or it is not github.com.
func accountLabel(account string, accounts []string) string {
	host, login := splitAccount(account)
	if host == "" {
		return login
	}
	hosts := map[string]bool{}
	for _, a := range accounts {
		h, _ := splitAccount(a)
		hosts[h] = true
	}
	if len(hosts) > 1 || host != defaultGitHubHost {
		return login + " — " + host
	}
	return login
}

// accountOptions are the picker's accounts: gh's, then the stored one if gh
// no longer lists it, so a stale value stays visible and changeable.
func accountOptions(accounts []string, stored string) []string {
	options := slices.Clone(accounts)
	if stored != "" && !slices.Contains(options, stored) {
		options = append(options, stored)
	}
	return options
}

// identityLine is the lead, the email in angle brackets ("" with no identity), and the note.
type identityLine struct{ lead, email, note string }

func identityLineOf(id lichclient.CommitIdentity) identityLine {
	if id.Email == "" {
		return identityLine{lead: i18n.T("env.commitIdentity.noneLead"), note: i18n.T("env.commitIdentity.noneNote")}
	}
	l := identityLine{lead: i18n.T("env.commitIdentity.landAs"), email: "<" + id.Email + ">", note: i18n.T("env.commitIdentity.setGlobal")}
	if id.Name != "" {
		l.lead = i18n.T("env.commitIdentity.landAsNamed", i18n.Args{"name": id.Name})
	}
	if id.Local {
		l.note = i18n.T("env.commitIdentity.setLocal")
	}
	return l
}
