import type { Shape } from "../../catalog"
import type { palette as en } from "../en/palette"

export const palette = {
  commandPalette: {
    title: "コマンドパレット",
    placeholder: "セッション、プロジェクト、発言内容にジャンプ。> を入力するとアクション…",
    searchLabel: "セッションとプロジェクトを検索",
    resultsLabel: "結果",
    filterLabel: "結果を絞り込み",
    forgetFailed: "{label} を履歴から削除できませんでした: {error}",
    resumeFailed: "{label} を再開できませんでした: {error}",
    actionUnavailable: "{label} はここでは利用できません",
    showBeside: "横に表示",
    nothingClosed: "閉じたものはまだありません",
    nothingClosedHint:
      "セッションを閉じると、ワークツリーが削除されるまで、ブランチ、エージェント、会話ごとここで待機します。",
    noMatches: "{query} に一致するものはありません",
    noMatchesInTab: "{tab} に {query} に一致するものはありません",
    shownOfTotal: "{total} 件中 {shown} 件",
    tab: {
      All: "すべて",
      Sessions: "セッション",
      Projects: "プロジェクト",
      Messages: "メッセージ",
      History: "履歴",
    },
    checkoutGone: "チェックアウトなし",
    matches: {
      one: "{count} 件の一致",
      other: "{count} 件の一致",
    },
    sessions: {
      one: "{count} 件のセッション",
      other: "{count} 件のセッション",
    },
    relocate: "場所を変更",
    reopen: "再度開く",
  },
} satisfies Shape<typeof en>
