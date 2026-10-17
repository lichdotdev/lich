import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { getLocale, resetLocale, UI_LANGUAGE_PREF } from "@/lib/i18n/i18n"
import { saveLocale, seedUiLanguageSetting } from "./ui-language-setting"

const getSetting = vi.fn()
const setSetting = vi.fn()
vi.mock("@/lib/rpc", () => ({
  Store: {
    GetSetting: (key: string, projectID: string) => getSetting(key, projectID),
    SetSetting: (key: string, projectID: string, value: string) =>
      setSetting(key, projectID, value),
  },
}))

function stubStorage(initial: Record<string, string> = {}) {
  const data = new Map(Object.entries(initial))
  vi.stubGlobal("localStorage", {
    getItem: (key: string) => data.get(key) ?? null,
    setItem: (key: string, value: string) => data.set(key, value),
    removeItem: (key: string) => data.delete(key),
  })
  return data
}

beforeEach(() => {
  resetLocale()
  getSetting.mockReset().mockResolvedValue("")
  setSetting.mockReset().mockResolvedValue(null)
})
afterEach(() => {
  vi.unstubAllGlobals()
  resetLocale()
})

describe("saveLocale", () => {
  it("switches the page and writes the global setting", async () => {
    const storage = stubStorage()
    await saveLocale("pt-BR")
    expect(getLocale()).toBe("pt-BR")
    expect(storage.get(UI_LANGUAGE_PREF)).toBe("pt-BR")
    expect(setSetting).toHaveBeenCalledWith("lich.appearance.language", "", "pt-BR")
  })

  it("rejects when the backend refuses the write", async () => {
    stubStorage()
    setSetting.mockRejectedValue(new Error("down"))
    await expect(saveLocale("es")).rejects.toThrow("down")
    expect(getLocale()).toBe("es")
  })
})

describe("seedUiLanguageSetting", () => {
  it("copies a stored choice into an empty setting", async () => {
    stubStorage({ [UI_LANGUAGE_PREF]: "zh-CN" })
    await seedUiLanguageSetting()
    expect(getSetting).toHaveBeenCalledWith("lich.appearance.language", "")
    expect(setSetting).toHaveBeenCalledWith("lich.appearance.language", "", "zh-CN")
  })

  it("never overwrites a value already in the setting", async () => {
    stubStorage({ [UI_LANGUAGE_PREF]: "zh-CN" })
    getSetting.mockResolvedValue("es")
    await seedUiLanguageSetting()
    expect(setSetting).not.toHaveBeenCalled()
  })

  it("does nothing without a stored choice", async () => {
    stubStorage()
    vi.stubGlobal("navigator", { language: "pt-BR" })
    await seedUiLanguageSetting()
    expect(getSetting).not.toHaveBeenCalled()
    expect(setSetting).not.toHaveBeenCalled()
  })
})
