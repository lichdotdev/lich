// @vitest-environment jsdom
//
// The New-worktree dialog's setup row, mounted for real: the frontend gate is
// otherwise node-only, so a component that throws on render passes a green
// suite.
//
// The harness is imported first for the reason render-budget.test.tsx names: it
// has to hook react-dom before anything else reaches it.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { beforeEach, expect, test, vi } from "vitest"
import { WorktreeSetupRow } from "./WorktreeSetupRow"

const setup = vi.hoisted(() => ({
  script: "",
  suggestion: "",
  detected: "",
  saved: [] as Array<[string, string]>,
}))

vi.mock("@/lib/rpc", () => ({
  ProjectService: {
    WorktreeSetup: () =>
      Promise.resolve({
        script: setup.script,
        suggestion: setup.suggestion || undefined,
        detected: setup.detected || undefined,
      }),
    SaveWorktreeSetup: (path: string, value: string) => {
      setup.saved.push([path, value])
      setup.script = value
      return Promise.resolve(null)
    },
  },
}))

beforeEach(() => {
  setup.script = ""
  setup.suggestion = ""
  setup.detected = ""
  setup.saved = []
})

const text = () => document.body.textContent ?? ""

test("a shipped setup script is shown with its file", async () => {
  setup.script = "pnpm install"
  const row = await mountBudget(createElement(WorktreeSetupRow, { projectPath: "/src/lich" }))

  expect(text()).toContain("pnpm install")
  expect(text()).toContain(".lich/setup-worktree.sh")
  await row.unmount()
})

test("Use saves the detected suggestion as the setup script", async () => {
  setup.suggestion = "pnpm install"
  setup.detected = "pnpm-lock.yaml"
  const row = await mountBudget(createElement(WorktreeSetupRow, { projectPath: "/src/lich" }))

  expect(text()).toContain("pnpm-lock.yaml")
  const use = [...document.querySelectorAll("button")].find((b) => b.textContent === "Use")
  if (!use) {
    throw new Error("Use button not rendered")
  }
  await row.act(async () => {
    use.click()
  })
  expect(setup.saved).toEqual([["/src/lich", "pnpm install"]])
  expect(text()).toContain(".lich/setup-worktree.sh")
  await row.unmount()
})
