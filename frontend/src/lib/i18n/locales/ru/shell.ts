import type { Shape } from "../../catalog"
import type { shell as en } from "../en/shell"

export const shell = {
  agentPluginGate: {
    updateAvailable: "Доступен плагин lich {version}",
    update: "Обновить",
    later: "Позже",
    installVersion: "Установить v{version}",
    updating: "Обновление плагина lich…",
    updated: "Плагин обновлён. Перезапустите сессии, чтобы применить.",
    updateFailed: "Не удалось обновить",
    installed: "Плагин установлен. Перезапустите сессии, чтобы применить.",
    codexTrust: "Codex: выполните /hooks в сессии Codex, чтобы доверять хукам плагина.",
    installFailed: "Не удалось установить: {error}",
    title: "Включите интеграцию с агентами",
    description:
      "Плагин lich сообщает приложению о статусе и названиях ваших сессий, обновляет их статус git и позволяет восстановленной сессии продолжить разговор. Установите его в те CLI, которыми пользуетесь.",
    needsHooks: "нужно доверять через /hooks",
    dontAskAgain: "Больше не спрашивать",
    notNow: "Не сейчас",
    installing: "Установка…",
    install: "Установить",
  },
  appUpdateGate: {
    available: "Доступен lich {version}",
    updateAndInstall: "Обновить и установить",
    later: "Позже",
    updated: "lich обновлён до {version}",
    retry: "Повторить",
    restart: "Перезапустить",
    restartFailed: "Не удалось перезапустить: {error}",
    install: "Установить",
    viewRelease: "Открыть релиз",
  },
  confirmDialog: {
    cancel: "Отмена",
  },
  emptySessions: {
    title: "Нет открытых сессий",
    description: "Откройте сессию, чтобы начать работу в этом проекте.",
    noAgent:
      "Агент в PATH не найден, поэтому откроется терминал. Укажите агента в разделе Настройки › Провайдеры.",
    newTerminal: "Новый терминал",
    newSession: "Новая сессия",
  },
  fileTree: {
    openInEditor: "Открыть в редакторе",
    expandAll: "Развернуть все",
    collapseAll: "Свернуть все",
  },
  footerBar: {
    attachFailed: "Не удалось прикрепить файл",
    attachFile: "Прикрепить файл",
    browseCode: "Обзор кода",
    reviewChanges: "Просмотр изменений",
    viewPullRequest: "Открыть pull request",
    pullRequest: "PR #{number}",
    leftFooter: "Левый нижний ряд",
    rightFooter: "Правый нижний ряд",
  },
  footerSession: {
    contextWindow: "Контекстное окно",
    contextTokens: "{tokens} / {window} токенов",
    costUnavailable: "Стоимость недоступна",
    sessionCost: "Стоимость сессии",
    costOfBudget: "{cost} из бюджета {budget}",
    costNote:
      "Стоимость API по всем разговорам этой сессии. Списания по подписке могут отличаться.",
    handsOnTime: "Время за работой",
    handsOnFailed: "Не удалось прочитать время за работой.",
  },
  gitMissingGate: {
    title: "git не установлен",
    description:
      "lich читает все ветки, изменения и worktree через git. Сессии работают и без него, но разделы контроля версий остаются пустыми.",
    notNow: "Не сейчас",
    installGit: "Установить git",
  },
  home: {
    title: "Нет открытых проектов",
    description: "Откройте папку, чтобы запустить сессию терминала.",
    openProject: "Открыть проект",
  },
  notificationsOptIn: {
    title: "Уведомлять, когда сессии что-то нужно от вас?",
    description:
      "Сессия только что запросила ваш ответ, пока вы были в другом месте. lich может показать уведомление на рабочем столе с названием сессии и проекта, чтобы вы могли запустить задачу и отойти от окна. Сессии, на которые вы смотрите, не уведомляют. Это можно изменить в любой момент в разделе Настройки › Уведомления.",
    noThanks: "Нет, спасибо",
    notifyMe: "Уведомлять",
  },
  paneSeams: {
    resizeColumns: "Изменить ширину столбцов",
    resizeRows: "Изменить высоту строк",
  },
  patchNotesDialog: {
    title: "Что нового в lich {version}",
    viewChangelog: "Открыть полный журнал изменений",
    gotIt: "Понятно",
  },
  planQuota: {
    planUsage: "Использование тарифа",
    locked: "Заблокировано",
  },
  providerSetupDialog: {
    chooseTitle: "Выберите агентов",
    noneTitle: "Агенты для кода не найдены",
    chooseDescription:
      "lich нашёл на вашем компьютере этих агентов. Включите те, которыми пользуетесь: остальные не попадут в меню «Новая сессия».",
    noneDescription:
      "lich запускает агентов, уже установленных на вашем компьютере, и ни одного не нашёл. Установите одного и проверьте снова.",
    changeLater: "Это можно изменить в любой момент в разделе Настройки › Провайдеры.",
    pointToBinary:
      "Агент уже установлен? Укажите lich его исполняемый файл в разделе Настройки › Провайдеры.",
    continue: "Продолжить",
  },
  quotaGauge: {
    aheadOfPace: "Опережает темп",
    aheadTooltip: "Расход идёт быстрее темпа этого окна.",
    locked: "Заблокировано",
  },
  resumeSessionDialog: {
    title: "Возобновить предыдущую сессию?",
    description:
      "После {label} остался разговор ({id}). Возобновите его, чтобы продолжить с того места, где он остановился, или начните новую сессию с чистого листа.",
    alwaysFor: "Всегда так делать для {provider}",
    changeInSettings: "· изменить в разделе Настройки › Провайдеры",
    thisProvider: "этого провайдера",
    startNew: "Начать новую",
    resume: "Возобновить",
  },
  shortcutsOverlay: {
    title: "Сочетания клавиш",
    rebind: "Изменить в разделе Настройки › Сочетания клавиш",
    close: "закрыть",
  },
  quitDialog: {
    title: "Выйти из lich?",
    description:
      "Все сессии завершатся вместе с ним. Если закрыть окно, они продолжат работать в фоне.",
    working: {
      one: "{count} сессия посреди хода и потеряет его:",
      few: "{count} сессии посреди хода и потеряют его:",
      many: "{count} сессий посреди хода и потеряют его:",
      other: "{count} сессии посреди хода и потеряют его:",
    },
    confirm: "Выйти из lich",
    failed: "Не удалось выйти из lich",
  },
  tray: {
    show: "Показать lich",
    running: "Запущено сессий: {count}",
    quit: "Выйти из lich",
    openNative: "Открыть нативное окно",
    openWeb: "Открыть веб-окно",
  },
  windowFallback: {
    title: "Открыто веб-окно",
    openLog: "Открыть журнал",
  },
  closeDialog: {
    title: "Оставить lich работать?",
    running: {
      one: "Запущена {count} сессия. В фоне она продолжит работать, а при повторном открытии lich вернётся на экран.",
      few: "Запущено {count} сессии. В фоне они продолжат работать, а при повторном открытии lich вернутся на экран.",
      many: "Запущено {count} сессий. В фоне они продолжат работать, а при повторном открытии lich вернутся на экран.",
      other:
        "Запущено {count} сессии. В фоне они продолжат работать, а при повторном открытии lich вернутся на экран.",
    },
    dontAsk: "Больше не спрашивать",
    changeInSettings: "· изменить в разделе Настройки › Оформление",
    keep: "Оставить работать",
    quit: "Выйти из lich",
    failed: "Не удалось закрыть окно",
  },
  uncleanExitGate: {
    title: "Предыдущий запуск lich завершился неожиданно",
    description:
      "Ваши сессии восстановлены, но всё, что в них выполнялось, остановилось вместе с ним.",
    openLogFolder: "Открыть папку журнала",
    opening: "Открытие папки журнала…",
    opened: "Папка журнала открыта.",
    openFailed: "Не удалось открыть папку журнала",
    dismiss: "Закрыть",
  },
  updateProgressToast: {
    downloading: "Загрузка lich {version}",
    installing: "Установка lich {version}…",
    reopens: "lich сам закроется и откроется снова.",
  },
  app: {
    stage: "Сцена",
    reloadStage: "Перезагрузить сцену",
    screen: "Этот экран",
    panel: "Панель",
  },
  projects: {
    relocated: "{name} теперь открывает {path}",
    relocateFailed: "Не удалось переместить: {error}",
    noLongerAvailable: "{label} больше нельзя возобновить",
    bringBackFailed: "Не удалось вернуть {label}",
    closed: "Закрыто: {label}",
    undo: "Отменить",
    entrypointSet: "Точка входа задана",
    entrypointSetDescription: "Запустится при следующем старте этого терминала.",
    entrypointCleared: "Точка входа сброшена",
    entrypointClearedDescription: "Этот терминал снова запускает обычную оболочку.",
    entrypointFailed: "Не удалось сохранить точку входа: {error}",
    scheduled: "Запланировано {when}",
    scheduledNow: "Запланировано на сейчас",
    scheduleCleared: "Расписание сброшено",
    scheduleFailed: "Не удалось запланировать: {error}",
  },
  projectEvents: {
    unlabeledSession: "Сессия",
    scheduleLost: "Запланированный промпт потерян вместе с {label}",
    scheduleDue: "Срок {when}: {prompt}",
    relayAnswered: "Ответ в собственной сессии: {target}",
    relayDetail: "Работа завершена без ответа через lich: откройте сессию, чтобы прочитать ответ.",
    open: "Открыть",
    needsInput: "{label}: нужен ваш ответ",
    finished: "{label}: работа завершена",
  },
} satisfies Shape<typeof en>
