import type { Shape } from "../../catalog"
import type { prompts as en } from "../en/prompts"

export const prompts = {
  pullRequest: {
    conflicts: "У pull request #{number} ({branch}) есть конфликты слияния с {base}. Разрешите их.",
    checksUnnamed: {
      one: "CI падает в pull request #{number} ({branch}). {count} проверка не пройдена; выясните, какая именно, и исправьте.",
      few: "CI падает в pull request #{number} ({branch}). {count} проверки не пройдены; выясните, какие именно, и исправьте.",
      many: "CI падает в pull request #{number} ({branch}). {count} проверок не пройдено; выясните, какие именно, и исправьте.",
      other:
        "CI падает в pull request #{number} ({branch}). {count} проверки не пройдены; выясните, какие именно, и исправьте.",
    },
    checksListed:
      "CI падает в pull request #{number} ({branch}). Исправьте эти проверки:\n\n{checks}",
    checksOmitted: {
      one: "\n\n…и ещё {count} упавшая проверка не показана.",
      few: "\n\n…и ещё {count} упавшие проверки не показаны.",
      many: "\n\n…и ещё {count} упавших проверок не показано.",
      other: "\n\n…и ещё {count} упавшей проверки не показано.",
    },
    create:
      "У ветки {branch} пока нет pull request. Откройте его через `gh pr create`: сначала отправьте ветку (push), если её нет на удалённом репозитории, напишите заголовок и описание по коммитам и по diff относительно базовой ветки и следуйте шаблону pull request репозитория, если он есть.",
  },
  delegate: {
    session: "Делегируйте сессии «{target}»: ",
    newWorktree: "Делегируйте новой сессии в отдельном worktree: ",
    newWorktreeWithCommand:
      "Делегируйте новой сессии в отдельном worktree (выполните `lich open --worktree <branch> --prompt <task>`): ",
  },
  reviewComments: {
    header: "Комментарии ревью:",
  },
  issue: {
    head: "GitHub issue #{number}: {title}\n{url}",
  },
} satisfies Shape<typeof en>
