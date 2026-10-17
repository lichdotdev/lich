import { renderToStaticMarkup } from "react-dom/server"
import { afterEach, describe, expect, it, vi } from "vitest"
import { resetLocale, setLocale } from "@/lib/i18n/i18n"
import { ru } from "@/lib/i18n/locales/ru"
import { zhCN } from "@/lib/i18n/locales/zh-cn"
import { Trans } from "./Trans"

afterEach(() => {
  vi.unstubAllGlobals()
  resetLocale()
})

describe("Trans", () => {
  it("places markup where the translation puts its placeholder", () => {
    const html = renderToStaticMarkup(
      <Trans k="settings.language.promptSaveFailed" params={{ error: <code>boom</code> }} />,
    )
    expect(html).toBe("Could not save the prompt language: <code>boom</code>")
  })

  it("renders in the current language", () => {
    vi.stubGlobal("localStorage", { setItem: () => {}, getItem: () => null })
    setLocale("pt-BR")
    const html = renderToStaticMarkup(<Trans k="common.count.file" params={{ count: 2 }} />)
    expect(html).toBe("2 arquivos")
  })

  it("renders in Spanish", () => {
    vi.stubGlobal("localStorage", { setItem: () => {}, getItem: () => null })
    setLocale("es")
    const html = renderToStaticMarkup(<Trans k="common.count.file" params={{ count: 2 }} />)
    expect(html).toBe("2 archivos")
  })

  it("renders Simplified Chinese", () => {
    vi.stubGlobal("localStorage", { setItem: () => {}, getItem: () => null })
    setLocale("zh-CN")
    const html = renderToStaticMarkup(<Trans k="common.count.file" params={{ count: 2 }} />)
    expect(html).toBe(zhCN.common.count.file.other.replace("{count}", "2"))
  })

  it("renders Korean", () => {
    vi.stubGlobal("localStorage", { setItem: () => {}, getItem: () => null })
    setLocale("ko")
    const html = renderToStaticMarkup(<Trans k="common.count.file" params={{ count: 2 }} />)
    expect(html).toBe("파일 2개")
  })

  it("renders Russian with the form its count selects", () => {
    vi.stubGlobal("localStorage", { setItem: () => {}, getItem: () => null })
    setLocale("ru")
    const html = renderToStaticMarkup(<Trans k="common.count.file" params={{ count: 5 }} />)
    expect(html).toBe(ru.common.count.file.many.replace("{count}", "5"))
  })
})
