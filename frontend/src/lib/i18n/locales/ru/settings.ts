import type { Shape } from "../../catalog"
import type { settings as en } from "../en/settings"

export const settings = {
  language: {
    uiTitle: "Язык интерфейса",
    uiDescription: "Язык собственных экранов lich.",
    promptTitle: "Язык промптов",
    promptDescription:
      "Язык текста, который lich передаёт вашим агентам: вводные инструкции, пересылаемые сообщения и уведомления. Сессии, запущенные с этого момента, используют его везде; уже открытые получают его только в пересылаемых сообщениях.",
    uiSearchWords: "язык локаль перевод русский английский language locale translation",
    promptSearchWords:
      "язык локаль перевод агент инструкции пересылка русский английский language locale translation agent briefing relay",
    uiSaveFailed: "Не удалось сохранить язык интерфейса: {error}",
    promptSaveFailed: "Не удалось сохранить язык промптов: {error}",
  },
  settings: {
    search: "Поиск настроек",
    resultCount: {
      one: "{count} настройка",
      few: "{count} настройки",
      many: "{count} настроек",
      other: "{count} настройки",
    },
    noMatch:
      "По запросу {query} ничего нет. Поиск идёт по названиям настроек, а не по их значениям.",
    section: {
      appearance: "Оформление",
      notifications: "Уведомления",
      hotkeys: "Сочетания клавиш",
      providers: "Провайдеры",
      sandbox: "Песочница",
      versionControl: "Контроль версий",
      updates: "Обновления",
      help: "Справка",
    },
  },
  appearanceSettings: {
    importPickTitle: "Импорт темы",
    templatePickTitle: "Сохранить шаблон темы",
    imported: "Тема импортирована: {name}",
    importFailed: "Не удалось импортировать тему: {error}",
    installed: {
      one: "Установлена {count} тема из {pack} v{version}",
      few: "Установлено {count} темы из {pack} v{version}",
      many: "Установлено {count} тем из {pack} v{version}",
      other: "Установлено {count} темы из {pack} v{version}",
    },
    installFailed: "Не удалось установить тему: {error}",
    upToDate: "{pack} уже версии v{version}",
    updated: "{pack} обновлён до v{version}",
    updateFailed: "Не удалось обновить тему: {error}",
    removed: "Тема удалена: {name}",
    removeFailed: "Не удалось удалить тему: {error}",
    templateSaved: "Шаблон темы сохранён в {path}",
    templateFailed: "Не удалось создать шаблон темы: {error}",
    zoomTitle: "Масштаб",
    zoomOut: "Уменьшить масштаб",
    zoomIn: "Увеличить масштаб",
    resetZoom: "Сбросить масштаб",
    resetTextSize: "Сбросить размер текста терминала",
    textSizeTitle: "Размер текста терминала",
    textSmaller: "Уменьшить текст терминала",
    textLarger: "Увеличить текст терминала",
    replaceThemesTitle: "Заменить импортированные темы?",
    replaceThemesDescription:
      "В репозитории есть темы, которые уже установлены: {themes}. Всё равно установить? Предыдущие версии будут удалены без возможности восстановления.",
    replaceThemes: "Заменить темы",
    removeThemeTitle: "Удалить импортированную тему?",
    removeThemeDescription: "Удалить {name}? Файл импортированной темы будет удалён из lich.",
    removeTheme: "Удалить тему",
    replaceThemeTitle: "Заменить импортированную тему?",
    replaceThemeDescription:
      "Тема с идентификатором {id} уже существует. Всё равно импортировать? Предыдущая тема будет удалена без возможности восстановления.",
    replaceTheme: "Заменить тему",
  },
  themePicker: {
    title: "Тема",
    searchWords: "цвета тёмная светлая терминал палитра colors dark light",
    description: "Задаёт цвета интерфейса и терминала.",
    choose: "Выбрать тему",
    system: "Системная",
    followsOs: "следует за ОС",
    inUse: "используется",
    bundled: "встроенная",
    count: {
      one: "{count} тема",
      few: "{count} темы",
      many: "{count} тем",
      other: "{count} темы",
    },
    import: "Импорт",
    updateNamed: "Обновить {name}",
    updateHint: "Проверить репозиторий на наличие новой версии",
    removeNamed: "Удалить {name}",
  },
  brokenThemeList: {
    label: "Темы, которые не загрузились",
    heading: {
      one: "{count} тема не загрузилась",
      few: "{count} темы не загрузились",
      many: "{count} тем не загрузились",
      other: "{count} темы не загрузились",
    },
    removeNamed: "Удалить {name}",
    remove: "Удалить",
  },
  importThemeDialog: {
    title: "Импорт темы",
    description:
      "Установите из git-репозитория, чтобы тема управлялась версиями, или выберите отдельный файл темы.",
    repositoryUrl: "URL репозитория",
    install: "Установить",
    installing: "Установка…",
    repositoryHint:
      "Репозиторий с файлом {file} и одним или несколькими файлами тем. При обновлении lich сравнивает версию из его манифеста.",
    or: "или",
    chooseFile: "Выбрать файл…",
    fileHint: "Отдельный файл темы. Без версии и без обновлений.",
    downloadTemplate: "Скачать шаблон",
  },
  fontSetting: {
    title: "Шрифт терминала",
    searchWords: "гарнитура моноширинный typeface monospace",
    sample: "съешь же ещё этих мягких французских булок 0O1lI",
    search: "Поиск шрифтов",
    empty: "Нет подходящих шрифтов.",
  },
  footerSettings: {
    title: "Нижняя панель",
    searchWords: "строка состояния расположение status bar layout arrange footer",
    loading: "Загрузка сохранённых настроек…",
    saving: "Сохранение…",
    description:
      "Перетащите элемент на другую сторону или за пределы панели, чтобы скрыть его. Действует для всех проектов.",
    restoreDefault: "Сбросить по умолчанию",
    saveFailed: "Не удалось сохранить нижнюю панель: {error}",
    ceilingTitle: "Лимит расходов",
    ceilingSearchWords: "стоимость бюджет доллары cost budget usd",
    ceilingDescription:
      "Предупреждать, когда стоимость API в сессии приближается к этой сумме: жёлтым на 80%, красным на 95%. Пусто: без лимита.",
    ceilingPlaceholder: "Без лимита",
    ceilingLabel: "Лимит расходов на сессию в долларах",
  },
  footerLayoutEditor: {
    zone: { available: "Доступные", left: "Слева", right: "Справа" },
    allPlaced: "Все элементы на нижней панели",
    dragHere: "Перетащите элементы сюда",
    moveNamed: "Переместить {name}",
    previewLabel: "Предпросмотр нижней панели",
    previewHint: "Предпросмотр · примерные значения · прокрутите, чтобы увидеть все элементы",
    previewScrollLabel: "Прокручиваемый предпросмотр нижней панели",
    previewEmpty: "Не выбрано ни одного элемента нижней панели",
  },
  diffLayoutSetting: {
    title: "Вид изменений",
    searchWords: "рядом разделённый единый ревью side by side split unified review diff",
    description:
      "Как показываются изменения на вкладке «Ревью» и в pull requests. Режим «Рядом» переключается на «Единый», пока панель слишком узкая для двух колонок.",
    layout: { unified: "Единый", split: "Рядом" },
  },
  notificationsSettings: {
    needsInputTitle: "Уведомлять, когда сессии нужен ввод",
    needsInputDescription:
      "Показывать уведомление на рабочем столе, когда сессия ждёт вас (запрос разрешения, вопрос), а окно lich не активно. Пока вы в lich, ничего не меняется: об этом сообщают колокольчик на карточке и всплывающее уведомление.",
    needsInputLabel: "Показывать уведомление на рабочем столе, когда сессии нужен ввод",
    finishedTitle: "Уведомлять, когда сессия закончила работу",
    finishedDescription:
      "Показывать уведомление на рабочем столе, когда сессия завершила ход и ей больше нечего у вас спросить, а окно lich не активно. Не зависит от настройки выше: о долгом запуске, от которого вы отошли, стоит знать, даже если вы не хотите слышать о каждом запросе. Всплывающего уведомления при этом нет: внутри lich об этом уже говорят карточка и вкладка проекта.",
    finishedLabel: "Показывать уведомление на рабочем столе, когда сессия закончила работу",
  },
  hotkeysSettings: {
    alsoBound: "Также назначено: {actions}",
    pressKeys: "Нажмите клавиши…",
    unassignNamed: "Снять сочетание для «{name}»",
    unassignHint: "Оставить действие без сочетания",
    resetNamed: "Сбросить сочетание для «{name}»",
    resetHint: "Восстановить сочетание по умолчанию",
    terminalNote:
      "Это сочетание принадлежит lich и закреплено: оно перекрывает горячую клавишу браузера, которая отвечает на само сочетание, а не на то, на что его переназначили.",
    passthroughNote:
      "lich переписывает эти сочетания на пути к терминалу агента, поэтому они закреплены и не переназначаются.",
  },
  providersPane: {
    detecting: "Поиск провайдеров…",
    heading: "Провайдеры",
    addProvider: "Добавить провайдера",
    moreDetected: "Найдено ещё: {count}",
    defaultTitle: "Провайдер по умолчанию",
    defaultSearchWords: "агент новая сессия agent new session",
    defaultDescription:
      "Какого провайдера запускают неявные действия: сочетание новой сессии, кнопка «Новая сессия» в пустом проекте и новые worktree.",
    allProjects: "Все проекты",
    defaultForAll: "Провайдер по умолчанию для всех проектов",
    defaultForProject: "Провайдер по умолчанию для {name}",
    clear: "Сбросить",
    clearFallsBack: "После сброса будет {name}",
    following: "Как для всех проектов: {name}",
    openSettings: "Открыть настройки {name}",
    defaultHere: "по умолчанию здесь",
    customPath: "свой путь",
    skipWorktrees: "worktree",
    skipEverywhere: "без запросов",
    enableNamed: "Включить {name}",
  },
  providersSettings: {
    defaultDescription:
      "Провайдер для неявных действий с сессиями, если проект, в котором вы работаете, не выбрал своего в разделе Настройки › Провайдеры.",
    defaultLabel: "Глобальный провайдер по умолчанию",
    detected: "Найден в PATH",
  },
  providerDetail: {
    back: "Все провайдеры",
    enabled: "Включён",
    rightNowTitle: "Сейчас",
    rightNowSearchWords: "открытые сессии документация open sessions docs",
    openSessions: {
      one: "{count} сессия открыта",
      few: "{count} сессии открыто",
      many: "{count} сессий открыто",
      other: "{count} сессии открыто",
    },
    documentation: "Документация",
  },
  providerToggleRow: {
    notFound: "Не найден в PATH",
  },
  providerDocsLink: {
    howToInstall: "Как установить",
  },
  providerBinary: {
    title: "Исполняемый файл",
    searchWords: "путь команда бинарник path executable command binary",
    description: "Какой исполняемый файл запускает сессия.",
    descriptionProject: "Какой исполняемый файл запускает сессия в {name}.",
    pickTitle: "Выберите исполняемый файл {provider}",
    pickerFailed: "Не удалось открыть выбор файла",
    projectOnly: "Только {name}",
    allProjects: "Все проекты",
    layersHint: "Откуда берётся: используется первый включённый слой",
    done: "Готово",
    useLayer: "Использовать исполняемый файл {provider}, заданный для: {layer}",
    layerPath: "Исполняемый файл {provider} для: {layer}",
    pickLayer: "Выбрать исполняемый файл {provider} для: {layer}",
    browse: "Обзор",
    useDifferent: "Использовать другой исполняемый файл",
    sourceProjectNamed: "задан для {name}",
    sourceProject: "задан для этого проекта",
    sourceGlobal: "задан для всех проектов",
    sourcePath: "из $PATH",
  },
  providerBinSettings: {
    skipTitle: "Пропускать запросы разрешений",
    skipSearchWords: "yolo опасно подтверждения dangerous approvals",
    skipDescription:
      "Насколько самостоятельно работает {provider}: он правит файлы, выполняет команды и устанавливает программы без подтверждения, а lich запускает его с {flag}.",
    skipLabel: "Насколько самостоятельно работает {provider}",
    skip: {
      never: {
        label: "Никогда",
        consequence: "Каждая правка и команда ждёт вас в любой рабочей копии.",
      },
      worktrees: {
        label: "Только worktree",
        consequence: "Сессии в папке проекта по-прежнему спрашивают.",
      },
      everywhere: {
        label: "Везде",
        consequence: "Включая каталог, в котором вы работаете. Никто ничего не спросит.",
      },
    },
    confirmTitle: "Пропускать запросы разрешений: {level}",
    confirmDescription:
      "{provider} будет править файлы, выполнять команды и устанавливать программы без подтверждения; запуск с {flag}. {consequence}",
    confirm: "Пропускать запросы",
  },
  planUsageSetting: {
    title: "Использование тарифа",
    searchWords: "лимиты квота подписка quota subscription limit",
    description:
      "Какую часть каждого окна израсходовала ваша подписка. Данные берутся из вашего аккаунта и обновляются каждые 5 минут, пока lich открыт.",
    signedOut: "Вход не выполнен.",
    signedOutRun: "Вход не выполнен. Выполните {command}, чтобы увидеть использование тарифа.",
    readFailed:
      "Не удалось прочитать использование тарифа. Эндпоинт использования у провайдера не документирован и мог измениться.",
    used: "использовано",
    resets: "сброс",
  },
  closeSetting: {
    title: "При закрытии окна",
    searchWords: "выход фон закрыть окно трей продолжать quit exit background close window tray",
    description:
      "Что lich делает со своими сессиями при закрытии окна. Если ни одна сессия не запущена, закрытие окна завершает lich.",
    choice: {
      ask: {
        label: "Спрашивать",
        consequence: "Каждый раз lich спрашивает, продолжать ли работу в фоне.",
      },
      background: {
        label: "Продолжать работу",
        consequence:
          "Окно закрывается, а сессии продолжают работать; при новом запуске lich они вернутся.",
      },
      quit: {
        label: "Выйти",
        consequence: "Закрытие окна завершает lich и все его сессии.",
      },
    },
  },
  windowSetting: {
    title: "Окно",
    searchWords: "веб нативное экспериментальное окно web native experimental window",
    description:
      "Какое окно открывает lich. Нативное окно новое и пока экспериментальное; если оно не запустится, lich откроет веб-окно.",
    choice: { web: { label: "Веб" }, native: { label: "Нативное (экспериментально)" } },
    switch: {
      title: "Переключиться на нативное окно сейчас?",
      description: "Это окно закроется, откроется нативное. Ваши сессии продолжат работу.",
      later: "Позже",
      now: "Переключить",
      failed: "Не удалось открыть нативное окно",
    },
  },
  restoreSetting: {
    title: "Восстановленные сессии",
    searchWords: "возобновить разговор новая перезапуск спрашивать resume conversation restart ask",
    description:
      "Что делает карточка, в которой до перезапуска lich работал {provider}, когда вы открываете её в первый раз.",
    label: "Что делает восстановленная карточка {provider}",
    choice: {
      ask: {
        label: "Спрашивать",
        consequence: "Каждая восстановленная карточка спрашивает перед открытием.",
      },
      resume: {
        label: "Возобновить",
        consequence:
          "Восстановленные карточки продолжают разговор при открытии. Для пустого разговора откройте новую сессию.",
      },
      fresh: {
        label: "Начать заново",
        consequence:
          "Восстановленные карточки открываются пустыми. Прежний разговор не подхватывается.",
      },
    },
  },
  pasteUnfoldSetting: {
    title: "Раскрывать длинные вставки",
    searchWords: "вставка свернуть заглушка вставленный текст paste fold chip summary placeholder",
    descriptionNextPaste:
      "Вставка, которую {provider} свернул бы в заглушку, попадает полным текстом, и её можно отредактировать перед отправкой. Действует со следующей вставки в каждой сессии {provider}.",
    descriptionNextSession:
      "Вставка, которую {provider} свернул бы в заглушку, попадает полным текстом, и её можно отредактировать перед отправкой. Действует для сессий, открытых после изменения.",
    label: "Раскрывать длинные вставки в {provider}",
  },
  subagentCardsSetting: {
    title: "Субагенты как сессии lich",
    searchWords:
      "агент субагент карточка worktree делегировать фон agent subagent card delegate background",
    description:
      "Универсальный субагент открывает собственную карточку и worktree вместо того, чтобы работать скрыто внутри {provider}. Действует для сессий, открытых после изменения.",
    label: "Запускать субагентов {provider} как сессии lich",
  },
  ultracodeSetting: {
    title: "Ultracode",
    searchWords:
      "воркфлоу оркестрация мультиагентный усилие workflow orchestration multi-agent effort",
    description:
      "Каждая сессия {provider} запускается с включённым ultracode и оркестрирует мультиагентные воркфлоу при любом уровне усилия. Остаётся включённым при перезапуске и возобновлении сессии.",
    label: "Запускать сессии {provider} в ultracode",
  },
  sandboxSettings: {
    rung: { off: "Выкл.", ask: "Спрашивать", worktrees: "Worktree", everywhere: "Везде" },
    exposes: {
      off: "Каждая сессия работает прямо на этом компьютере.",
      ask: "Сессия, для которой вы снимете флажок, работает прямо на этом компьютере.",
      worktrees: "Сессии в папке проекта работают прямо на этом компьютере.",
    },
    summary:
      "Пустой домашний каталог, система только для чтения, запись лишь в рабочей копии. Сеть остаётся включённой.",
    confinedTitle: "Какие сессии работают в песочнице",
    confinedSearchWords: "изоляция isolate bwrap seatbelt",
    confinedHint:
      "Режим «Спрашивать» задаёт вопрос в меню «Новая сессия» и в диалоге «Новый worktree». Сессия, открытая там, где ответить некому (другой сессией или через инструмент MCP), работает в песочнице. Терминалы в песочницу никогда не помещаются: песочница нужна для агентов, работающих без присмотра.",
    carryTitle: "Что сессия в песочнице может получить",
    sshTitle: "SSH-агент",
    sshSearchWords: "push ключи keys",
    sshDescription:
      "git push работает внутри. Подписывает любой ключ из вашего агента для любого хоста, пока сессия работает.",
    sshLoaded: "Загружено: {keys}",
    sshNone: "Ничего не загружено.",
    ghTitle: "Токен GitHub",
    ghSearchWords: "gh учётные данные credentials",
    ghDescription:
      "gh работает внутри. Агент может прочитать токен из своего окружения и использовать его за пределами этого репозитория.",
    ghAccount: "От имени {login}.",
    ghActiveAccount: "От имени активного аккаунта gh.",
    rungLabel: "Какие сессии {provider} работают в песочнице",
    confirmTitle: "Меньше сессий {provider} в песочнице",
    confirmDescription: "{rung}: {exposes}",
    confirm: "Оставить без песочницы",
    canConfine: "Этот компьютер может изолировать сессии",
    cannotConfine: "Этот компьютер не может изолировать сессии",
    statusDetail: "({detail})",
  },
  vcsToolsSetting: {
    title: "Утилиты командной строки",
    searchWords: "git gh установка install",
    description: "lich управляет git и CLI GitHub. Всё на этом экране работает через них.",
    notOnPath: "Нет в $PATH",
    install: "Установить",
  },
  versionControlSettings: {
    account: "Аккаунт GitHub",
    accountFor: "Аккаунт GitHub для {name}",
    accountSearchWords: "вход gh переключить login switch",
    openProject: "Откройте проект, чтобы настроить его контроль версий.",
    accountDescription:
      "От имени какого аккаунта lich запускает gh для этого проекта: pull requests, проверки и рабочие копии PR. У gh один активный аккаунт на хост, поэтому репозиторий, видимый только другому аккаунту, считается ненайденным. Аккаунты на корпоративном хосте перечислены вместе с ним.",
    activeAccount: "активный аккаунт gh",
    selectAccount: "Выберите аккаунт",
    listFailed: "Не удалось получить список аккаунтов GitHub: {error}",
    noAccounts: "В gh не выполнен вход ни в один аккаунт. Выполните `gh auth login` в терминале.",
  },
  updatesSettings: {
    applicationTitle: "Приложение",
    applicationSearchWords: "версия обновление version upgrade",
    applicationDescription: "lich {version}: проверяет обновления при запуске и каждый час.",
    checkForUpdates: "Проверить обновления",
    available: "Доступен lich {version}: следуйте подсказке.",
    upToDate: "У вас последняя версия.",
    checkFailed: "Не удалось проверить: есть ли подключение к сети?",
    whatsNewTitle: "Что нового",
    whatsNewSearchWords: "список изменений заметки о релизе changelog release notes",
    patchNotesFor: "Заметки к v{version}.",
    noPatchNotes: "Для этой сборки заметок нет.",
    viewPatchNotes: "Показать заметки",
  },
  pluginSetting: {
    title: "Плагин lich",
    searchWords: "хуки установка hooks install",
    description:
      "Статус сессий, названия, обновление git и возобновление в тех CLI провайдеров, которые умеют его запускать.",
    installing: "Установка плагина lich для {name}…",
    installed: "Плагин установлен. Перезапустите сессии, чтобы применить.",
    installFailed: "Не удалось установить",
    updating: "Обновление плагина lich для {name}…",
    updated: "Плагин обновлён. Перезапустите сессии, чтобы применить.",
    updateFailed: "Не удалось обновить",
    checked: "Проверено.",
    checkFailed: "Не удалось проверить: есть ли подключение к сети?",
    cliMissing: "CLI не установлен",
    notInstalled: "Плагин не установлен",
    unsupported: "v{version}, не поддерживается этой версией lich",
    install: "Установить",
    updateTo: "Обновить до v{version}",
    updateAll: "Обновить все",
    updatingAll: "Обновление плагина lich во всех CLI…",
    installVersion: "Установить v{version}",
    codexHint: "Codex: выполните /hooks в сессии Codex, чтобы доверять хукам плагина.",
    crushHint:
      "Crush: сообщает идентификатор сессии и обновляет статус git. События конца хода у него нет, поэтому у его карточек нет статуса и они сохраняют собственное название.",
    ompHint:
      "oh-my-pi: сообщает свой статус, название и изменения в git. События запроса разрешения у него не замечено, поэтому сессия, ожидающая вашего разрешения, показывает индикатор загрузки, а не колокольчик.",
    antigravityHint:
      "Antigravity: сообщает свой статус, название и изменения в git. События запроса разрешения у него не замечено, поэтому сессия, ожидающая вашего разрешения, показывает индикатор загрузки, а не колокольчик.",
    cursorHint:
      "Cursor CLI: запускает плагин, установленный в Claude Code, поэтому сообщает то же, что и та версия, и обновляется вместе с ней. lich добавляет здесь только собственные инструменты. Как и у двух предыдущих, события запроса разрешения нет, поэтому сессия, ожидающая вашего разрешения, показывает индикатор загрузки, а не колокольчик.",
    checkForUpdates: "Проверить обновления",
  },
  helpSettings: {
    bugTitle: "Сообщить об ошибке",
    bugSearchWords: "баг issue github",
    bugDescription:
      "Открывает в браузере форму сообщения об ошибке с уже заполненными версией и платформой. lich не создаёт issue сам: вы пишете его самостоятельно. Выполните `lich rage` в терминале, чтобы собрать лог, версии и то, что lich обнаружил на этом компьютере, в один архив для вложения.",
    openBugReport: "Открыть форму ошибки",
    logTitle: "Файл журнала",
    logSearchWords: "отладка логи debug logs",
    logDescription:
      "В журнале есть абсолютные пути, названия проектов и веток, а также ваш логин gh, но никогда не токен сессии. Прочитайте его, прежде чем куда-либо прикладывать.",
    openingLogFolder: "Открытие папки журнала…",
    logFolderOpened: "Папка журнала открыта.",
    openLogFolderFailed: "Не удалось открыть папку журнала",
    openLogFolder: "Открыть папку журнала",
    noLogFile: "Журнал пишется только в stderr: файла журнала нет.",
    aboutTitle: "О программе",
    aboutSearchWords: "версия лицензия version license",
    aboutVersion: "lich {version}, {platform}.",
    repository: "Репозиторий",
  },
} satisfies Shape<typeof en>
