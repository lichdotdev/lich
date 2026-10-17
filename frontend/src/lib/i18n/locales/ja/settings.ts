import type { Shape } from "../../catalog"
import type { settings as en } from "../en/settings"

export const settings = {
  language: {
    uiTitle: "インターフェースの言語",
    uiDescription: "lich 自体の画面で使う言語です。",
    promptTitle: "プロンプトの言語",
    promptDescription:
      "lich がエージェントに渡すテキスト（ブリーフィング、中継されたメッセージ、通知）の言語です。今後開始するセッションではすべてにこの言語を使います。開いているセッションでは中継されたメッセージにのみ使われます。",
    uiSearchWords: "言語 ロケール 翻訳 英語 ポルトガル語 日本語",
    promptSearchWords:
      "言語 ロケール 翻訳 エージェント ブリーフィング 中継 英語 ポルトガル語 日本語",
    uiSaveFailed: "インターフェースの言語を保存できませんでした：{error}",
    promptSaveFailed: "プロンプトの言語を保存できませんでした：{error}",
  },
  settings: {
    search: "設定を検索",
    resultCount: { one: "{count} 件の設定", other: "{count} 件の設定" },
    noMatch: "{query} に一致する項目はありません。検索対象は各設定の名前で、値は対象外です。",
    section: {
      appearance: "外観",
      notifications: "通知",
      hotkeys: "ショートカット",
      providers: "プロバイダー",
      sandbox: "サンドボックス",
      versionControl: "バージョン管理",
      updates: "アップデート",
      help: "ヘルプ",
    },
  },
  appearanceSettings: {
    importPickTitle: "テーマをインポート",
    templatePickTitle: "テーマのテンプレートを保存",
    imported: "テーマをインポートしました：{name}",
    importFailed: "テーマのインポートに失敗しました：{error}",
    installed: {
      one: "{pack} v{version} から {count} 個のテーマをインストールしました",
      other: "{pack} v{version} から {count} 個のテーマをインストールしました",
    },
    installFailed: "テーマのインストールに失敗しました：{error}",
    upToDate: "{pack} はすでに v{version} です",
    updated: "{pack} を v{version} に更新しました",
    updateFailed: "テーマの更新に失敗しました：{error}",
    removed: "テーマを削除しました：{name}",
    removeFailed: "テーマの削除に失敗しました：{error}",
    templateSaved: "テーマのテンプレートを {path} に保存しました",
    templateFailed: "テーマのテンプレートの作成に失敗しました：{error}",
    zoomTitle: "ズーム",
    zoomOut: "縮小",
    zoomIn: "拡大",
    resetZoom: "ズームをリセット",
    resetTextSize: "ターミナルの文字サイズをリセット",
    textSizeTitle: "ターミナルの文字サイズ",
    textSmaller: "ターミナルの文字を小さく",
    textLarger: "ターミナルの文字を大きく",
    replaceThemesTitle: "インポート済みのテーマを置き換えますか？",
    replaceThemesDescription:
      "このリポジトリには、すでにインストール済みのテーマが含まれています：{themes}。それでもインストールしますか？以前のバージョンは完全に削除されます。",
    replaceThemes: "テーマを置き換える",
    removeThemeTitle: "インポートしたテーマを削除しますか？",
    removeThemeDescription:
      "{name} を削除しますか？インポートしたテーマファイルが lich から削除されます。",
    removeTheme: "テーマを削除",
    replaceThemeTitle: "インポート済みのテーマを置き換えますか？",
    replaceThemeDescription:
      "ID が {id} のテーマはすでに存在します。それでもインポートしますか？以前のテーマは完全に削除されます。",
    replaceTheme: "テーマを置き換える",
  },
  themePicker: {
    title: "テーマ",
    searchWords: "色 ダーク ライト ターミナル パレット",
    description: "インターフェースとターミナルの配色です。",
    choose: "テーマを選択",
    system: "システム",
    followsOs: "OS に従う",
    inUse: "使用中",
    bundled: "同梱",
    count: { one: "{count} 個のテーマ", other: "{count} 個のテーマ" },
    import: "インポート",
    updateNamed: "{name} を更新",
    updateHint: "リポジトリに新しいバージョンがあるか確認します",
    removeNamed: "{name} を削除",
  },
  brokenThemeList: {
    label: "読み込めないテーマ",
    heading: {
      one: "{count} 個のテーマを読み込めません",
      other: "{count} 個のテーマを読み込めません",
    },
    removeNamed: "{name} を削除",
    remove: "削除",
  },
  importThemeDialog: {
    title: "テーマをインポート",
    description:
      "テーマをバージョン管理するには git リポジトリからインストールします。単一のテーマファイルを選ぶこともできます。",
    repositoryUrl: "リポジトリの URL",
    install: "インストール",
    installing: "インストール中…",
    repositoryHint:
      "{file} と 1 つ以上のテーマファイルを含むリポジトリです。更新時に lich はマニフェストのバージョンを比較します。",
    or: "または",
    chooseFile: "ファイルを選択…",
    fileHint: "単一のテーマファイルです。バージョンも更新もありません。",
    downloadTemplate: "テンプレートをダウンロード",
  },
  fontSetting: {
    title: "ターミナルのフォント",
    searchWords: "書体 等幅 フォント",
    sample: "the quick brown fox 0O1lI",
    search: "フォントを検索",
    empty: "一致するフォントがありません。",
  },
  footerSettings: {
    title: "フッター",
    searchWords: "ステータスバー レイアウト 配置",
    loading: "保存済みの選択を読み込み中…",
    saving: "保存中…",
    description:
      "項目を左右の間でドラッグするか、外へドラッグして非表示にします。すべてのプロジェクトに適用されます。",
    restoreDefault: "デフォルトに戻す",
    saveFailed: "フッターを保存できませんでした：{error}",
    ceilingTitle: "支出の上限",
    ceilingSearchWords: "コスト 予算 ドル usd",
    ceilingDescription:
      "セッションの API コストがこの金額に近づくと警告します。80% で黄色、95% で赤になります。空欄なら上限なしです。",
    ceilingPlaceholder: "上限なし",
    ceilingLabel: "セッションの支出上限（ドル）",
  },
  footerLayoutEditor: {
    zone: { available: "利用可能", left: "左", right: "右" },
    allPlaced: "すべての項目がフッターにあります",
    dragHere: "ここに項目をドラッグ",
    moveNamed: "{name} を移動",
    previewLabel: "フッターのプレビュー",
    previewHint: "プレビュー · 値は例です · スクロールですべての項目を表示",
    previewScrollLabel: "スクロール可能なフッターのプレビュー",
    previewEmpty: "フッターの項目が選択されていません",
  },
  diffLayoutSetting: {
    title: "差分のレイアウト",
    searchWords: "左右 分割 統合 レビュー 差分",
    description:
      "レビュータブとプルリクエストで変更を表示する方法です。パネルが 2 列には狭すぎるあいだ、左右分割は統合表示になります。",
    layout: { unified: "統合", split: "左右分割" },
  },
  notificationsSettings: {
    needsInputTitle: "セッションが入力を必要としたら通知する",
    needsInputDescription:
      "セッションがあなたの対応待ち（権限の確認や質問など）で止まり、lich のウィンドウが使用中のウィンドウでないときに、デスクトップ通知を表示します。lich を使用中は何も変わりません。カードのベルとトーストでお知らせします。",
    needsInputLabel: "セッションが入力を必要としたらデスクトップ通知を表示する",
    finishedTitle: "セッションの作業が終わったら通知する",
    finishedDescription:
      "セッションが確認事項を残さずにターンを終え、lich のウィンドウが使用中のウィンドウでないときに、デスクトップ通知を表示します。上の設定とは独立しています。すべてのプロンプトの通知は不要でも、席を離れていた長い実行の終了は知っておく価値があります。トーストは表示されません。lich の中では、カードとそのプロジェクトタブがすでに知らせています。",
    finishedLabel: "セッションの作業が終わったらデスクトップ通知を表示する",
  },
  hotkeysSettings: {
    alsoBound: "{actions} にも割り当てられています",
    pressKeys: "キーを押してください…",
    unassignNamed: "{name} のショートカットを解除",
    unassignHint: "このアクションを未割り当てにします",
    resetNamed: "{name} のショートカットをリセット",
    resetHint: "デフォルトのショートカットに戻します",
    terminalNote:
      "lich 固有のもので、変更できません。これはブラウザーのアクセラレーターを上書きしており、そのアクセラレーターは割り当て直した先ではなくキーの組み合わせに反応するためです。",
    passthroughNote:
      "これらは lich がエージェントのターミナルに送る途中で書き換えるため、割り当て直せず固定です。",
  },
  providersPane: {
    detecting: "プロバイダーを検出中…",
    heading: "プロバイダー",
    addProvider: "プロバイダーを追加",
    moreDetected: "ほかに {count} 個を検出",
    defaultTitle: "デフォルトのプロバイダー",
    defaultSearchWords: "エージェント 新しいセッション",
    defaultDescription:
      "暗黙のセッション操作で起動するプロバイダーです。新しいセッションのショートカット、空のプロジェクトの「新しいセッション」ボタン、新しく作成したワークツリーが対象です。",
    allProjects: "すべてのプロジェクト",
    defaultForAll: "すべてのプロジェクトのデフォルトプロバイダー",
    defaultForProject: "{name} のデフォルトプロバイダー",
    clear: "クリア",
    clearFallsBack: "クリアすると {name} に戻ります",
    following: "すべてのプロジェクトの設定に従う：{name}",
    openSettings: "{name} の設定を開く",
    defaultHere: "ここでのデフォルト",
    customPath: "カスタムパス",
    skipWorktrees: "ワークツリー",
    skipEverywhere: "確認なし",
    enableNamed: "{name} を有効にする",
  },
  providersSettings: {
    defaultDescription:
      "暗黙のセッション操作で使うプロバイダーです。現在のプロジェクトが 設定 › プロバイダーで独自に選んでいる場合はそちらが使われます。",
    defaultLabel: "グローバルのデフォルトプロバイダー",
    detected: "PATH 上で検出",
  },
  providerDetail: {
    back: "すべてのプロバイダー",
    enabled: "有効",
    rightNowTitle: "現在",
    rightNowSearchWords: "開いているセッション ドキュメント",
    openSessions: {
      one: "{count} 個のセッションが開いています",
      other: "{count} 個のセッションが開いています",
    },
    documentation: "ドキュメント",
  },
  providerToggleRow: {
    notFound: "PATH 上に見つかりません",
  },
  providerDocsLink: {
    howToInstall: "インストール方法",
  },
  providerBinary: {
    title: "バイナリ",
    searchWords: "パス 実行ファイル コマンド",
    description: "セッションが起動する実行ファイルです。",
    descriptionProject: "{name} のセッションが起動する実行ファイルです。",
    pickTitle: "{provider} のバイナリを選択",
    pickerFailed: "ファイル選択ダイアログを開けませんでした",
    projectOnly: "{name} のみ",
    allProjects: "すべてのプロジェクト",
    layersHint: "取得元。オンになっている最初のレイヤーが優先されます",
    done: "完了",
    useLayer: "{layer} に設定された {provider} のバイナリを使う",
    layerPath: "{layer} の {provider} バイナリ",
    pickLayer: "{layer} の {provider} バイナリを選択",
    browse: "参照",
    useDifferent: "別のバイナリを使う",
    sourceProjectNamed: "{name} で設定",
    sourceProject: "このプロジェクトで設定",
    sourceGlobal: "すべてのプロジェクトで設定",
    sourcePath: "$PATH から",
  },
  providerBinSettings: {
    skipTitle: "権限の確認をスキップ",
    skipSearchWords: "yolo 危険 承認",
    skipDescription:
      "{provider} が確認なしで実行する範囲です。ファイルの編集、コマンドの実行、インストールを確認なしで行い、lich は {flag} を付けて起動します。",
    skipLabel: "{provider} が確認なしで実行する範囲",
    skip: {
      never: {
        label: "しない",
        consequence: "すべての編集とコマンドが、どのチェックアウトでもあなたの確認を待ちます。",
      },
      worktrees: {
        label: "ワークツリーのみ",
        consequence: "プロジェクトディレクトリのセッションは引き続き確認します。",
      },
      everywhere: {
        label: "どこでも",
        consequence: "作業中のツリーも含みます。何も確認されません。",
      },
    },
    confirmTitle: "権限の確認をスキップ：{level}",
    confirmDescription:
      "{provider} は {flag} を付けて起動され、ファイルの編集、コマンドの実行、インストールを確認なしで行います。{consequence}",
    confirm: "確認をスキップ",
  },
  planUsageSetting: {
    title: "プランの使用量",
    searchWords: "クォータ サブスクリプション 上限 使用量",
    description:
      "サブスクリプションが各期間をどれだけ使ったかを示します。アカウントから読み取り、lich が開いているあいだ 5 分ごとに更新します。",
    signedOut: "サインアウトしています。",
    signedOutRun:
      "サインアウトしています。プランの使用量を読み取るには {command} を実行してください。",
    readFailed:
      "プランの使用量を読み取れませんでした。プロバイダーの使用量エンドポイントは非公開のため、変更された可能性があります。",
    used: "使用済み",
    resets: "リセット",
  },
  closeSetting: {
    title: "ウィンドウを閉じたとき",
    searchWords: "終了 バックグラウンド 閉じる ウィンドウ トレイ 実行を続ける",
    description:
      "ウィンドウを閉じたときに lich がセッションをどう扱うかです。実行中のセッションがなければ、ウィンドウを閉じると lich は終了します。",
    choice: {
      ask: {
        label: "確認する",
        consequence: "lich が毎回、バックグラウンドで実行を続けるかどうかを確認します。",
      },
      background: {
        label: "実行を続ける",
        consequence:
          "ウィンドウは閉じますが、セッションは作業を続けます。lich を再び開くと元に戻ります。",
      },
      quit: {
        label: "終了",
        consequence: "ウィンドウを閉じると、lich とその中のすべてのセッションが終了します。",
      },
    },
  },
  windowSetting: {
    title: "ウィンドウ",
    searchWords: "ウェブ ネイティブ 実験的 ウィンドウ 開く",
    description:
      "lich が開くウィンドウです。ネイティブウィンドウは新しく、まだ実験的です。起動に失敗した場合は、代わりにウェブウィンドウを開きます。",
    choice: { web: { label: "ウェブ" }, native: { label: "ネイティブ（実験的）" } },
    switch: {
      title: "今すぐネイティブウィンドウに切り替えますか？",
      description:
        "このウィンドウが閉じてネイティブウィンドウが開きます。セッションは実行を続けます。",
      later: "後で",
      now: "今すぐ切り替える",
      failed: "ネイティブウィンドウを開けませんでした",
    },
  },
  restoreSetting: {
    title: "復元されたセッション",
    searchWords: "再開 会話 新規 再起動 確認",
    description:
      "lich の再起動前に {provider} を実行していたカードを、最初に開いたときの動作です。",
    label: "復元された {provider} カードの動作",
    choice: {
      ask: { label: "確認する", consequence: "復元されたカードは、開く前に毎回確認します。" },
      resume: {
        label: "再開",
        consequence:
          "復元されたカードは、開くと会話を続けます。空のセッションが必要なら、新しいセッションを開いてください。",
      },
      fresh: {
        label: "新規に開始",
        consequence: "復元されたカードは空の状態で開きます。以前の会話は引き継がれません。",
      },
    },
  },
  pasteUnfoldSetting: {
    title: "長い貼り付けを展開",
    searchWords: "貼り付け 折りたたみ チップ 要約 プレースホルダー テキスト",
    descriptionNextPaste:
      "{provider} がプレースホルダーに折りたたむような貼り付けを全文のまま入力するので、送信前に編集できます。次の貼り付けから、すべての {provider} セッションに適用されます。",
    descriptionNextSession:
      "{provider} がプレースホルダーに折りたたむような貼り付けを全文のまま入力するので、送信前に編集できます。変更後に開いたセッションに適用されます。",
    label: "{provider} で長い貼り付けを展開する",
  },
  subagentCardsSetting: {
    title: "サブエージェントを lich セッションとして実行",
    searchWords: "エージェント サブエージェント カード ワークツリー 委任 バックグラウンド",
    description:
      "汎用サブエージェントが {provider} の中で隠れて動く代わりに、専用のカードとワークツリーを開きます。変更後に開いたセッションに適用されます。",
    label: "{provider} のサブエージェントを lich セッションとして実行する",
  },
  ultracodeSetting: {
    title: "Ultracode",
    searchWords: "ワークフロー オーケストレーション マルチエージェント エフォート",
    description:
      "すべての {provider} セッションが ultracode をオンにして開始し、どのエフォートで動いていてもマルチエージェントのワークフローをオーケストレーションします。セッションを再起動または再開してもオンのままです。",
    label: "{provider} セッションを ultracode で開始する",
  },
  sandboxSettings: {
    rung: { off: "オフ", ask: "確認する", worktrees: "ワークツリー", everywhere: "どこでも" },
    exposes: {
      off: "すべてのセッションがマシン上で直接実行されます。",
      ask: "チェックを外したセッションはマシン上で直接実行されます。",
      worktrees: "プロジェクトディレクトリのセッションはマシン上で直接実行されます。",
    },
    summary:
      "空のホーム、読み取り専用のマシン、書き込みはチェックアウト内のみ。ネットワークはオンのままです。",
    confinedTitle: "隔離して実行するセッション",
    confinedSearchWords: "隔離 bwrap seatbelt",
    confinedHint:
      "「確認する」を選ぶと、「新しいセッション」メニューと「新しいワークツリー」ダイアログで確認します。誰も答えられない場所で開かれたセッション（別のセッションから、または MCP ツール経由）は隔離されます。ターミナルは隔離されません。サンドボックスは無人で作業するエージェントのためのものです。",
    carryTitle: "隔離されたセッションに持ち込めるもの",
    sshTitle: "SSH エージェント",
    sshSearchWords: "push 鍵 キー",
    sshDescription:
      "内部で git push が使えます。セッションが動いているあいだ、どのホストに対しても、エージェント内のすべての ID で署名します。",
    sshLoaded: "読み込み済み：{keys}",
    sshNone: "何も読み込まれていません。",
    ghTitle: "GitHub トークン",
    ghSearchWords: "gh 認証情報",
    ghDescription:
      "内部で gh が使えます。エージェントは環境変数からトークンを読み取り、このリポジトリの外で使うことができます。",
    ghAccount: "{login} として。",
    ghActiveAccount: "gh のアクティブなアカウントとして。",
    rungLabel: "隔離して実行する {provider} セッション",
    confirmTitle: "隔離する {provider} セッションを減らす",
    confirmDescription: "{rung}：{exposes}",
    confirm: "隔離しない",
    canConfine: "このマシンはセッションを隔離できます",
    cannotConfine: "このマシンはセッションを隔離できません",
    statusDetail: "：{detail}",
  },
  vcsToolsSetting: {
    title: "コマンドラインツール",
    searchWords: "git gh インストール",
    description:
      "lich は git と GitHub CLI を使って動作します。この画面の操作はすべてこれらを通して行われます。",
    notOnPath: "$PATH にありません",
    install: "インストール",
  },
  versionControlSettings: {
    // The block names itself after the open project: accountFor starts with
    // the text of account, which is what the settings index matches on.
    account: "GitHub アカウント",
    accountFor: "GitHub アカウント（{name}）",
    accountSearchWords: "ログイン gh 切り替え",
    openProject: "バージョン管理を設定するには、プロジェクトを開いてください。",
    accountDescription:
      "このプロジェクトで lich が gh を実行するアカウントです（プルリクエスト、チェック、PR のチェックアウト）。gh はホストごとにアクティブなアカウントを 1 つ保持するため、別のアカウントでしか見えないリポジトリは見つからないと表示されます。エンタープライズホストのアカウントはホスト名とともに表示されます。",
    activeAccount: "gh のアクティブなアカウント",
    selectAccount: "アカウントを選択",
    listFailed: "GitHub アカウントを一覧表示できませんでした：{error}",
    noAccounts:
      "gh はどのアカウントにもサインインしていません。ターミナルで `gh auth login` を実行してください。",
  },
  updatesSettings: {
    applicationTitle: "アプリケーション",
    applicationSearchWords: "バージョン アップグレード",
    applicationDescription: "lich {version}。起動時と 1 時間ごとにアップデートを確認します。",
    checkForUpdates: "アップデートを確認",
    available: "lich {version} が利用可能です。表示される案内に従ってください。",
    upToDate: "最新バージョンです。",
    checkFailed: "確認に失敗しました。オンラインですか？",
    whatsNewTitle: "新機能",
    whatsNewSearchWords: "変更履歴 リリースノート",
    patchNotesFor: "v{version} のパッチノートです。",
    noPatchNotes: "このビルドのパッチノートはありません。",
    viewPatchNotes: "パッチノートを表示",
  },
  pluginSetting: {
    title: "lich プラグイン",
    searchWords: "フック インストール",
    description:
      "セッションの状態、タイトル、git の更新、再開を、それを実行できるプロバイダー CLI の中で提供します。",
    installing: "{name} に lich プラグインをインストール中…",
    installed: "プラグインをインストールしました。適用するにはセッションを再起動してください。",
    installFailed: "インストールに失敗しました",
    updating: "{name} の lich プラグインを更新中…",
    updated: "プラグインを更新しました。適用するにはセッションを再起動してください。",
    updateFailed: "更新に失敗しました",
    checked: "確認しました。",
    checkFailed: "確認に失敗しました。オンラインですか？",
    cliMissing: "CLI がインストールされていません",
    notInstalled: "プラグインがインストールされていません",
    unsupported: "v{version}、この lich ではサポートされていません",
    install: "インストール",
    updateTo: "v{version} に更新",
    updateAll: "すべて更新",
    updatingAll: "すべての CLI で lich プラグインを更新中…",
    installVersion: "v{version} をインストール",
    codexHint:
      "Codex：プラグインのフックを信頼するには、Codex セッションで /hooks を実行してください。",
    crushHint:
      "Crush：セッション ID を報告し、git の状態を更新します。ターン終了イベントがないため、カードには状態が表示されず、名前も独自のままです。",
    ompHint:
      "oh-my-pi：状態、名前、git の変更を報告します。承認イベントが確認されていないため、権限の確認を待っているセッションにはベルではなくスピナーが表示されます。",
    antigravityHint:
      "Antigravity：状態、名前、git の変更を報告します。承認イベントが確認されていないため、権限の確認を待っているセッションにはベルではなくスピナーが表示されます。",
    cursorHint:
      "Cursor CLI：Claude Code にインストールされたプラグインを実行するため、そのバージョンが報告する内容を報告し、一緒に更新されます。ここでは lich は独自のツールだけを追加します。上の 2 つと同様に承認イベントを発生させないため、権限の確認を待っているセッションにはベルではなくスピナーが表示されます。",
    checkForUpdates: "アップデートを確認",
  },
  helpSettings: {
    bugTitle: "バグを報告",
    bugSearchWords: "issue github バグ 不具合",
    bugDescription:
      "バージョンとプラットフォームが入力された状態で、ブラウザーでバグ報告フォームを開きます。lich が issue を作成するのではなく、あなた自身が書きます。ターミナルで `lich rage` を実行すると、ログ、バージョン、このマシンで lich が見つけた情報を 1 つのアーカイブにまとめて添付できます。",
    openBugReport: "バグ報告を開く",
    logTitle: "ログファイル",
    logSearchWords: "デバッグ ログ",
    logDescription:
      "ログには絶対パス、プロジェクト名とブランチ名、gh のログイン名が含まれます。セッショントークンは含まれません。何かに添付する前に内容を確認してください。",
    openingLogFolder: "ログフォルダーを開いています…",
    logFolderOpened: "ログフォルダーを開きました。",
    openLogFolderFailed: "ログフォルダーを開けませんでした",
    openLogFolder: "ログフォルダーを開く",
    noLogFile: "ログは stderr のみに出力されています。ログファイルはありません。",
    aboutTitle: "このアプリについて",
    aboutSearchWords: "バージョン ライセンス",
    aboutVersion: "lich {version}（{platform}）。",
    repository: "リポジトリ",
  },
} satisfies Shape<typeof en>
