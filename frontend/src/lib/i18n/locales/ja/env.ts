import type { Shape } from "../../catalog"
import type { env as en } from "../en/env"

export const env = {
  providerSummary: {
    installed: "インストール済み",
    installedCustomPath: "インストール済み · カスタムパス",
    signedOut: "サインアウト中",
    windowPercent: "{length} ウィンドウの {percent}%",
    percentUsed: "{percent}% 使用",
  },
  quota: {
    tokenLogin: "トークンログイン",
    weekShort: "週",
  },
  paths: {
    cwdUnknown: "cwd 不明 · {host} 内",
  },
  sandbox: {
    windowsReason: "lich には Windows 用のサンドボックスバックエンドがありません",
    windowsAdvice:
      "インストールするものはありません。すべてのセッションはマシン上で直接実行されます。",
    macReason: "sandbox-exec を利用できません",
    macAdvice:
      "macOS には /usr/bin/sandbox-exec が同梱されているため、それが動作しないマシンは lich では修復できない形で壊れています。すべてのセッションはマシン上で直接実行されます。",
    linuxReason: "bubblewrap がインストールされていません",
    linuxAdvice:
      "bubblewrap をインストールして lich を開き直してください。それまでは、すべてのセッションがマシン上で直接実行されます。",
    confinedMeans:
      "エージェント自身の状態だけを置いた空のホーム、読み取り専用のマシン、書き込みはチェックアウト内のみ。ネットワークは有効のままです。",
  },
  vcsTools: {
    gitWithout: "これがないと、ブランチ、差分、ワークツリーは空のままです。",
    ghWithout: "これがないと、プルリクエスト、チェック、PR のチェックアウトは利用できません。",
  },
  binary: {
    parkedProject: "{name} の上書きはオフ",
    parkedProjectFallback: "プロジェクト",
    parkedGlobal: "グローバルの上書きはオフ",
    executable: "実行可能",
    noSuchFile: "ファイルが存在しません",
    notOnPath: "$PATH にありません",
    notExecutable: "実行可能ではありません",
    homeNotExpanded: "~ は展開されません",
    relativePath: "相対パス",
    detailHomeShortcut:
      "lich はバイナリを直接起動するため、~ は展開されずそのまま扱われます。フルパスを使ってください。",
    detailRelative:
      "相対パスは各セッションの作業ディレクトリを基準に解決されるため、セッションごとに異なるバイナリを指すことになります。フルパスを使ってください。",
    detailBroken:
      "これが修正されるまでセッションは起動しません。上書きはオフにしたりクリアしたりすることもでき、その場合は下の階層の設定が使われます。",
  },
  commitIdentity: {
    noneLead: "このチェックアウトには git の ID がありません。",
    noneNote: "user.email が設定されるまでコミットは拒否されます。",
    landAsNamed: "コミットは {name} として記録されます",
    landAs: "コミットの記録者:",
    setLocal: "（このリポジトリで設定され、グローバルの設定を上書きしています）",
    setGlobal: "（git の user.email であり、このアカウントではありません）",
  },
} satisfies Shape<typeof en>
