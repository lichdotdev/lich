import type { Shape } from "../../catalog"
import type { common as en } from "../en/common"

export const common = {
  count: {
    file: {
      one: "{count} файл",
      few: "{count} файла",
      many: "{count} файлов",
      other: "{count} файла",
    },
    commit: {
      one: "{count} коммит",
      few: "{count} коммита",
      many: "{count} коммитов",
      other: "{count} коммита",
    },
  },
  time: {
    justNow: "только что",
  },
  action: {
    cancel: "Отмена",
    save: "Сохранить",
    create: "Создать",
  },
  checkAgainButton: {
    check: "Проверить снова",
    checking: "Проверка…",
  },
  errorBoundary: {
    stoppedRendering: "{label}: ошибка отрисовки",
    reload: "Перезагрузить окно",
    retry: "Повторить",
  },
  stepper: {
    default: "По умолчанию",
  },
  toolMissing: {
    notInstalled: "{label} не установлен",
    install: "Установить {bin}",
  },
  pickerDialog: {
    navigate: "навигация",
    filter: "фильтр",
    close: "закрыть",
  },
  ui: {
    dialog: {
      close: "Закрыть",
    },
  },
} satisfies Shape<typeof en>
