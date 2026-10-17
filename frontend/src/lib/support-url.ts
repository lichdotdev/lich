// The links behind Settings → Help.
//
// A bug report is a browser hand-off, never a `gh` call. Two reasons, and both
// outlive the convenience: `gh issue create` cannot attach a file, and the log
// file is the attachment; and a project can name a gh account other than the
// logged-in one (the vcs.account ceiling), so "which account files this?" has
// no answer a single button can pick. lich prefills the form; the user sends it.

import type { Diagnostics } from "./api-types"

export const REPO_URL = "https://github.com/lichdotdev/lich"

// bugReportUrl opens the bug form with the two fields a reporter should never
// have to look up. The query keys are the `id`s in .github/ISSUE_TEMPLATE/bug.yml
// — GitHub silently ignores a key that matches no field, so they move together.
export function bugReportUrl(diagnostics: Diagnostics): string {
  const params = new URLSearchParams({
    template: "bug.yml",
    version: diagnostics.version,
    platform: diagnostics.platform,
  })
  return `${REPO_URL}/issues/new?${params}`
}
