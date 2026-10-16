import { useState } from "react"
import { toast } from "sonner"
import { ConfirmDialog } from "@/components/ConfirmDialog"
import { Button } from "@/components/ui/button"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { errorText } from "@/lib/utils"
import { useT } from "@/lib/i18n/i18n"
import { isMac } from "@/lib/platform"
import { System } from "@/lib/rpc"
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

// WindowSetting picks the window lich opens. Choosing the native one from
// this window offers to switch now; Later leaves it for the next time lich
// opens its window. macOS has no native window build, so the choice is not
// offered there.
export function WindowSetting() {
  const t = useT()
  const [stored, persist] = useStoredSetting(WINDOW_KIND_SETTING_KEY, GLOBAL_SCOPE)
  const [offer, setOffer] = useState(false)
  if (isMac) return null
  const choice = windowKindOf(stored)

  const choose = (next: string) => {
    void persist(next)
    setOffer(next === "native")
  }
  const switchNow = () => {
    setOffer(false)
    void System.OpenWindow("native").catch((error) =>
      toast.error(`${t("settings.windowSetting.switch.failed")}: ${errorText(error)}`),
    )
  }

  return (
    <SettingBlock
      title={t("settings.windowSetting.title")}
      description={t("settings.windowSetting.description")}
    >
      <ToggleGroup
        value={[choice]}
        // An empty array is the pressed choice pressed again: keep it.
        onValueChange={(next) => next[0] && choose(next[0])}
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
      <ConfirmDialog
        open={offer}
        onCancel={() => setOffer(false)}
        title={t("settings.windowSetting.switch.title")}
        description={t("settings.windowSetting.switch.description")}
        cancelLabel={t("settings.windowSetting.switch.later")}
      >
        <Button onClick={switchNow}>{t("settings.windowSetting.switch.now")}</Button>
      </ConfirmDialog>
    </SettingBlock>
  )
}
