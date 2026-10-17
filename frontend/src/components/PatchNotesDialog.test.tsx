// @vitest-environment jsdom
//
// The harness is imported first for the reason render-budget.test.tsx names.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { afterEach, expect, test, vi } from "vitest"
import type { PatchNotes } from "@/lib/api-types"
import { resetLocale, setLocale } from "@/lib/i18n/i18n"
import { PatchNotesDialog } from "./PatchNotesDialog"

vi.mock("@/lib/rpc", () => ({ System: { OpenExternal: () => Promise.resolve(null) } }))

afterEach(() => resetLocale())

const notes: PatchNotes = {
  version: "0.69.0",
  highlights: null,
  groups: [
    { label: "Fixed", items: ["**Uma correção.**", "**Outra.**"] },
    { label: "Highlights", items: ["**Fora das seis.**"] },
  ],
}

test("names Keep a Changelog's groups in the interface language", async () => {
  setLocale("pt-BR")
  const mounted = await mountBudget(createElement(PatchNotesDialog, { notes, onClose: () => {} }))
  const text = document.body.textContent
  expect(text).toContain("2 correções")
  expect(text).toContain("Correções")
  expect(text).toContain("1 highlights")
  await mounted.unmount()
})
