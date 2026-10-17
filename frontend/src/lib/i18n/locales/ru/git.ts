import type { Shape } from "../../catalog"
import type { git as en } from "../en/git"

export const git = {
  baseStatus: {
    behind: {
      one: "Позади {base} на {count} коммит",
      few: "Позади {base} на {count} коммита",
      many: "Позади {base} на {count} коммитов",
      other: "Позади {base} на {count} коммита",
    },
    conflict: {
      one: "Конфликты в {count} файле",
      few: "Конфликты в {count} файлах",
      many: "Конфликты в {count} файлах",
      other: "Конфликты в {count} файлах",
    },
  },
  fileSearch: {
    cutMatches: "{count}+ совпадений",
    summary: "{matches} в {files}",
    matches: {
      one: "{count} совпадение",
      few: "{count} совпадения",
      many: "{count} совпадений",
      other: "{count} совпадения",
    },
    cutNote: "Показаны первые {count} строк с совпадениями. Сузьте поиск.",
    tooLarge: {
      one: "{count} файл больше 1 МБ не просмотрен.",
      few: "{count} файла больше 1 МБ не просмотрено.",
      many: "{count} файлов больше 1 МБ не просмотрено.",
      other: "{count} файла больше 1 МБ не просмотрено.",
    },
  },
  fileTree: {
    hidden: "Скрыто: {names}.",
    cut: "В этой папке больше файлов, чем может показать дерево.",
  },
  lastTurn: {
    saidPreviousTurn: "из предыдущего хода",
    noTurnWindow:
      "{name} не сообщает ни о начале, ни о конце хода, поэтому определить окно нельзя.",
  },
  carry: {
    reused:
      "{name} уже существует, поэтому открыто как есть: незакоммиченная работа не перенесена.",
    failed: "Не удалось перенести незакоммиченную работу: {error}",
  },
  codemirror: {
    expand: {
      one: "Развернуть {count} неизменённую строку",
      few: "Развернуть {count} неизменённые строки",
      many: "Развернуть {count} неизменённых строк",
      other: "Развернуть {count} неизменённой строки",
    },
    showLines: "Показать строки {from}–{to}",
    revertTitle: "Откатить это изменение",
    revert: "Откатить",
  },
} satisfies Shape<typeof en>
