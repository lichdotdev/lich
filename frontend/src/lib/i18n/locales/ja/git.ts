import type { Shape } from "../../catalog"
import type { git as en } from "../en/git"

export const git = {
  baseStatus: {
    behind: {
      one: "{base} から {count} コミット遅れ",
      other: "{base} から {count} コミット遅れ",
    },
    conflict: {
      one: "{count} 個のファイルにコンフリクト",
      other: "{count} 個のファイルにコンフリクト",
    },
  },
  fileSearch: {
    cutMatches: "{count}+ 件の一致",
    summary: "{files} 中 {matches}",
    matches: {
      one: "{count} 件の一致",
      other: "{count} 件の一致",
    },
    cutNote: "一致した最初の {count} 行を表示しています。検索を絞り込んでください。",
    tooLarge: {
      one: "1 MB を超える {count} 個のファイルは検索されていません。",
      other: "1 MB を超える {count} 個のファイルは検索されていません。",
    },
  },
  fileTree: {
    hidden: "非表示: {names}。",
    cut: "このフォルダーにはツリーに表示しきれないほど多くのファイルがあります。",
  },
  lastTurn: {
    saidPreviousTurn: "前のターンから",
    noTurnWindow: "{name} はターンの開始も終了も報告しないため、区切る期間がありません。",
  },
  carry: {
    reused:
      "{name} は既に存在していたため、そのままの状態でチェックアウトされました。コミットされていない作業は引き継がれていません。",
    failed: "コミットされていない作業を引き継げませんでした: {error}",
  },
  codemirror: {
    expand: {
      one: "変更のない {count} 行を展開",
      other: "変更のない {count} 行を展開",
    },
    showLines: "{from}〜{to} 行目を表示",
    revertTitle: "この変更を元に戻す",
    revert: "元に戻す",
  },
} satisfies Shape<typeof en>
