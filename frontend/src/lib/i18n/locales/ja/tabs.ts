import type { Shape } from "../../catalog"
import type { tabs as en } from "../en/tabs"

export const tabs = {
  homeTab: {
    home: "ホーム",
  },
  notificationsButton: {
    title: "通知",
    titlePending: "通知、未確認 {count} 件",
    caughtUp: "すべて確認済みです",
    dismiss: "{name} を閉じる",
  },
  openProjectMenu: {
    open: "プロジェクトを開く",
    recent: "最近のプロジェクト",
    relocate: "場所を変更",
    openFolder: "フォルダーを開く…",
    moreClosed: {
      one: "ほかに閉じたプロジェクトが {count} 件あります。パレットで検索してください",
      other: "ほかに閉じたプロジェクトが {count} 件あります。パレットで検索してください",
    },
  },
  projectTab: {
    close: "{name} を閉じる",
  },
  projectTabs: {
    pullRequests: "プルリクエスト",
    settings: "設定",
    closeTitle: "まだ実行中のセッションがあります",
    closeBody: {
      one: "{name} を閉じると、ターンの途中のセッションが停止します。プロジェクトを開き直すと、それぞれ中断したところから再開できますが、実行中のターンは失われます。",
      other:
        "{name} を閉じると、ターンの途中の {count} 件のセッションが停止します。プロジェクトを開き直すと、それぞれ中断したところから再開できますが、実行中のターンは失われます。",
    },
    closeAnyway: "それでも閉じる",
  },
} satisfies Shape<typeof en>
