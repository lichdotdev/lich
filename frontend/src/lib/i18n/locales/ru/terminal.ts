import type { Shape } from "../../catalog"
import type { terminal as en } from "../en/terminal"

export const terminal = {
  dropHint: {
    attachTo: "Прикрепить к {label}",
    confined: "Вне рабочей копии файл попадёт как копия",
    pasted: "Его путь будет вставлен в строку ввода",
  },
  exitBanner: {
    restart: "Перезапустить",
    close: "Закрыть",
  },
  searchBar: {
    placeholder: "Найти",
    label: "Поиск в терминале",
    previous: "Предыдущее совпадение",
    next: "Следующее совпадение",
    close: "Закрыть поиск",
  },
  drop: {
    notAttached: "Не прикреплено: {files}",
  },
  host: {
    stopShowing: "Перестать показывать {name}",
    paneName: "Панель {name}",
    rememberFailed: "Не удалось запомнить выбор: {error}",
    showBeside: "Показать сессию рядом с этой",
    noRoom: "Нет места для ещё одной панели",
  },
  showBesidePicker: {
    title: "Показать рядом",
    placeholder: "Поиск сессий для показа рядом…",
    search: "Поиск сессий для показа рядом",
    results: "Сессии, которые можно показать рядом",
    pick: "показать",
    noMatch: "Нет сессий по запросу {query}",
    onWall: "в {group}",
  },
  view: {
    restartFailed: "Не удалось перезапустить сессию: {error}",
    startFailed: "Не удалось запустить сессию: {error}",
    pasteSettingFailed:
      "Не удалось узнать, раскрывать ли длинные вставки, поэтому эта вставлена как есть: {error}",
  },
  sessionExit: {
    ended: "Сессия завершена",
    endedWithCode: "Сессия завершена с кодом {code}",
  },
  dropFiles: {
    folderConfined:
      "папки вне рабочей копии этой сессии в песочнице передать нельзя; перетащите файлы",
    folderMissing:
      "папка не найдена ни в этой сессии, ни в вашем домашнем каталоге; перетащите файлы",
  },
  copyToast: {
    copied: {
      one: "скопирован {count} символ в буфер обмена",
      few: "скопировано {count} символа в буфер обмена",
      many: "скопировано {count} символов в буфер обмена",
      other: "скопировано {count} символа в буфер обмена",
    },
  },
} satisfies Shape<typeof en>
