import type { Shape } from "../../catalog"
import type { common as en } from "../en/common"

export const common = {
  count: {
    file: {
      one: "{count} 個のファイル",
      other: "{count} 個のファイル",
    },
    commit: {
      one: "{count} 件のコミット",
      other: "{count} 件のコミット",
    },
  },
  time: {
    justNow: "たった今",
  },
  action: {
    cancel: "キャンセル",
    save: "保存",
    create: "作成",
  },
  checkAgainButton: {
    check: "再確認",
    checking: "確認中…",
  },
  errorBoundary: {
    stoppedRendering: "{label} の描画が停止しました",
    reload: "ウィンドウを再読み込み",
    retry: "再試行",
  },
  stepper: {
    default: "デフォルト",
  },
  toolMissing: {
    notInstalled: "{label} がインストールされていません",
    install: "{bin} をインストール",
  },
  pickerDialog: {
    navigate: "移動",
    filter: "絞り込み",
    close: "閉じる",
  },
  ui: {
    dialog: {
      close: "閉じる",
    },
  },
} satisfies Shape<typeof en>
