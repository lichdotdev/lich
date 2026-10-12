package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/omartelo/lich/native/lichclient"
)

// ponytail: English until the native window has i18n (env.commitIdentity).
const (
	identityNoneLead    = "No git identity in this checkout."
	identityNoneNote    = "Commits will be refused until user.email is set."
	identityLandAsNamed = "Commits land as %s"
	identityLandAs      = "Commits land as"
	identitySetLocal    = "— set in this repository, overriding your global one."
	identitySetGlobal   = "— git user.email, not this account."
)

const defaultGitHubHost = "github.com"

// splitAccount is gh-account.ts splitAccount: "<host>/<login>", the host ""
// for the host-less form stored before accounts carried one.
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

// identityLine is commit-identity.ts commitIdentityRow: the lead, the email
// in angle brackets ("" with no identity), and the note.
type identityLine struct{ lead, email, note string }

func identityLineOf(id lichclient.CommitIdentity) identityLine {
	if id.Email == "" {
		return identityLine{lead: identityNoneLead, note: identityNoneNote}
	}
	l := identityLine{lead: identityLandAs, email: "<" + id.Email + ">", note: identitySetGlobal}
	if id.Name != "" {
		l.lead = fmt.Sprintf(identityLandAsNamed, id.Name)
	}
	if id.Local {
		l.note = identitySetLocal
	}
	return l
}
