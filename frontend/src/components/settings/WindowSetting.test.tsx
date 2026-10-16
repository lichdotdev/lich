// @vitest-environment jsdom
//
// The harness is imported first for the reason render-budget.test.tsx names.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { expect, test, vi } from "vitest"
import { WindowSetting, windowKindOf } from "./WindowSetting"

const written: string[] = []
const opened: string[] = []

vi.mock("@/lib/rpc", () => ({
  System: {
    OpenWindow: (kind: string) => {
      opened.push(kind)
      return Promise.resolve()
    },
  },
  Store: {
    GetSetting: () => Promise.resolve(""),
    SetSetting: (key: string, scope: string, value: string) => {
      written.push(`${key}|${scope}|${value}`)
      return Promise.resolve()
    },
  },
}))

test("opens the web window unless native was chosen", () => {
  expect(windowKindOf(undefined)).toBe("web")
  expect(windowKindOf("")).toBe("web")
  expect(windowKindOf("electron")).toBe("web")
  expect(windowKindOf("native")).toBe("native")
})

test("writes the native choice globally", async () => {
  const mounted = await mountBudget(createElement(WindowSetting))
  await mounted.act(() => {})
  expect(document.body.textContent).toContain("Which window lich opens.")

  const native = [...document.querySelectorAll("button")].find(
    (b) => b.textContent === "Native (experimental)",
  )
  await mounted.act(() => native?.click())
  expect(written).toEqual(["window.kind||native"])
  expect(document.body.textContent).toContain("Switch to the native window now?")

  const now = [...document.querySelectorAll("button")].find((b) => b.textContent === "Switch now")
  await mounted.act(() => now?.click())
  expect(opened).toEqual(["native"])
  await mounted.unmount()
})
