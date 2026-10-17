import type { Shape } from "../../catalog"
import type { update as en } from "../en/update"

export const update = {
  plugin: {
    incompatibleInstall:
      "Плагин lich в {installs} не поддерживается этой версией lich. Установите v{version}: именно этот релиз поддерживается.",
    incompatibleUpdate:
      "Плагин lich в {installs} не поддерживается этой версией lich. Обновите lich или проверьте подключение к сети, чтобы найти поддерживаемый релиз.",
  },
  progress: {
    updateFailed: "Не удалось обновить: {error}",
    installFailed: "Не удалось установить: {error}",
    downloadFailed: "Не удалось скачать: {error}",
    downloadFailedAt: "Не удалось скачать на {percent}%: {error}",
    bytesOf: "{received} из {total}",
  },
} satisfies Shape<typeof en>
