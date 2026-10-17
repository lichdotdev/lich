import type { Shape } from "../../catalog"
import type { prompts as en } from "../en/prompts"

export const prompts = {
  pullRequest: {
    conflicts:
      "プルリクエスト #{number}（{branch}）に {base} とのマージコンフリクトがあります。解決してください。",
    checksUnnamed: {
      one: "プルリクエスト #{number}（{branch}）で CI が失敗しています。{count} 件のチェックが失敗しています。どのチェックかを特定して修正してください。",
      other:
        "プルリクエスト #{number}（{branch}）で CI が失敗しています。{count} 件のチェックが失敗しています。どのチェックかを特定して修正してください。",
    },
    checksListed:
      "プルリクエスト #{number}（{branch}）で CI が失敗しています。次のチェックを修正してください:\n\n{checks}",
    checksOmitted: {
      one: "\n\n…ほかに、一覧にない失敗したチェックが {count} 件あります。",
      other: "\n\n…ほかに、一覧にない失敗したチェックが {count} 件あります。",
    },
    create:
      "ブランチ {branch} にはまだプルリクエストがありません。`gh pr create` で作成してください。ブランチがリモートにない場合は先にプッシュし、タイトルと本文はコミットとベースブランチに対する差分から書き、リポジトリにプルリクエストテンプレートがあればそれに従ってください。",
  },
  delegate: {
    session: "「{target}」セッションに委任: ",
    newWorktree: "新しいワークツリーセッションに委任: ",
    newWorktreeWithCommand:
      "新しいワークツリーセッションに委任（`lich open --worktree <branch> --prompt <task>` を実行）: ",
  },
  reviewComments: {
    header: "レビューコメント:",
  },
  issue: {
    head: "GitHub issue #{number}: {title}\n{url}",
  },
} satisfies Shape<typeof en>
