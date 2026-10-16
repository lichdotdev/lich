import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { useT } from "@/lib/i18n/i18n"
import { isMac } from "@/lib/platform"
import { useStoredSetting } from "@/lib/use-stored-setting"
import { SettingBlock } from "./SettingBlock"

const GLOBAL_SCOPE = ""

// system.WindowKindKey and its two windows.
export const WINDOW_KIND_SETTING_KEY = "window.kind"
const WINDOW_KINDS = ["web", "native"] as const

/** The stored window choice: the web window unless native was picked. */
export function windowKindOf(stored: string | undefined): (typeof WINDOW_KINDS)[number] {
  return stored === "native" ? "native" : "web"
}

// WindowSetting picks the window lich opens. macOS has no native window
// build, so the choice is not offered there.
export function WindowSetting() {
  const t = useT()
  const [stored, persist] = useStoredSetting(WINDOW_KIND_SETTING_KEY, GLOBAL_SCOPE)
  if (isMac) return null
  const choice = windowKindOf(stored)

  return (
    <SettingBlock
      title={t("settings.windowSetting.title")}
      description={t("settings.windowSetting.description")}
    >
      <ToggleGroup
        value={[choice]}
        // An empty array is the pressed choice pressed again: keep it.
        onValueChange={(next) => next[0] && void persist(next[0])}
        spacing={1}
        aria-label={t("settings.windowSetting.title")}
        className="border border-border p-[0.1875rem]"
      >
        {WINDOW_KINDS.map((k) => (
          <ToggleGroupItem key={k} value={k} size="sm">
            {t(`settings.windowSetting.choice.${k}.label`)}
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
    </SettingBlock>
  )
}
