import type { Shape } from "../../catalog"
import type { diff as en } from "../en/diff"

export const diff = {
  binaryPreview: {
    before: "変更前",
    after: "変更後",
    added: "追加",
    deleted: "削除",
    binary: "バイナリファイル",
    binaryNoPreview: "バイナリファイル · この種類のファイルはプレビューできません",
    loading: "読み込み中…",
    missing: "このリビジョンには存在しません",
    pdfSide: "PDF のどちら側を表示するか",
  },
  commentBatch: {
    openSession: "これらのコメントを送るセッションを開いてください",
    removeComment: "コメントを削除",
    notePlaceholder: "変更全体についてのメモ…",
    noteLabel: "変更全体についてのメモ",
    count: {
      one: "{count} 件のコメント",
      other: "{count} 件のコメント",
    },
    clear: "クリア",
    send: "送信",
  },
  diffBulk: {
    collapseAll: "すべてのファイルを折りたたむ",
    expandAll: "すべてのファイルを展開",
    showWhitespace: "空白の変更を表示",
    hideWhitespace: "空白の変更を非表示",
  },
  discardDialog: {
    title: "変更を破棄",
    description:
      "{path} のコミットされていない変更をすべて元に戻しますか？この操作は取り消せません。",
    descriptionAdded:
      "{path} のコミットされていない変更をすべて元に戻しますか？ファイルはディスクから削除されます。この操作は取り消せません。",
    confirm: "変更を破棄",
  },
  injectMenu: {
    injectFile: "ファイルを挿入",
    injectLines: "行を挿入",
    injectLinesRange: "行 {lines} を挿入",
    sessionComment: "セッションへのコメント…",
    sessionCommentRange: "セッションへのコメント {lines}…",
    reviewComment: "プルリクエストにコメント…",
    reviewCommentRange: "プルリクエストにコメント {lines}…",
    revertSelected: "選択した行を元に戻す",
    revert: {
      one: "変更された {count} 行を元に戻す",
      other: "変更された {count} 行を元に戻す",
    },
  },
  fileDiff: {
    readFailed: "{path} を読み込めませんでした",
    whitespaceOnly: "空白のみ",
    onlyWhitespaceChanged: "空白のみ変更されました",
    addAsContext: "ファイルをコンテキストとして追加",
    discard: "変更を破棄",
    viewed: "確認済み",
  },
  reviewPanel: {
    lastTurnHint:
      "直前のターンの実行中にディスク上で変更された内容です。フォーマッター、エディター、あなた自身による編集も含みます。",
    discardFailed: "変更を破棄できませんでした: {error}",
    reverted: "{name} の +{added} -{deleted} を元に戻しました",
    undo: "元に戻す",
    undoFailed: "{name} での復元を取り消せませんでした",
    revertFailed: "{name} の行を元に戻せませんでした",
    said: "発言",
    sourceLabel: "表示する変更",
    workingTree: "作業ツリー",
    lastTurn: "直前のターン",
    ended: "{age}前に終了",
    turnReadFailed: "直前のターンを読み込めませんでした",
    notRepository: "git リポジトリではありません",
    loading: "読み込み中…",
    emptyTitle: "この期間には何も変更されていません。",
    emptyBody: "直前のターンの開始から終了までの間に、ディスク上のファイルは変更されませんでした。",
    unrecordedTitle: "直前のターンは記録されていません。",
    unrecordedBody: "ここでターンが終了すると表示されます。",
    lostTitle: "このターンの記録は失われました。",
    lostBody:
      "ここでターンが終了しましたが、lich はそのチェックアウトのスナップショットを取得できませんでした。次のターンは通常どおり記録されます。",
    noChanges: "コミットされていない変更はありません",
    squeezed: "左右に並べて表示するにはパネルの幅が足りません",
  },
  reviewSlots: {
    forSession: "セッション向け · {lines}",
    onPullRequest: "プルリクエスト上 · {lines}",
    addToBatch: "バッチに追加",
    addToReview: "レビューに追加",
    sessionPlaceholder:
      "ここで何を変えるべきですか？ほかのコメントと一緒にセッションへ送られます。",
    reviewPlaceholder: "コメントを残してください。レビューを送信したときに送られます。",
  },
} satisfies Shape<typeof en>
