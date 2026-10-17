<div align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="frontend/public/appicon.png" />
    <img src="frontend/public/appicon-light.png" alt="lich" width="88" height="88" />
  </picture>
  <h1>lich</h1>
  <p><a href="README.md">English</a> · <a href="README.pt-BR.md">Português (Brasil)</a> · <a href="README.es.md">Español</a> · <a href="README.zh-CN.md">简体中文</a> · <strong>日本語</strong> · <a href="README.ko.md">한국어</a> · <a href="README.ru.md">Русский</a></p>
  <p><strong>すでに使っているコーディングエージェントのための、ターミナルファーストな ADE。</strong></p>
  <p>
    プロジェクトを開き、Claude Code、Codex、opencode などのエージェントを
    本物のターミナルで実行しながら、git（ワークツリー、diff、プルリクエスト）も
    ウィンドウを離れずに視界に収められます。静的な Go バイナリ1つで、Electron は使いません。
    UI は Linux、Windows、macOS のいずれでも、lich 独自の組み込み Chromium で開きます。
  </p>
  <p><a href="https://omartelo.github.io/lich/"><strong>omartelo.github.io/lich</strong></a></p>
  <p>
    <a href="https://github.com/omartelo/lich/releases"><img alt="Release" src="https://img.shields.io/github/v/release/omartelo/lich?color=4285F4&label=release" /></a>
    <img alt="Go" src="https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white" />
    <img alt="Shell" src="https://img.shields.io/badge/shell-embedded%20Chromium%20(CEF)-4285F4?logo=googlechrome&logoColor=white" />
    <img alt="Platform" src="https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-333" />
    <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-AGPL--3.0-blue" /></a>
    <a href="https://github.com/sponsors/omartelo"><img alt="Sponsor" src="https://img.shields.io/github/sponsors/omartelo?color=ea4aaa&logo=githubsponsors&label=sponsors" /></a>
  </p>
  <img src="docs/media/session.png" alt="1つのウォールに並んだ4つの Claude Code セッション。それぞれ専用の git ワークツリーで動いており、サイドバーにはブランチと diff バッジ付きで一覧され、フッターにはモデル、プラン、ブランチが表示されています" width="1000" />
  <!-- sponsor-logos: company logos go here, between the screenshot and Why lich -->
</div>

## なぜ lich なのか

lich でできること：

