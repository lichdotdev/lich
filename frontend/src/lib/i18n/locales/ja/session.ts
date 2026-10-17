import type { Shape } from "../../catalog"
import type { session as en } from "../en/session"

export const session = {
  schedule: {
    choice15Minutes: "15 分",
    choice1Hour: "1 時間",
    choice4Hours: "4 時間",
    choiceTomorrow: "明日",
    countdown: "{amount}後",
    today: "今日 {clock}",
    weekdayAt: "{weekday} {clock}",
  },
  limitLine: {
    sessionWindow: "セッション上限",
    weeklyWindow: "週間上限",
    usageWindow: "使用量上限",
  },
  filter: {
    phase: {
      waiting: "入力待ち",
      running: "実行中",
      unread: "未読",
      idle: "アイドル",
    },
    phaseJoiner: "または",
    noMatch: "セッションはありません。アクティブなセッションは表示されたままです。",
    noMatchQuery:
      "「{query}」に一致するセッションはありません。アクティブなセッションは表示されたままです。",
    noMatchPhases: "{phases}のセッションはありません。アクティブなセッションは表示されたままです。",
    noMatchPhasesQuery:
      "「{query}」に一致する{phases}のセッションはありません。アクティブなセッションは表示されたままです。",
  },
  cost: {
    mixedModels: "この会話は途中でモデルが切り替わったため、lich は料金を算出できません。",
    unpricedModel:
      "このモデルの料金はまだありません。lich が取得するにはネットワーク接続が必要です。",
  },
  handsOn: {
    detailTurn:
      "このセッションで作業していた時間です（入力、報告、ターンの実行を含む）。15 分を超える間隔は離席時間として扱われます。",
    detailTool:
      "このセッションで作業していた時間です（入力、ツール呼び出しの報告を含む）。15 分を超える間隔は離席時間として扱われます。",
  },
  spawnGate: {
    checkoutGone:
      "このセッションのワークツリーが存在しないため、セッションは閉じられました。再開するにはワークツリーを作り直してください。",
    conversationGone: "以前の会話は利用できなくなりました。新しいセッションを開始します。",
  },
  fork: {
    unavailable: "{name} はフォークを保持しないため、再開のみ可能です。",
  },
  palette: {
    untitledConversation: "無題の会話",
    groupSessions: "セッション",
    groupProjects: "プロジェクト",
    groupMessages: "メッセージ",
    groupOpen: "開いている",
    groupClosed: "閉じた",
    groupClosedSessions: "閉じたセッション",
    groupOutsideLich: "lich の外部",
    indexing: {
      one: "{count} 件のセッションをインデックス中",
      other: "{count} 件のセッションをインデックス中",
    },
  },
} satisfies Shape<typeof en>
