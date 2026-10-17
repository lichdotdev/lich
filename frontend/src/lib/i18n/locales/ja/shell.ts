import type { Shape } from "../../catalog"
import type { shell as en } from "../en/shell"

export const shell = {
  agentPluginGate: {
    updateAvailable: "lich プラグイン {version} が利用可能です",
    update: "更新",
    later: "後で",
    installVersion: "v{version} をインストール",
    updating: "lich プラグインを更新しています…",
    updated: "プラグインを更新しました。反映するにはセッションを再起動してください。",
    updateFailed: "更新に失敗しました",
    installed: "プラグインをインストールしました。反映するにはセッションを再起動してください。",
    codexTrust:
      "Codex: Codex のセッションで /hooks を実行し、プラグインのフックを信頼してください。",
    installFailed: "インストールに失敗しました: {error}",
    title: "エージェント連携を有効にする",
    description:
      "lich プラグインはセッションの状態とタイトルをアプリに伝え、git の状態を更新し、復元したセッションが会話を再開できるようにします。使っている CLI にインストールしてください。",
    needsHooks: "信頼するには /hooks が必要です",
    dontAskAgain: "今後表示しない",
    notNow: "今はしない",
    installing: "インストール中…",
    install: "インストール",
  },
  appUpdateGate: {
    available: "lich {version} が利用可能です",
    updateAndInstall: "更新してインストール",
    later: "後で",
    updated: "lich を {version} に更新しました",
    retry: "再試行",
    restart: "再起動",
    restartFailed: "再起動に失敗しました: {error}",
    install: "インストール",
    viewRelease: "リリースを見る",
  },
  confirmDialog: {
    cancel: "キャンセル",
  },
  emptySessions: {
    title: "開いているセッションはありません",
    description: "セッションを開いて、このプロジェクトで作業を始めましょう。",
    noAgent:
      "PATH にエージェントが見つからないため、ターミナルを開きます。設定 › プロバイダーで設定してください。",
    newTerminal: "新しいターミナル",
    newSession: "新しいセッション",
  },
  fileTree: {
    openInEditor: "エディターで開く",
    expandAll: "すべて展開",
    collapseAll: "すべて折りたたむ",
  },
  footerBar: {
    attachFailed: "ファイルを添付できませんでした",
    attachFile: "ファイルを添付",
    browseCode: "コードを閲覧",
    reviewChanges: "変更をレビュー",
    viewPullRequest: "プルリクエストを見る",
    pullRequest: "PR #{number}",
    leftFooter: "左フッター",
    rightFooter: "右フッター",
  },
  footerSession: {
    contextWindow: "コンテキストウィンドウ",
    contextTokens: "{tokens} / {window} トークン",
    costUnavailable: "コストは取得できません",
    sessionCost: "セッションのコスト",
    costOfBudget: "予算 {budget} のうち {cost}",
    costNote:
      "このセッションの会話全体の API コストです。サブスクリプションの料金とは異なる場合があります。",
    handsOnTime: "作業時間",
    handsOnFailed: "作業時間を取得できませんでした。",
  },
  gitMissingGate: {
    title: "git がインストールされていません",
    description:
      "lich はブランチ、差分、ワークツリーをすべて git で読み取ります。git がなくてもセッションは動きますが、バージョン管理の画面は空のままです。",
    notNow: "今はしない",
    installGit: "git をインストール",
  },
  home: {
    title: "開いているプロジェクトはありません",
    description: "フォルダーを開いてターミナルセッションを始めましょう。",
    openProject: "プロジェクトを開く",
  },
  notificationsOptIn: {
    title: "セッションが入力を待っているときに通知しますか？",
    description:
      "別の作業をしている間に、セッションが入力を求めました。lich はそのようなときにセッション名とプロジェクト名を添えてデスクトップ通知を出せるので、作業を始めたらウィンドウを離れても大丈夫です。表示中のセッションは通知しません。設定 › 通知でいつでも変更できます。",
    noThanks: "通知しない",
    notifyMe: "通知する",
  },
  paneSeams: {
    resizeColumns: "列のサイズを変更",
    resizeRows: "行のサイズを変更",
  },
  patchNotesDialog: {
    title: "lich {version} の新機能",
    viewChangelog: "変更履歴をすべて見る",
    gotIt: "了解",
  },
  planQuota: {
    planUsage: "プランの使用量",
    locked: "ロック中",
  },
  providerSetupDialog: {
    chooseTitle: "エージェントを選択",
    noneTitle: "コーディングエージェントが見つかりません",
    chooseDescription:
      "lich がこのマシンで見つけたエージェントです。使うものを有効にしてください。それ以外は新規セッションのメニューに表示されません。",
    noneDescription:
      "lich はこのマシンにインストール済みのエージェントを実行しますが、見つかりませんでした。いずれかをインストールしてから、もう一度確認してください。",
    changeLater: "設定 › プロバイダーでいつでも変更できます。",
    pointToBinary:
      "すでにインストール済みですか？設定 › プロバイダーで lich にそのバイナリの場所を指定してください。",
    continue: "続行",
  },
  quotaGauge: {
    aheadOfPace: "ペース超過",
    aheadTooltip: "この期間のペースより速く消費しています。",
    locked: "ロック中",
  },
  resumeSessionDialog: {
    title: "前回のセッションを再開しますか？",
    description:
      "{label} に会話が残っています ({id})。再開すると中断したところから会話を続けられます。新規で始めると空の会話になります。",
    alwaysFor: "{provider} では常にこうする",
    changeInSettings: "· 設定 › プロバイダーで変更できます",
    thisProvider: "このプロバイダー",
    startNew: "新規で始める",
    resume: "再開",
  },
  shortcutsOverlay: {
    title: "キーボードショートカット",
    rebind: "設定 › ショートカットで変更",
    close: "閉じる",
  },
  quitDialog: {
    title: "lich を終了しますか？",
    description:
      "すべてのセッションも一緒に終了します。ウィンドウを閉じるだけなら、セッションはバックグラウンドで動き続けます。",
    working: {
      one: "{count} 件のセッションがターンの途中で、それが失われます:",
      other: "{count} 件のセッションがターンの途中で、それが失われます:",
    },
    confirm: "lich を終了",
    failed: "lich を終了できませんでした",
  },
  tray: {
    show: "lich を表示",
    running: "実行中のセッション: {count}",
    quit: "lich を終了",
    openNative: "ネイティブウィンドウを開く",
    openWeb: "Web ウィンドウを開く",
  },
  windowFallback: {
    title: "Web ウィンドウを開きました",
    openLog: "ログを開く",
  },
  closeDialog: {
    title: "lich を実行したままにしますか？",
    running: {
      one: "{count} 件のセッションが実行中です。バックグラウンドで動き続け、lich をもう一度開くと画面に戻ります。",
      other:
        "{count} 件のセッションが実行中です。バックグラウンドで動き続け、lich をもう一度開くと画面に戻ります。",
    },
    dontAsk: "今後表示しない",
    changeInSettings: "· 設定 › 外観で変更できます",
    keep: "実行したままにする",
    quit: "lich を終了",
    failed: "ウィンドウを閉じられませんでした",
  },
  uncleanExitGate: {
    title: "前回の lich は予期せず終了しました",
    description: "セッションは復元されましたが、実行中だった処理はそのときに停止しています。",
    openLogFolder: "ログフォルダーを開く",
    opening: "ログフォルダーを開いています…",
    opened: "ログフォルダーを開きました。",
    openFailed: "ログフォルダーを開けませんでした",
    dismiss: "閉じる",
  },
  updateProgressToast: {
    downloading: "lich {version} をダウンロード中",
    installing: "lich {version} をインストール中…",
    reopens: "lich は自動的に終了して再び開きます。",
  },
  app: {
    stage: "ステージ",
    reloadStage: "ステージを再読み込み",
    screen: "この画面",
    panel: "パネル",
  },
  projects: {
    relocated: "{name} は {path} を開くようになりました",
    relocateFailed: "場所の変更に失敗しました: {error}",
    noLongerAvailable: "{label} はもう再開できません",
    bringBackFailed: "{label} を元に戻せませんでした",
    closed: "{label} を閉じました",
    undo: "元に戻す",
    entrypointSet: "エントリーポイントを設定しました",
    entrypointSetDescription: "次にこのターミナルが起動したときに実行されます。",
    entrypointCleared: "エントリーポイントを解除しました",
    entrypointClearedDescription: "このターミナルは通常のシェルで起動するようになりました。",
    entrypointFailed: "エントリーポイントを保存できませんでした: {error}",
    scheduled: "{when} に予約しました",
    scheduledNow: "今すぐ実行するよう予約しました",
    scheduleCleared: "予約を解除しました",
    scheduleFailed: "予約できませんでした: {error}",
  },
  projectEvents: {
    unlabeledSession: "セッション",
    scheduleLost: "{label} とともに予約したプロンプトが失われました",
    scheduleDue: "{when} 予定: {prompt}",
    relayAnswered: "{target} は自分のセッションで回答しました",
    relayDetail: "lich 経由で返信せずに終了しました。開いて回答を確認してください。",
    open: "開く",
    needsInput: "{label} が入力を待っています",
    finished: "{label} の作業が完了しました",
  },
} satisfies Shape<typeof en>
