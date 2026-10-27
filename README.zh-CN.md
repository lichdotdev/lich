<div align="center">
  <img src="docs/media/appicon.svg" alt="lich" width="88" height="88" />
  <h1>lich</h1>
  <p><a href="README.md">English</a> · <a href="README.pt-BR.md">Português (Brasil)</a> · <a href="README.es.md">Español</a> · <strong>简体中文</strong> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a> · <a href="README.ru.md">Русский</a></p>
  <p>
    <a href="https://github.com/lichdotdev/lich/releases"><img alt="Release" src="https://img.shields.io/github/v/release/lichdotdev/lich?color=4285F4&label=release" /></a>
    <img alt="Platform" src="https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-333" />
    <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-AGPL--3.0-blue" /></a>
    <a href="https://github.com/sponsors/omartelo"><img alt="Sponsor" src="https://img.shields.io/github/sponsors/omartelo?color=ea4aaa&logo=githubsponsors&label=sponsors" /></a>
  </p>
  <p>
    <strong>让编程智能体并排运行，各自在自己的分支上。</strong><br/>
    Claude Code、Codex、opencode，或者你已经在用的智能体，各自在独立的 worktree 里，全都在同一个窗口中。
  </p>
  <h3><a href="#安装">下载 lich</a> · <a href="https://lichdotdev.github.io/lich/index.zh-CN.html">lichdotdev.github.io/lich</a></h3>
  <img src="docs/media/session.png" alt="同一分屏里并排的四个 Claude Code 会话，各自在独立的 git worktree 里；侧栏列出每个会话的分支和 diff 标记，底部是项目正在运行的服务和当前 profile；底栏显示分支、模型和套餐" width="1000" />
  <!-- sponsor-logos: company logos go here, between the screenshot and Features -->
</div>

## 功能

<table>
<tr>
<td width="50%" valign="middle">

### 智能体并排运行

把会话放到同一个分屏里，lich 替你排好各个面板。哪个在忙、哪个在等你、还剩多少上下文，一眼就能看出来。

</td>
<td width="50%"><img src="docs/media/wall-mixed.png" alt="同一分屏里的 Claude Code 和 Codex 会话，各自在自己的分支上" width="100%" /></td>
</tr>
<tr>
<td width="50%" valign="middle">

### 每个任务一个 worktree，无需准备

从任意基础分支开出一个会话，lich 会复制你的 `.env` 文件，并分配一个空闲的开发服务器端口。尚未提交的改动也可以一起带过去。

</td>
<td width="50%"><img src="docs/media/worktrees.png" alt="侧栏里的会话，每个都有自己的分支和 diff 标记" width="100%" /></td>
</tr>
<tr>
<td width="50%" valign="middle">

### 审查 diff，逐行评论

在会话旁边看智能体改了什么，给需要返工的行写评论，再把所有评论合成一条 prompt 发回去。

</td>
<td width="50%"><img src="docs/media/review-comments.png" alt="一份 diff，其中两条行评论排队等待发送到会话" width="100%" /></td>
</tr>
<tr>
<td width="50%" valign="middle">

### 不开浏览器也能处理 Pull Request

创建 Pull Request，请该分支的智能体来写，直接在 diff 里审查，再合并，全程不离开这个窗口。

</td>
<td width="50%"><img src="docs/media/pull-request-review.png" alt="在 lich 里打开的 Pull Request，带有文件树、diff 和一条待提交的审查评论" width="100%" /></td>
</tr>
<tr>
<td width="50%" valign="middle">

### 做成你喜欢的样子

可以导入 JSON 主题，也可以从 git 仓库安装。

[文档 →](docs/themes.md)

</td>
<td width="50%"><img src="docs/media/themes.png" alt="外观设置，里面有主题选择、字体和底栏布局" width="100%" /></td>
</tr>
</table>

此外还有：

- **会话之间互派任务。** 一个智能体把任务交给另一个会话，再拿回答案。在 shell 里用 `lich` 命令也能做到同样的事（[文档](docs/cli.md)）。
- **沙箱里的智能体。** 让智能体在沙箱里跳过权限确认，沙箱看不到你的密钥、凭据和其他仓库（[限制](docs/ceilings.md)）。
- **开发服务器就在侧栏。** 保存项目需要一直开着的东西，一键启动或停止。
- **什么都找得到。** `Ctrl`/`Cmd`+`K` 可以按名称或聊过的内容跳到会话。
- **需要你时会知会你。** 会话在等你回复时，弹出桌面通知。

## 安装

```bash
curl -fsSL https://raw.githubusercontent.com/lichdotdev/lich/main/install.sh | sh
```

一行命令即可：脚本会识别你的发行版、校验下载内容，再通过包管理器安装 lich。

