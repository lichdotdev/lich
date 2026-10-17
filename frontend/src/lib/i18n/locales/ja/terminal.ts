import type { Shape } from "../../catalog"
import type { terminal as en } from "../en/terminal"

export const terminal = {
  dropHint: {
    attachTo: "{label} に添付",
    confined: "チェックアウトの外にあるものはコピーとして渡されます",
    pasted: "パスがプロンプトに貼り付けられます",
  },
  exitBanner: {
    restart: "再起動",
    close: "閉じる",
  },
  searchBar: {
    placeholder: "検索",
    label: "ターミナルを検索",
    previous: "前の一致",
    next: "次の一致",
    close: "検索を閉じる",
  },
  drop: {
    notAttached: "添付されませんでした: {files}",
  },
  host: {
    stopShowing: "{name} の表示をやめる",
    paneName: "{name} ペイン",
    rememberFailed: "選択を保存できませんでした: {error}",
    showBeside: "このセッションの横に別のセッションを表示",
    noRoom: "これ以上ペインを追加する余白がありません",
  },
  showBesidePicker: {
    title: "横に表示",
    placeholder: "横に表示するセッションを検索…",
    search: "横に表示するセッションを検索",
    results: "横に表示できるセッション",
    pick: "表示",
    noMatch: "{query} に一致するセッションはありません",
    onWall: "{group} に表示中",
  },
  view: {
    restartFailed: "セッションを再起動できませんでした: {error}",
    startFailed: "セッションを開始できませんでした: {error}",
    pasteSettingFailed:
      "長い貼り付けを展開するかどうかの設定を読み込めなかったため、今回は一度だけ貼り付けました: {error}",
  },
  sessionExit: {
    ended: "セッションが終了しました",
    endedWithCode: "セッションがコード {code} で終了しました",
  },
  dropFiles: {
    folderConfined:
      "このサンドボックス化されたセッションのチェックアウト外にあるフォルダーは渡せません。ファイルをドロップしてください",
    folderMissing:
      "このセッションまたはホームの下にフォルダーが見つかりません。ファイルをドロップしてください",
  },
  copyToast: {
    copied: {
      one: "{count} 文字をクリップボードにコピーしました",
      other: "{count} 文字をクリップボードにコピーしました",
    },
  },
} satisfies Shape<typeof en>
