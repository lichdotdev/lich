// @vitest-environment jsdom
//
// The harness is imported first for the reason render-budget.test.tsx names.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { beforeEach, expect, test, vi } from "vitest"
import { WindowFallbackGate } from "./WindowFallbackGate"

const warned: { title: string; description?: string }[] = []
let reason = ""

vi.mock("sonner", () => ({
  toast: {
    warning: (title: string, opts: { description?: string }) =>
      warned.push({ title, description: opts.description }),
  },
}))
vi.mock("@/lib/rpc", () => ({
  System: { TakeWindowFallback: () => Promise.resolve(reason), RevealLog: () => Promise.resolve() },
}))

beforeEach(() => {
  warned.length = 0
})

test("tells why the web window opened in the native one's place", async () => {
  reason = "no lich-native beside /usr/bin/lich"
  const mounted = await mountBudget(createElement(WindowFallbackGate))
  await mounted.act(() => {})
  expect(warned).toEqual([{ title: "Opened the web window", description: reason }])
  await mounted.unmount()
})

test("says nothing when the chosen window opened", async () => {
  reason = ""
  const mounted = await mountBudget(createElement(WindowFallbackGate))
  await mounted.act(() => {})
  expect(warned).toEqual([])
  await mounted.unmount()
})
