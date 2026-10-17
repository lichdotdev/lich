import type { Shape } from "../../catalog"
import type { session as en } from "../en/session"

export const session = {
  schedule: {
    choice15Minutes: "15 мин",
    choice1Hour: "1 час",
    choice4Hours: "4 часа",
    choiceTomorrow: "Завтра",
    countdown: "через {amount}",
    today: "сегодня {clock}",
    weekdayAt: "{weekday} {clock}",
  },
  limitLine: {
    sessionWindow: "Лимит сессии",
    weeklyWindow: "Недельный лимит",
    usageWindow: "Лимит использования",
  },
  filter: {
    phase: {
      waiting: "ожидающих",
      running: "работающих",
      unread: "непрочитанных",
      idle: "простаивающих",
    },
    phaseJoiner: " или ",
    noMatch: "Сессий нет. Активная сессия остаётся.",
    noMatchQuery: "Нет сессий по запросу «{query}». Активная сессия остаётся.",
    noMatchPhases: "Нет {phases} сессий. Активная сессия остаётся.",
    noMatchPhasesQuery: "Нет {phases} сессий по запросу «{query}». Активная сессия остаётся.",
  },
  cost: {
    mixedModels: "В этом разговоре менялась модель, поэтому lich не может посчитать стоимость.",
    unpricedModel: "Цены на эту модель пока нет: lich нужна сеть, чтобы её получить.",
  },
  handsOn: {
    detailTurn:
      "Сколько времени с этой сессией работали: набирали в ней текст, получали от неё отчёты или шёл ход. Пауза дольше 15 минут считается отсутствием.",
    detailTool:
      "Сколько времени с этой сессией работали: набирали в ней текст или получали отчёт о вызове инструмента. Пауза дольше 15 минут считается отсутствием.",
  },
  spawnGate: {
    checkoutGone:
      "Worktree этой сессии удалён, поэтому сессию закрыли. Создайте worktree заново, чтобы вернуться к ней.",
    conversationGone: "Прежний разговор больше недоступен: запускается новая сессия.",
  },
  fork: {
    unavailable: "{name} не поддерживает форк, только возобновление.",
  },
  palette: {
    untitledConversation: "Разговор без названия",
    groupSessions: "Сессии",
    groupProjects: "Проекты",
    groupMessages: "Сообщения",
    groupOpen: "Открытые",
    groupClosed: "Закрытые",
    groupClosedSessions: "Закрытые сессии",
    groupOutsideLich: "Вне lich",
    indexing: {
      one: "индексируется {count} сессия",
      few: "индексируются {count} сессии",
      many: "индексируются {count} сессий",
      other: "индексируется {count} сессии",
    },
  },
} satisfies Shape<typeof en>
