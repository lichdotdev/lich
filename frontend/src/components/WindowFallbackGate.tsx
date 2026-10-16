import { useEffect } from "react"
import { toast } from "sonner"
import { System } from "@/lib/rpc"
import { runWithToast } from "@/lib/toast-async"
import { t } from "@/lib/i18n/i18n"

// WindowFallbackGate says why this web window opened when the native one was
// chosen: the native window was missing or ended before it opened. The
// backend hands the reason over once, so a reload does not tell it again.
export function WindowFallbackGate() {
  useEffect(() => {
    void System.TakeWindowFallback()
      .then((reason) => {
        if (reason) tell(reason)
      })
      .catch(() => {})
  }, [])

  return null
}

const tell = (reason: string) => {
  toast.warning(t("shell.windowFallback.title"), {
    description: reason,
    duration: Infinity,
    action: {
      label: t("shell.windowFallback.openLog"),
      onClick: () =>
        void runWithToast(
          t("settings.helpSettings.openingLogFolder"),
          System.RevealLog,
          t("settings.helpSettings.logFolderOpened"),
          t("settings.helpSettings.openLogFolderFailed"),
        ),
    },
  })
}
