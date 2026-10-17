import type { Shape } from "../../catalog"
import type { palette as en } from "../en/palette"

export const palette = {
  commandPalette: {
    title: "Палитра команд",
    placeholder: "Перейти к сессии, проекту или сказанному, либо введите > для действий…",
    searchLabel: "Поиск по сессиям и проектам",
    resultsLabel: "Результаты",
    filterLabel: "Фильтр результатов",
    forgetFailed: "Не удалось забыть {label}: {error}",
    resumeFailed: "Не удалось возобновить {label}: {error}",
    actionUnavailable: "{label} здесь недоступно",
    showBeside: "показать рядом",
    nothingClosed: "Закрытых пока нет",
    nothingClosedHint:
      "Закройте сессию, и она будет ждать здесь вместе со своей веткой, агентом и разговором, пока её worktree не удалят.",
    noMatches: "Нет совпадений для {query}",
    noMatchesInTab: "Нет совпадений для {query} (вкладка {tab})",
    shownOfTotal: "{shown} из {total}",
    tab: {
      All: "Все",
      Sessions: "Сессии",
      Projects: "Проекты",
      Messages: "Сообщения",
      History: "История",
    },
    checkoutGone: "рабочей копии нет",
    matches: {
      one: "{count} совпадение",
      few: "{count} совпадения",
      many: "{count} совпадений",
      other: "{count} совпадения",
    },
    sessions: {
      one: "{count} сессия",
      few: "{count} сессии",
      many: "{count} сессий",
      other: "{count} сессии",
    },
    relocate: "переместить",
    reopen: "открыть снова",
  },
} satisfies Shape<typeof en>