| 平台 | 获取方式 |
| --- | --- |
| **Linux** | 上面的 `install.sh`，或 AUR 的 [`lich-bin`](https://aur.archlinux.org/packages/lich-bin)（`yay -S lich-bin`）。需要 `zenity` 和 glibc 2.34 或更高版本（Debian 12、Ubuntu 22.04、RHEL 9 及以上）。 |
| **macOS** | `brew install --cask lichdotdev/tap/lich` |
| **Windows** | 从 [Releases](https://github.com/lichdotdev/lich/releases) 下载安装程序或便携版 zip，或用 `scoop install https://github.com/lichdotdev/lich/releases/latest/download/lich.json` |

macOS 和 Windows 的构建还没有签名，所以从 Releases 下载时 Gatekeeper 和 SmartScreen
会弹出警告；用 Homebrew 安装则不会触发 Gatekeeper 提示。各发行版的软件包、Linux
tarball、清除隔离标记，以及从旧的 Homebrew formula 升级，见 [INSTALL.md](INSTALL.md)。

## 快速上手

1. **打开项目。** 标签栏里的 `+` 会列出最近关闭的项目，并打开文件夹选择器。选一个 git
   仓库即可。
2. **选择智能体。** 首次启动会列出在你机器上找到的智能体。设置 › 提供商里可以设置默认智能体，
   以及每个可执行文件的路径。
3. **开启会话。** *新建会话*会在项目里打开你的智能体。每个检出目录标题栏上的 `+`，
   可以在该检出目录里打开任意智能体或普通终端。
4. **给它一个 worktree** *（可选）*。从任意基础分支创建，lich 会在其中打开一个新会话。

## 各智能体支持情况

每个智能体都在真实终端里运行，都能恢复对话，都能在沙箱里工作，也都能访问你的其他会话。
区别在于 lich 能从会话里*读出*什么，因为每个 CLI 记录的内容都不一样。

<details>
<summary>查看各智能体支持什么</summary>

| 功能 | Claude Code | Codex | Antigravity | opencode | oh-my-pi | Crush | Cursor CLI | Kiro CLI |
| --- | :-: | :-: | :-: | :-: | :-: | :-: | :-: | :-: |
| 底栏显示上下文窗口 | 有 | 有 | 无 | 无 | 无 | 无 | 无 | 有 |
| 在底栏显示上一轮有多少来自提示缓存 | 有 | 有 | 无 | 在费用提示框中 | 在费用提示框中 | 无 | 无 | 无 |
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

</details>

每一处差异都是有意为之，每一处背后实测了什么，见 [`docs/ceilings.md`](docs/ceilings.md)。

## 配置

- **智能体。** 在设置 › 提供商里设置每个智能体的可执行文件路径，以及所有项目的默认智能体；
  项目也可以选自己的。没有设置路径时，lich 会运行登录 shell 的 `PATH` 里找到的第一个可执行文件，
  `PATH` 里有多个版本时 `lich doctor` 会发出警告。
- **底栏。** 在设置 › 外观里拖动各项，选择底栏显示什么。花费默认隐藏，因为只有按 token 计费时
  这个数字才有意义。
- **Worktree。** 项目里的 `.worktreeinclude` 文件决定哪些被 gitignore 忽略的文件会被复制到新
  worktree。
- **会话钩子。** 在设置里安装 [lich 插件](https://github.com/lichdotdev/lich-plugin)后，
  会话会自动给自己的卡片起标题，并在写入文件时立即刷新 git 状态。
- **脚本调用。** [docs/stability.md](docs/stability.md) 说明了 lich 的哪些部分可以放心用脚本调用。

## 隐私与更新

一切都在你的机器上运行：不需要账号，不需要登录，没有遥测。lich 只会为下面这几件事联网：

- **更新**：启动时和之后每小时向 GitHub Releases 检查一次新版本，启动时还会读取 lich 插件的发布列表。
- **套餐用量**：使用智能体已有的登录，查询各提供商自己的用量接口（Anthropic、OpenAI、Cursor、
  OpenCode Go），最多每五分钟一次。Antigravity 的套餐信息直接从 `agy` 读取。
- **价格**：GitHub 上 LiteLLM 的公开价格表，只在内置价格表里没有某个模型时下载一次。

Windows 安装程序装的版本会原地更新；通过 Homebrew、Scoop、AUR 或 Linux 软件包安装的，
则通过对应的包管理器更新，lich 会给出要执行的命令。
出了问题？`lich doctor` 会检查你的环境，`lich rage` 会把 bug 报告打包成一个压缩文件，不会上传
任何内容。在你把日志附上去之前，设置 › 帮助会说明日志里有哪些内容（路径、项目名和分支名、
你的 gh 登录名，绝不包含会话 token）。bug 和功能需求请提交到
[Issues](https://github.com/lichdotdev/lich/issues)；各版本的改动见
[CHANGELOG.md](CHANGELOG.md)。

## 从源码构建

lich 由两个二进制文件组成：`lich` 是静态的纯 Go 后端，`lich-native` 是窗口，在 `native/`
下用 cgo 构建。你需要 **Go 1.27.0+** 和 **[Task](https://taskfile.dev)**；窗口另需 C 工具链、
Gio 的系统软件包，以及用 `native/GHOSTTY_COMMIT` 记录的提交构建的 `libghostty-vt`。
构建方法见 [`native/slice.sh`](native/slice.sh)，窗口说明见
[`docs/native-window.md`](docs/native-window.md)。

```bash
task build          # 后端二进制文件 -> bin/lich
task build:native   # 窗口 -> bin/lich-native
task run            # 构建并运行
task test           # Go 测试套件
task package        # bin/ 下生成 .deb + .rpm + Arch .pkg.tar.zst（需要 nfpm）
```

再接入一个智能体，要改两个仓库里的十几个文件：
[`docs/adding-a-provider.md`](docs/adding-a-provider.md) 是完整的清单。

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
