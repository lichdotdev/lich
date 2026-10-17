import type { Shape } from "../../catalog"
import type { dock as en } from "../en/dock"

export const dock = {
  rightDock: {
    fileBrowser: "Обозреватель файлов",
    reviewChanges: "Ревью изменений",
    tabCode: "Код",
    tabReview: "Ревью",
    exitFullScreen: "Выйти из полноэкранного режима",
    fullScreen: "Во весь экран",
    closePanel: "Закрыть панель",
    fileTree: "Дерево файлов",
    reviewPanel: "Панель ревью",
    resizePanel: "Изменить размер панели",
  },
  searchResults: {
    prompt: "Поиск по тексту всех файлов в этой рабочей копии",
    searching: "Поиск…",
    noMatch: "Нет файлов, содержащих «{query}»",
  },
  filesPanel: {
    searchPlaceholder: "Поиск в файлах",
    filterPlaceholder: "Фильтр по имени",
    searchLabel: "Поиск в файлах",
    filterLabel: "Фильтр файлов по имени",
    matchesLabel: "Что ищет поле",
    byName: "Имя",
    byText: "Текст",
    readFailed: "Не удалось прочитать эту папку",
    loading: "Загрузка…",
    noMatch: "Нет подходящих файлов",
    empty: "Здесь нет файлов",
    backToTree: "Назад к дереву файлов",
    readOnly: "только чтение",
  },
} satisfies Shape<typeof en>
