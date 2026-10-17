import type { Shape } from "../../catalog"
import type { footer as en } from "../en/footer"

export const footer = {
  item: {
    attach: {
      label: "ファイルを添付",
      example: "ファイルを添付",
    },
    files: {
      label: "ファイルエクスプローラー",
      example: "ファイル",
    },
    changes: {
      label: "変更",
    },
    pr: {
      label: "プルリクエスト",
    },
    checkout: {
      label: "ブランチ",
    },
    path: {
      label: "作業ディレクトリ",
    },
    model: {
      label: "モデル",
      example: "モデル",
    },
    context: {
      label: "コンテキストウィンドウ",
    },
    plan: {
      label: "プラン使用量",
    },
    cost: {
      label: "コスト",
    },
    handsOn: {
      label: "作業時間",
    },
    clock: {
      label: "日付と時刻",
    },
  },
} satisfies Shape<typeof en>