- **すでに持っているエージェントをそのまま動かせます。** [Claude Code](https://www.anthropic.com/claude-code)、
  [Codex](https://github.com/openai/codex)、Antigravity、
  [opencode](https://github.com/sst/opencode)、oh-my-pi、
  [Crush](https://github.com/charmbracelet/crush)、
  [Cursor CLI](https://cursor.com/docs/cli)、
  [Kiro CLI](https://kiro.dev/docs/cli/) は、どれもここで同じように動きます。
  各バイナリを lich に一度指定しておけば、あとは既定のプロバイダーを選ぶことも、
  セッションごとに選ぶこともできます。セッションから lich が読み取れる内容は
  プロバイダーごとに異なり、その一覧が
  [プロバイダー対応状況](#プロバイダー対応状況)の表です。
- **本物のターミナルを維持できます。** PTY を使うシェルをプロジェクトごとに複数持て、
  GPU で描画されます。検索可能なスクロールバックは、ページを完全に再読み込みしても残ります。
  ターミナルにエントリポイント（`lazygit`、`k9s`、`pnpm dev` など）を設定すると、
  開くたびにそのツールが直接起動します。Linux、macOS、Windows のどれでも同じで、
  シェルの rc も PowerShell プロファイルも先に読み込まれないため、自分の環境で動くエントリポイントは
  共有した相手の環境でも動きます。フッターは `cd` に追従してブランチ名を表示します。
  Claude Code、Codex、Kiro CLI のセッションでは、使用中のモデルとコンテキストウィンドウも表示します。
  Claude Code、Codex、Antigravity、Cursor CLI、OpenCode Go のサブスクリプションでは、
  プランのローリングウィンドウがどれだけ残っているかも表示します。
  設定すれば、費用を記録するプロバイダーではセッションの消費額も表示します。
- **あるセッションに、別のセッションのために働かせられます。** タスクを別のカードに渡すと、
  そのカード自身のエージェントが答えを書き戻します。どちらの側で何が動いていても同じです。
  エージェントは、起動時に渡されるツール（Claude Code と Codex では MCP）か、
  プラグインが持ち込むツールを通じて、ほかのセッションに届きます。この機能一式は
  任意のシェルで `lich` コマンドとしても使え、`--json` にも対応しているので、
  エージェントを介さずにスクリプトからセッションを操作できます
  ([`docs/cli.md`](docs/cli.md))。
- **複数のセッションを同時に見渡せます。** 任意のセッションを、今いるセッションの隣のウォールに置くと、
  lich がペインの数とウィンドウの広さから配置を自動で決めます。8つのペインは
  ウルトラワイドでは横4列、ノートPCでは横2列に並びます。ほかのセッションに仕事を渡したセッションなら、
  ワンクリックでそのセッションを中心にウォールを組めます。ウォールには名前が付き、
  プロジェクトごとに好きなだけ持てて、それぞれがサイドバーに折りたたみ、名前変更、
  解体ができるブロックを持ちます。
- **セットアップなしでワークツリーに分岐できます。** 任意のベースブランチからワークツリーを作ると、
  lich が gitignore 対象の `.env*` ファイルをコピーし、ほかのチェックアウトにも
  マシン上のどのプロセスにも使われていない開発サーバーのポートを割り当て、
  エージェントを起動する前にプロジェクトごとのセットアップスクリプトを実行します。
  チェックアウトの未コミットの作業から始めれば、未追跡ファイルを含めてその変更が引き継がれます。
- **セッションを隔離して実行できます。** エージェントを OS のサンドボックス内で開けます。
  そのプロバイダー自身の状態だけを持つ空の新しいホーム、読み取り専用のそれ以外のマシン、
  開いた対象のチェックアウトへの書き込み権限という構成です。ssh キーも、クラウドの認証情報も、
  ディスク上のほかのリポジトリも、そこには単に存在しません。エージェントに権限確認を
  省略させることへの対抗策です。Linux では bubblewrap、macOS では `sandbox-exec` を使います。
  悪意あるコードに対する境界ではなく、止められないものは
  [`docs/ceilings.md`](docs/ceilings.md) に書かれています。
- **diff を読む場所でレビューできます。** CodeMirror のドックが、作業中の変更をライブのファイルツリーの隣に表示し、
  頼めば空白が動いただけの変更を折りたたみ、変更された画像や PDF は変更前後を並べて表示します。
  選択範囲を右クリックするとその行にコメントでき、まとめたコメントは1つのプロンプトとして
  セッションに貼り付けられます（送信はされません）。Code タブでは、チェックアウトを閲覧し、
  ファイルの本文を検索できます。
- **プルリクエストをここから出せます。** リポジトリのオープンなプルリクエストを一覧し、
  1つを専用のワークツリーにチェックアウトして、diff を読み、インラインでレビューし、
  ベースブランチが実際に受け付ける方法でマージできます。まだプルリクエストがないブランチでは、
  **Create with agent** がそのブランチのセッションにプルリクエストを開くよう依頼します。
  Enter を押すまで何も送信されません。

そのほか：JSON でインポートするか git リポジトリからインストールできる[テーマ](docs/themes.md)、
名前や会話での発言内容でジャンプできる `Ctrl`/`Cmd`+`K` パレット、
セッションがあなたの応答を待っているときのデスクトップ通知、
まったく起動しないときのための `lich rage` / `lich doctor` があります。

開発は活発に進んでいます。バグや機能要望は
[Issues](https://github.com/omartelo/lich/issues) へ、各バージョンの変更点は
[CHANGELOG.md](CHANGELOG.md) をご覧ください。

## プロバイダー対応状況

どのプロバイダーも、本物のターミナルで起動し、会話 ID で再開でき、同じサンドボックスで隔離され、
ほかのセッションに届く同じ手段が渡されます。違いは、lich がセッションから何を*読み取れる*かです。
lich は各 CLI が書き残したものを読むので、書き残す内容は CLI ごとに異なります。

| 得られるもの | Claude Code | Codex | Antigravity | opencode | oh-my-pi | Crush | Cursor CLI | Kiro CLI |
| --- | :-: | :-: | :-: | :-: | :-: | :-: | :-: | :-: |
| フッターにコンテキストウィンドウを表示 | あり | あり | なし | なし | なし | なし | なし | あり |
| フッターに費用を表示 | あり | あり | なし | あり | あり | あり | なし | クレジット |
| プランの残量を表示 | あり | あり | あり | OpenCode Go のみ | なし | なし | あり | なし |
| プランの使用上限で止まったターンが、上限のリセット後に再開する | あり | あり | なし | 独自のリトライ | 独自のリトライ | なし | なし | なし |
| ターン実行中はスピナー、終了時はリングを表示 | あり | あり | あり | あり | あり | なし | あり | あり |
| エージェントが開いたプルリクエストが、ターン終了時にカードへ表示される | あり | あり | あり | あり | あり | なし（ウィンドウがフォーカスを取り戻した時点で表示） | あり | あり |
| lich のライトテーマとダークテーマを切り替えると、新しい配色で再描画される | あり | なし | なし | あり | あり | なし | あり | なし |
| エージェントがあなたの応答待ちで止まるとベルが鳴る | あり | あり | なし | あり | なし | なし | なし | なし |
| Ctrl+Shift+Y で、応答待ちの次のセッションへジャンプ | あり | あり | なし | あり | なし | なし | なし | なし |
| 会話のコンパクト化中であることがカードに表示される | あり | なし | なし | なし | なし | なし | なし | なし |
| サイドバーを状態で絞り込み：Waiting、Running、Unread | あり | あり | Waiting なし | あり | Waiting なし | なし（常に Idle） | Waiting なし | Waiting なし |
| エージェントが別のセッション内で実行した場合（`claude -p`、`cursor-agent -p`）、そのセッションのカードには影響しない | あり | あり | あり | あり | あり | あり | あり | なし |
| ターン実行中はマシンをアイドルスリープさせない | あり | あり | あり | あり | あり | なし | あり | あり |
| Review タブの **Last turn** に、エージェントの要約を表示 | あり | あり | あり | あり | あり | なし | あり | あり |
| パレットから会話を検索 | あり | あり | あり | あり | あり | あり | なし | あり |
| lich の外で始めた会話を、パレットの History タブから再開 | あり | あり | あり | あり | あり | Linux のみ。macOS と Windows は未検証 | あり | あり |
| 長いペーストを、折りたたまれたプレースホルダーではなく編集可能なテキストとして貼り付けられる（Settings › Providers、既定ではオフ） | あり | なし | なし | あり | 未計測 | なし（`paste_N.txt` の添付になる） | なし | あり |
| 会話を新しいワークツリーにフォーク | あり | あり | なし | あり | なし | なし | なし | なし |
| 推論エフォートを指定してセッションを開く | あり | あり | あり | なし | あり | なし | モデル名で指定 | あり |
| ultracode をオンにしてセッションを開く | あり | なし | なし | なし | なし | なし | なし | なし |
| 複数のアカウントをプロファイルとして並べて使え、それぞれが独自の完全なログインを持ち、別のアカウントへ移したセッションが同じカードで続行される（ネイティブウィンドウ、実験的機能：Settings › Providers） | あり | あり。各プロファイルが独自のプラグインを持ち、Personal の設定はすべてで共有される | なし | あり。ターミナルカードからログインする。会話がログインを保持するデータベースにあるため、セッションを別のプロファイルへ移すことはできない | あり。ターミナルカードからログインし、設定はすべてのプロファイルで共有される | 未計測 | Linux と Windows（macOS ではログインがキーチェーンにある）。ターミナルカードからログインし、`mcp.json`、フック、スキルはすべてのプロファイルで共有される | Linux のみ（macOS ではログインがキーチェーンにあり、Windows ではそれを移す手段がない）。ターミナルカードからログインし、会話はすべてのプロファイルで共有される |
| メインのアカウントで信頼したフォルダーは、ほかのどのプロファイルでも「trust this folder?」と聞かれずに開く | あり | あり | プロファイルなし | 何も聞かない | 何も聞かない | 未計測 | あり | 何も聞かない |
| サンドボックス内のセッションは「trust this folder?」と聞かれずに開く | あり | あり | あり | 何も聞かない | 何も聞かない | あり。プロジェクトの初期化も聞かれない | あり | 何も聞かない |
| 信頼したフォルダーの worktree は「trust this folder?」と聞かれずに開く | あり | あり | あり | 何も聞かない | 何も聞かない | あり。プロジェクトの初期化も聞かれない | あり | 何も聞かない |
| 実行中のセッションを `lich control` またはエージェントから操作：プロンプト、停止、モデル、エフォート、スラッシュコマンド | あり | なし | なし | なし | なし | なし | なし | なし |
| 別のセッションからのメッセージが、ターミナルに入力されることなく届く | あり | なし | なし | なし | なし | なし | なし | なし |
| エージェントが起動したサブエージェントが、専用のカードを持つ lich セッションとして、依頼元セッションのチェックアウトで実行され、完全なレポートを返して、完了すると閉じる。そのサブエージェントのサブエージェントにもカードが付き、1段下に表示される | あり | なし | なし | なし | なし | なし | なし | なし |
| 実行中のセッションを止めずに、補足の質問をする（`lich ask`） | あり | なし | なし | なし | なし | なし | なし | なし |
| コンテキスト、費用、プラン使用量を、CLI 自身の計測どおりに表示 | あり | なし | なし | なし | なし | なし | なし | なし |
| ターミナルのステータスラインに、セッションが誰に回答する義務があるか、誰を待っているか、受け取れる回答があるかを表示 | あり | なし | なし | なし | なし | なし | なし | なし |
| サブエージェントやワークフローのステップから、非公開で委任する | あり | あり | あり | あり | あり | あり | あり | あり |
| 再開したセッションが、直前のモデルとエフォートを保持する | モデルのみ | あり | なし | モデルのみ | あり | なし | モデルのみ | モデルのみ。エフォートは未検証 |
| 待機を中断しても、結果はあとで受け取れる | あり | あり | あり | あり | なし | あり | あり | あり |
| 別のセッションに渡したタスクが、そのエージェントが返信せずにターンを終えた時点で、すぐに「unanswered」と報告される | あり | あり | あり | あり | あり | なし（期限まで待つ） | あり | あり |
| lich からエージェントへの独自メッセージ（渡されたタスク、結果の準備完了、通知）を、選択したプロンプト言語で送る | あり | あり | あり | あり | あり | あり | あり | あり |
| 起動時に、プロンプト言語で、作業を隠れたサブエージェントではなく lich セッションとして分散するよう伝えられる | あり | なし | なし | なし | あり | なし | なし | なし |

どの欠落も意図的なもので、CLI が報告しているものを lich が出し惜しみしているわけではありません。
それぞれの背後で何を計測したかは、[`docs/ceilings.md`](docs/ceilings.md) に書かれています。

## インストール

1行で、ディストリビューションを検出し、チェックサムを検証して、
パッケージマネージャー経由でネイティブパッケージと依存関係をインストールします：

```bash
curl -fsSL https://raw.githubusercontent.com/omartelo/lich/main/install.sh | sh
```

| プラットフォーム | 入手方法 | 実行時に必要なもの |
| --- | --- | --- |
| **Linux** | 上記の `install.sh`、または AUR の [`lich-bin`](https://aur.archlinux.org/packages/lich-bin) (`yay -S lich-bin`) | `zenity`。加えて、パッケージに同梱されるウィンドウのために glibc 2.34 以降（Debian 12、Ubuntu 22.04、RHEL 9 以上） |
| **macOS** | `brew install --cask lichdotdev/tap/lich` | 不要。ウィンドウはアプリに同梱されており、Apple Silicon と Intel のどちらでも使えます |
| **Windows** | [Releases](https://github.com/omartelo/lich/releases) のインストーラーまたはポータブル zip、または Scoop：`scoop install https://github.com/omartelo/lich/releases/latest/download/lich.json` | 不要。どちらにもウィンドウが同梱されています |

ディストリビューションごとの手動パッケージ、Linux の tarball、Windows のポータブル zip については
[INSTALL.md](INSTALL.md) をご覧ください。macOS と Windows のバイナリは署名されていないため、
公証と署名が提供されるまでは Gatekeeper と SmartScreen が警告を表示します。
Homebrew 経由のインストールでは Gatekeeper のプロンプトを回避できますが、
Releases ページからのダウンロードでは、隔離フラグを手動で解除する必要があります。
macOS ではこの cask が `Lich.app` をインストールするので、lich は `/Applications` と、
実行中の Dock に専用のアイコンを持ちます。古い formula からアップグレードする場合は、
先に `brew uninstall lich` が必要です。理由は [INSTALL.md](INSTALL.md) にあります。

## はじめに

1. lich を**インストール**して起動します。
2. **プロジェクトを開きます。** タブバーの `+` から最近閉じたものが一覧され、
   OS のフォルダーピッカーが開くので、git リポジトリを指定します。
3. **lich にエージェントを教えます。** 初回起動時に、マシン上で見つかったエージェントが一覧されます。
   Settings › Providers では、各バイナリのパスを設定し、既定のプロバイダーを選べます。
   プロジェクトは既定に従うことも、同じ画面で別のプロバイダーを選ぶこともできます。
4. **セッションを開始します。** *New Session* は、プロジェクト内でエージェントを実行するターミナルを起動します。
   各チェックアウトのヘッダーにも `+` メニューがあり、有効なプロバイダーや素のターミナルを
   そのチェックアウトで直接開けます。ヘッダー自体をクリックすると、そのセッションを折りたたみ、展開できます。
5. **ワークツリーに分岐します** *(任意)*。任意のベースブランチから作成すると、
   lich が初期化を行い、新しいセッションに入ります。

## 設定

- **プロバイダー**：Settings › Providers で、各プロバイダーのバイナリパスと、
  すべてのプロジェクトの既定を設定します。開いているプロジェクトの行では、その選択を上書きできます。
  **Clear** で上書きが解除されるので、以後の既定の変更が自動的に反映されます。
  パスが未設定の場合、セッションはログインシェルの `PATH` にあるそのプロバイダーのバイナリのうち最初のものを実行します。
  ネイティブウィンドウでは、Binary ブロックにそのファイルとバージョンが表示され、
  `PATH` にあるほかのバージョンも一覧されます（`lich doctor` もそれについて警告します）。
  特定のものを選ぶには、Binary にそのパスを設定します
  （設定名は `provider.<id>.bin`、例：`provider.codex.bin`）。
  プロバイダーのセクションの冒頭には、lich が読み取れる場合にプランの残量が表示されます。
  フッターに何を表示するかは、Settings › Appearance で項目をドラッグして設定します。
  費用の表示は、トークン単位で課金される場合にだけ意味を持つ数値なので、最初は非表示です。
- **ワークツリー**：プロジェクトのチェックアウトにある `.lich/setup-worktree.sh` が、
  新しいワークツリーのターミナルで、エージェントより先に実行されます。New worktree ダイアログには
  それが表示され、リポジトリに用意がない場合は検出された候補が提案されます。
  `.worktreeinclude` ファイルで、gitignore 対象のどのファイルをコピーするかを調整できます。
- **セッションフック**：Settings からインストールした
  [lich プラグイン](https://github.com/lichdotdev/lich-plugin)があると、
  セッションが自分のカードにタイトルを付け、ファイルを書き込んだ瞬間に git を更新します。
- **安定性**：lich のどの部分がスクリプトから安全に使えるか、どの部分がリリースごとに変わりうるかは、
  [docs/stability.md](docs/stability.md) に書かれています。

## プライバシーと更新

すべてあなたのマシン上で動きます。アカウントもサインインもテレメトリもありません。
バックエンドはトークン認証付きのループバックリスナーです。lich が自分からネットワークに接続するのは、
次の用途だけです：

- **更新**：起動時とその後1時間ごとの GitHub Releases に対するバージョン確認、
  および起動時の GitHub 上の lich プラグインのリリース一覧の取得。
- **プラン使用量**：各プロバイダー自身の使用量エンドポイント（Anthropic、OpenAI、Cursor、
  OpenCode Go）に対して、セッションの CLI がすでに使っているログインで、最大でも5分に1回。
  Antigravity のプランは `agy` 自身から読み取ります。
- **価格**：GitHub 上の LiteLLM の公開価格表。同梱の表にないモデルについて、一度だけ取得します。

Windows インストーラーのビルドはその場で更新されます。Homebrew、Scoop、AUR、
Linux のパッケージ経由のインストールは各パッケージマネージャーで更新され、
lich がそのコマンドを案内します。
Settings › Help には、バグ報告にログファイルを添付する前に、そこに何が含まれるか
（パス、プロジェクト名とブランチ名、gh のログイン名。セッショントークンは含まれません）が書かれており、
`lich rage` はその報告を1つのアーカイブにまとめます。どこにもアップロードしません。

## ソースからのビルド

純粋な Go のバックエンド（Go 1.27、`CGO_ENABLED=0`）が、組み込みの React 18 /
TypeScript / Vite フロントエンドを、トークン認証付きのループバックリスナー
（HTTP RPC と WebSocket）経由で配信します。ターミナルは WebGL アドオン付きの xterm.js で、
コードと diff の画面は CodeMirror 6 です。Chromium シェルについては決定記録があります：
[`docs/chromium-shell.md`](docs/chromium-shell.md)。前提となるのは **Go
1.27.0 以降**、**Node + pnpm**、**[Task](https://taskfile.dev)** で、
Go バイナリに C ツールチェーンは不要です。ウィンドウ（`shell/`、CEF 上の Rust）には
Rust ツールチェーンが加わり、Linux では Chromium の開発用ライブラリ、Windows では MSVC も必要です。
一覧は [CONTRIBUTING.md](CONTRIBUTING.md) にあります。

```bash
task dev      # hot-reload dev mode (Vite on :9245)
task build    # production binary -> bin/lich
task run      # build + run
task test     # Go + frontend suites
```

Linux のリリースパッケージをローカルで作成するには
（[nfpm](https://nfpm.goreleaser.com/) が必要です）：

```bash
task package   # .deb + .rpm + Arch .pkg.tar.zst in bin/
```

lich が動かす8つのエージェント CLI にもう1つ追加する作業は、2つのリポジトリにまたがる
十数個のファイルに及ぶ変更です。その地図が
[`docs/adding-a-provider.md`](docs/adding-a-provider.md) です。

## スポンサー

lich は1人で書かれ、保守されています。スポンサーになると、開発にかかる時間の費用をまかなえ、
プロジェクトの独立性を保てます。アプリに有料プランはなく、今後も設けません。

[**スポンサーになる**](https://github.com/sponsors/omartelo)

<!-- sponsor-names: monthly sponsors go here -->

### 支援者

<!-- backers: one-time supporters go here -->

まだいません。

## ライセンス

[AGPL-3.0-only](LICENSE) © 2026 omartelo

lich はフリーソフトウェアです。GNU Affero General Public License v3 の条件のもとで、
使用、研究、改変、再配布ができます。配布またはネットワーク経由で提供される派生物は、
同じライセンスで公開しなければなりません。v0.9.0 までのリリースは引き続き MIT ライセンスです。
