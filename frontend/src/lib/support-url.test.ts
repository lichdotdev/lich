import { describe, expect, it } from "vitest"
import { bugReportUrl } from "./support-url"

describe("bugReportUrl", () => {
  const diagnostics = { version: "0.23.0", platform: "linux/amd64", logPath: "/c/lich/lich.log" }

  it("names the template and prefills version and platform", () => {
    const params = new URL(bugReportUrl(diagnostics)).searchParams
    expect(params.get("template")).toBe("bug.yml")
    expect(params.get("version")).toBe("0.23.0")
    // The slash has to survive the round trip: a raw one would end the path.
    expect(params.get("platform")).toBe("linux/amd64")
  })

  it("points at the repository's new-issue form", () => {
    const url = new URL(bugReportUrl(diagnostics))
    expect(url.origin).toBe("https://github.com")
    expect(url.pathname).toBe("/lichdotdev/lich/issues/new")
  })

  it("carries a dev build through instead of dropping the field", () => {
    // A dev build reports "dev", and that is worth seeing in the issue: it says
    // the reporter is not on a release.
    const params = new URL(bugReportUrl({ ...diagnostics, version: "dev" })).searchParams
    expect(params.get("version")).toBe("dev")
  })
})
