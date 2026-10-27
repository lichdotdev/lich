<div align="center">
  <img src="docs/media/appicon.svg" alt="lich" width="88" height="88" />
  <h1>lich</h1>
  <p><a href="README.md">English</a> · <a href="README.pt-BR.md">Português (Brasil)</a> · <a href="README.es.md">Español</a> · <strong>简体中文</strong> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a> · <a href="README.ru.md">Русский</a></p>
  <p><strong>终端优先的 ADE，接入你正在用的 AI 编程智能体。</strong></p>
  <p>
    打开项目，在真正的终端里运行 Claude Code、Codex、opencode 等智能体，git 也一直在
    眼前：worktree、diff 和 Pull Request，都不用离开这个窗口。不用 Electron，也不用浏览器：
    在 Linux、Windows 和 macOS 上，lich 都用 Gio 绘制自己的原生窗口。
  </p>
  <p><a href="https://lichdotdev.github.io/lich/index.zh-CN.html"><strong>lichdotdev.github.io/lich</strong></a></p>
  <p>
    <a href="https://github.com/lichdotdev/lich/releases"><img alt="Release" src="https://img.shields.io/github/v/release/lichdotdev/lich?color=4285F4&label=release" /></a>
    <img alt="Go" src="https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white" />
    <img alt="Window" src="https://img.shields.io/badge/window-native%20(Gio)-4285F4" />
    <img alt="Platform" src="https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-333" />
    <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-AGPL--3.0-blue" /></a>
    <a href="https://github.com/sponsors/omartelo"><img alt="Sponsor" src="https://img.shields.io/github/sponsors/omartelo?color=ea4aaa&logo=githubsponsors&label=sponsors" /></a>
  </p>
  <img src="docs/media/session.png" alt="同一分屏里并排的四个 Claude Code 会话，各自在独立的 git worktree 里；侧栏列出每个会话的分支和 diff 标记，底栏显示模型、套餐和分支" width="1000" />
  <!-- sponsor-logos: company logos go here, between the screenshot and Why lich -->
</div>

> 本文档译自英文 [README.md](README.md)。英文版是唯一的事实来源；两者不一致时，以英文版为准。

## 为什么选 lich

用 lich，你可以：

