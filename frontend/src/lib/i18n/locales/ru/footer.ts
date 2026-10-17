import type { Shape } from "../../catalog"
import type { footer as en } from "../en/footer"

export const footer = {
  item: {
    attach: { label: "Прикрепить файл", example: "Прикрепить файл" },
    files: { label: "Обозреватель файлов", example: "Файлы" },
    changes: { label: "Изменения" },
    pr: { label: "Pull request" },
    checkout: { label: "Ветка" },
    path: { label: "Рабочая папка" },
    model: { label: "Модель", example: "Модель" },
    context: { label: "Окно контекста" },
    plan: { label: "Использование тарифа" },
    cost: { label: "Стоимость" },
    handsOn: { label: "Время за работой" },
    clock: { label: "Дата и время" },
  },
} satisfies Shape<typeof en>