- **直接用你手上的智能体。** [Claude Code](https://www.anthropic.com/claude-code)、
  [Codex](https://github.com/openai/codex)、Antigravity、
  [opencode](https://github.com/sst/opencode)、oh-my-pi、
  [Crush](https://github.com/charmbracelet/crush)、
  [Cursor CLI](https://cursor.com/docs/cli) 和
  [Kiro CLI](https://kiro.dev/docs/cli/) 在 lich 里的运行方式完全相同。每个智能体的
  可执行文件只需指定一次，之后可以设一个默认，也可以每个会话单独选。lich 能从会话里
  *读出*哪些信息，各智能体不尽相同，详见[各智能体支持情况](#各智能体支持情况)一表。
- **用的是真正的终端。** shell 由 PTY 驱动，每个项目可以开多个，用 GPU 渲染；滚动
  缓冲区支持搜索，关掉窗口再打开也不会丢。给某个终端设一个入口命令（`lazygit`、`k9s`、
  `pnpm dev`），它每次打开都直接进入这个工具，Linux、macOS 和 Windows 上都一样：
  启动前不加载 shell rc 或 PowerShell profile，所以你这里能用的入口命令，分享给别人
  也照样能用。底栏会跟着 `cd` 切换目录并显示当前分支；Claude Code、Codex 和 Kiro CLI
  会话还会显示模型和上下文窗口用量；Claude Code、Codex、Antigravity、Cursor CLI 和
  OpenCode Go 订阅会显示套餐滚动窗口还剩多少额度；如果你需要，还能显示会话花了多少钱，
  前提是该智能体记录了这项数据。
- **让会话之间互派任务。** 把任务交给另一张卡片，由那张卡片上的智能体写回答案，两边跑的
  是什么智能体都可以：智能体通过启动时注入的工具访问其他会话，Claude Code 和 Codex 用
  MCP，其余的由插件提供。这整套能力在任何 shell 里也都能通过 `lich` 命令使用，同样支持
  `--json`，脚本不需要智能体参与就能操控会话（[`docs/cli.md`](docs/cli.md)）。
- **同时盯住好几个会话。** 把任意会话放到当前会话旁边组成分屏，lich 会根据面板数量和
  窗口大小自动排版：八个会话在超宽屏上排成两行、每行四个，在笔记本上每行两个。派出过任务
  的会话，点一下就能把相关会话围着它排好。每个分屏都有名字，一个项目可以保存任意多个，
  每个分屏在侧栏里占一块，可以折叠、重命名或拆散。
- **一键开 worktree，免去准备工作。** 从任意基础分支创建 worktree，lich 会复制被
  gitignore 忽略的 `.env*` 文件，并分配一个其他检出和本机任何进程都没在用的开发服务器端口。
  如果从当前检出里未提交的改动开始，这些改动
  也会一并带过去，包括未跟踪的文件。
- **在沙箱里跑会话。** 智能体可以在操作系统沙箱里启动：全新的空 home 目录，里面只有该
  智能体自己的状态，机器其余部分只读，只有打开它的那个检出目录可写。你的 ssh 密钥、云服务
  凭据和磁盘上的其他仓库，在沙箱里根本看不到；放手让智能体跳过权限确认时，这就是对应的
  防护。Linux 上用 bubblewrap，macOS 上用 `sandbox-exec`；它不能用来防御恶意代码，
  它拦不住哪些事，见 [`docs/ceilings.md`](docs/ceilings.md)。
- **在原处审查 diff。** diff 面板在实时文件树旁显示工作区改动，可以按需折叠只改了
  空白的部分，改过的图片或 PDF 会对比显示改动前后。选中内容后右键，就能针对这些行写评论；
  这批评论会合成一条 prompt 粘贴到会话里，但不会自动发送。面板的“代码”标签页可以浏览
  检出目录，并搜索文件内容。
- **在这里完成 Pull Request。** 列出仓库里开着的 PR，把其中一个检出到独立的 worktree，
  然后阅读 diff、直接在 lich 里审查并合并，可选的合并方式就是基础分支实际允许的那些。
  在还没有 PR 的分支上，点 **让智能体创建** 会请该分支的会话去开一个，你按回车之前什么
  都不会发出去。

此外还有：[主题](docs/themes.md)，可以导入 JSON，也可以从 git 仓库安装；`Ctrl`/`Cmd`+`K`
命令面板，可以按名称或聊过的内容跳到会话；会话等你回复时弹出桌面通知；以及 lich 完全
起不来时用的 `lich rage` / `lich doctor`。

项目正在积极开发中：bug 和功能需求请提交到
[Issues](https://github.com/lichdotdev/lich/issues)，各版本的改动见
[CHANGELOG.md](CHANGELOG.md)。

## 各智能体支持情况

每个智能体都在真实终端里启动，都按对话 id 恢复，都用同一套沙箱隔离，访问其他会话的方式也
相同。区别在于 lich 能从会话里*读出*什么：lich 读取的是各 CLI 自己写下的记录，而每家
记录的内容都不一样。

| 功能 | Claude Code | Codex | Antigravity | opencode | oh-my-pi | Crush | Cursor CLI | Kiro CLI |
| --- | :-: | :-: | :-: | :-: | :-: | :-: | :-: | :-: |
| 底栏显示上下文窗口 | 有 | 有 | 无 | 无 | 无 | 无 | 无 | 有 |
| 卡片提示显示正在运行的模型，若与打开时不同也显示打开时的模型 | 有 | 有 | 无 | 无 | 无 | 无 | 无 | 有 |
| 底栏显示花费 | 有 | 有 | 无 | 有 | 有 | 有 | 无 | 按点数计 |
| 套餐剩余额度 | 有 | 有 | 有 | 仅 OpenCode Go | 无 | 无 | 有 | 无 |
| 因套餐用量上限中断的一轮，在额度重置后自动继续 | 有 | 有 | 无 | 由其自身重试 | 由其自身重试 | 无 | 无 | 无 |
| 一轮运行中显示转圈，结束时显示圆环 | 有 | 有 | 有 | 有 | 有 | 无 | 有 | 有 |
| 卡片以对话命名 | 有 | 取自你的第一条提示 | 取自你的第一条提示 | 有 | 有，从第二轮起 | 无 | 有，第一轮结束时 | 取自你的第一条提示，第一轮结束时 |
| 智能体开的 PR 在一轮结束时显示在卡片上 | 有 | 有 | 有 | 有 | 有 | 无，等窗口重新获得焦点时显示 | 有 | 有 |
| lich 在浅色和深色主题间切换时，按新主题重绘 | 有 | 无 | 无 | 有 | 有 | 无 | 有 | 无 |
| 智能体等你处理时响铃 | 有 | 有 | 无 | 有 | 无 | 无 | 无 | 无 |
| Ctrl+Shift+Y 跳到下一个等你处理的会话 | 有 | 有 | 无 | 有 | 无 | 无 | 无 | 无 |
| 对话正在压缩时，卡片上会显示 | 有 | 无 | 无 | 无 | 无 | 无 | 无 | 无 |
| 侧栏按状态筛选：等待中、运行中、未读 | 有 | 有 | 无“等待中” | 有 | 无“等待中” | 无，始终为“空闲” | 无“等待中” | 无“等待中” |
| 由你的智能体在另一个会话里调用（`claude -p`、`cursor-agent -p`）时，不会算到那个会话的卡片上 | 有 | 有 | 有 | 有 | 有 | 有 | 有 | 无 |
| 一轮运行时阻止机器空闲休眠 | 有 | 有 | 有 | 有 | 有 | 无 | 有 | 有 |
| “审查”标签页的 **上一轮**，附智能体自己的总结 | 有 | 有 | 有 | 有 | 有 | 无 | 有 | 有 |
| 在命令面板里搜索对话内容 | 有 | 有 | 有 | 有 | 有 | 有 | 无 | 有 |
| 在命令面板的“历史”标签页恢复在 lich 之外开始的对话 | 有 | 有 | 有 | 有 | 有 | Linux；macOS 和 Windows 未测试 | 有 | 有 |
| 长文本粘贴可以作为可编辑的文本出现，不折叠成占位符（设置 › 提供商，默认关闭） | 有 | 无 | 无 | 有 | 未测量 | 无，会变成 `paste_N.txt` 附件 | 无 | 有 |
| 把对话分叉到新的 worktree，也可以分叉到原检出目录 | 有 | 有 | 无 | 有 | 无 | 无 | 无 | 无 |
| 在智能体自己的应用里恢复另一张卡片已打开的对话时会发出警告 | 有 | 在你下一次发提示词时 | 在你下一次发提示词时，未测试 | 无 | 有 | 在它下一次调用工具时 | 无 | 有 |
| 以指定的推理强度打开会话 | 有 | 有 | 有 | 无 | 有 | 无 | 写在模型名里 | 有 |
| 以 ultracode 模式打开会话 | 有 | 无 | 无 | 无 | 无 | 无 | 无 | 无 |
| 多个账号以配置档案并存，各自独立完整登录；会话换到另一个账号后仍在同一张卡片里继续（设置 › 提供商） | 有 | 有，每个配置档案有自己的插件，“个人”的设置由所有配置档案共用 | 无 | 有，在终端卡片里登录；会话不能换到另一个配置档案，因为对话和登录信息存在同一个数据库里 | 有，在终端卡片里登录，所有配置档案共用设置 | 未测量 | Linux 和 Windows（macOS 上登录信息在钥匙串里），在终端卡片里登录，`mcp.json`、hooks 和 skills 由所有配置档案共用 | 仅 Linux（macOS 把登录信息放在钥匙串里，Windows 上没有办法迁移），在终端卡片里登录，对话由所有配置档案共用 |
| 主账号里信任过的文件夹，在其他配置档案里打开时都不再问“trust this folder?” | 有 | 有 | 无配置档案 | 本来就不问 | 本来就不问 | 未测量 | 有 | 本来就不问 |
| 沙箱会话打开时不问“trust this folder?” | 有 | 有 | 有 | 本来就不问 | 本来就不问 | 有，也不再问是否初始化项目 | 有 | 本来就不问 |
| 信任过的文件夹，其 worktree 打开时不问“trust this folder?” | 有 | 有 | 有 | 本来就不问 | 本来就不问 | 有，也不再问是否初始化项目 | 有 | 本来就不问 |
| 用 `lich control` 或智能体操控运行中的会话：发提示词、停止、换模型、换推理强度、斜杠命令 | 有 | 无 | 无 | 无 | 无 | 无 | 无 | 无 |
| 其他会话发来的消息直接送达，不经终端输入 | 有 | 无 | 无 | 无 | 无 | 无 | 无 | 无 |
| 智能体启动的子智能体作为 lich 会话运行：有自己的卡片，在发起会话的检出目录里工作，交回完整报告后自动关闭；子智能体再启动的子智能体也有卡片，多一层为止 | 有 | 无 | 无 | 无 | 无 | 无 | 无 | 无 |
| 不打断运行中的会话，顺带问它一个问题（`lich ask`） | 有 | 无 | 无 | 无 | 无 | 无 | 无 | 无 |
| 上下文、花费和套餐用量直接采用 CLI 自己的统计 | 有 | 无 | 无 | 无 | 无 | 无 | 无 | 无 |
| 终端状态栏显示该会话欠谁一个回复、在等谁，以及有哪些回复可以收取 | 有 | 无 | 无 | 无 | 无 | 无 | 无 | 无 |
| 从子智能体或工作流步骤私下委派任务 | 有 | 有 | 有 | 有 | 有 | 有 | 有 | 有 |
| 恢复的会话保留原来的模型和推理强度 | 仅模型 | 有 | 无 | 模型 | 有 | 无 | 模型 | 模型；推理强度未测试 |
| 中断等待后，结果仍会留着等你收取 | 有 | 有 | 有 | 有 | 无 | 有 | 有 | 有 |
| 交给另一个会话的任务，对方一轮结束却没回复时，立即报告“未回复” | 有 | 有 | 有 | 有 | 有 | 无，要等到截止时间 | 有 | 有 |
| lich 发给智能体的消息（派出的任务、结果已就绪、通知）使用你选的提示词语言 | 有 | 有 | 有 | 有 | 有 | 有 | 有 | 有 |
| 启动时用提示词语言告诉智能体，要把工作分派成 lich 会话，而不是看不见的子智能体 | 有 | 无 | 无 | 无 | 有 | 无 | 无 | 无 |
| 会话卡片在会话旁的标签页中打开一个由智能体操作的浏览器 | 有 | 有 | 有 | 卡片会打开；智能体用 `lich browser`，而不是 `browser_*` 工具 | 有 | 有 | 有 | 有 |

每一处差异都是有意为之，没有一处是 CLI 报告了而 lich 不显示；每一处背后实测了什么，见
[`docs/ceilings.md`](docs/ceilings.md)。

## 安装

一行命令即可：脚本会识别你的发行版、校验 checksum，再通过包管理器安装原生软件包及其依赖：

```bash
curl -fsSL https://raw.githubusercontent.com/lichdotdev/lich/main/install.sh | sh
```

| 平台 | 安装方式 | 运行时依赖 |
| --- | --- | --- |
| **Linux** | 上面的 `install.sh`，或 AUR 的 [`lich-bin`](https://aur.archlinux.org/packages/lich-bin)（`yay -S lich-bin`） | `zenity`；软件包自带的窗口需要 glibc 2.34 或更高版本（Debian 12、Ubuntu 22.04、RHEL 9 及以上） |
| **macOS** | `brew install --cask lichdotdev/tap/lich` | 无，Apple Silicon 和 Intel 上窗口都随应用附带 |
| **Windows** | 从 [Releases](https://github.com/lichdotdev/lich/releases) 下载安装程序或便携版 zip，或用 Scoop：`scoop install https://github.com/lichdotdev/lich/releases/latest/download/lich.json` | 无，两种方式都自带窗口 |

各发行版的手动安装包、Linux tarball 和 Windows 便携版 zip 见 [INSTALL.md](INSTALL.md)。macOS 和
Windows 的二进制文件没有签名，在公证和签名上线之前，Gatekeeper 和 SmartScreen 会弹出警告。
用 Homebrew 安装不会触发 Gatekeeper 提示；从 Releases 页面下载的则需要手动清除隔离标记。
macOS 上 cask 会安装 `Lich.app`，所以 lich 在 `/Applications` 和运行时的 Dock 里都有自己的
图标。从旧的 formula 升级，需要先执行 `brew uninstall lich`，原因见 [INSTALL.md](INSTALL.md)。

## 快速上手

1. **安装**并启动 `lich`。
2. **打开项目**：标签栏里的 `+` 会列出最近关闭的项目，并打开系统的文件夹选择器；选一个
   git 仓库即可。
3. **为 lich 指定智能体**：首次启动时会列出在你机器上找到的智能体；之后可以在
   设置 › 提供商里设置每个智能体的可执行文件路径，并选定默认智能体。每个项目可以沿用
   默认，也可以在同一界面里改选别的智能体。
4. **开启会话**：*新建会话*会在项目里打开一个终端并运行你的智能体。每个检出目录的标题栏
   也有一个 `+` 菜单，可以直接在该检出目录里打开任意已启用的智能体或普通终端；点击标题栏
   本身可以折叠或展开其下的会话。
5. **新建 worktree** *（可选）*：从任意基础分支创建；lich 准备好文件后，会直接带你进入
   一个新会话。

## 配置

- **智能体**：在设置 › 提供商里设置每个智能体的可执行文件路径，以及所有项目的默认智能体。
  当前项目在那里的那一行可以覆盖这个选择；点**清除**会去掉覆盖，之后默认值的改动就会自动
  生效。没有设置路径时，会话会运行登录 shell 的 `PATH` 里找到的第一个该智能体可执行文件；
  “可执行文件”一栏会显示具体是哪个文件及其版本，并列出 `PATH` 里的其他版本，
  `lich doctor` 也会对此发出警告。要指定其中一个，在“可执行文件”里填写它的路径（对应设置项
  `provider.<id>.bin`，例如 `provider.codex.bin`）。
  只要 lich 能读到套餐信息，智能体那一节开头就会显示套餐剩余额度。底栏显示哪些内容，在
  设置 › 外观里拖动各项来调整；花费默认隐藏，因为只有按 token 计费时这个数字才有意义。
- **Worktree**：项目检出目录里的 `.worktreeinclude` 文件可以调整哪些被 gitignore 忽略的文件
  会被复制到新 worktree。
- **会话钩子**：在设置里安装 [lich 插件](https://github.com/lichdotdev/lich-plugin)后，
  会话会自动给自己的卡片起标题，并在写入文件时立即刷新 git 状态。
- **稳定性**：lich 哪些部分可以放心用脚本调用、哪些在任何版本里都可能变动，见
  [docs/stability.md](docs/stability.md)。

## 隐私与更新

一切都在你的机器上运行。不需要账号，不需要登录，没有遥测：后端只监听本地回环地址，并用
token 鉴权。lich 自己只会为下面这几件事联网，别的一概不连：

- **更新**：启动时和之后每小时向 GitHub Releases 检查一次新版本，启动时还会读取 GitHub 上
  lich 插件的发布列表。
- **套餐用量**：使用会话 CLI 已有的登录，查询各智能体自己的用量接口（Anthropic、OpenAI、
  Cursor、OpenCode Go），最多每五分钟一次。Antigravity 的套餐信息直接从 `agy` 读取。
- **价格**：GitHub 上 LiteLLM 的公开价格表，只在内置价格表里没有某个模型时下载一次。

Windows 安装程序装的版本会自动原地更新；通过 Homebrew、Scoop、AUR 或 Linux 软件包安装的，
则通过对应的包管理器更新，lich 会给出要执行的命令。
在你把日志附到 bug 报告之前，设置 › 帮助会说明日志文件里有哪些内容：路径、项目名和分支名、
你的 gh 登录名，但绝不包含会话 token；`lich rage` 会把这份报告打包成一个压缩文件，不会上传
任何内容。

## 从源码构建

lich 由两个二进制文件组成。后端 `lich` 是纯 Go（Go 1.27，`CGO_ENABLED=0`，一个静态
二进制文件），通过本地回环监听器（HTTP RPC + WebSocket，token 鉴权）提供会话。窗口
`lich-native` 是 `native/` 下独立的 Go 模块，用 cgo 构建：由 [Gio](https://gioui.org) 绘制，
终端用 `libghostty-vt`，代码和 diff 界面用 [godemirror](https://github.com/lichdotdev/godemirror)。
前置条件：**Go 1.27.0+** 和 **[Task](https://taskfile.dev)**；窗口另需 C 工具链、Gio 的系统软件包，
以及用 Zig 0.16 从 `native/GHOSTTY_COMMIT` 记录的 Ghostty 提交构建的 `libghostty-vt`。
构建方法见 [`native/slice.sh`](native/slice.sh)，窗口说明见
[`docs/native-window.md`](docs/native-window.md)。

```bash
task build          # 后端二进制文件 -> bin/lich
task build:native   # 窗口 -> bin/lich-native
task run            # 构建并运行
task test           # Go 测试套件
```

在本地打包 Linux 发布版本（需要 [nfpm](https://nfpm.goreleaser.com/)）：

```bash
task package   # bin/ 下生成 .deb + .rpm + Arch .pkg.tar.zst
```

在 lich 现有的八个智能体 CLI 之外再接入一个，是唯一一种要改两个仓库、十几个文件的改动：
具体要改哪些地方，见 [`docs/adding-a-provider.md`](docs/adding-a-provider.md)。

## 赞助

lich 由一个人开发和维护。你的赞助用来支付投入的时间，也让项目保持独立：这个应用没有
付费版，将来也不会有。

[**成为赞助者**](https://github.com/sponsors/omartelo)

<!-- sponsor-names: monthly sponsors go here -->

### 支持者

<!-- backers: one-time supporters go here -->

暂时还没有。

## 许可

[AGPL-3.0-only](LICENSE) © 2026 omartelo

lich 是自由软件：你可以按照 GNU Affero 通用公共许可证第 3 版的条款使用、研究、修改和
再分发它。任何分发出去或通过网络提供服务的衍生作品，都必须以同一许可证发布。v0.9.0
及更早的版本仍采用 MIT 许可证。
