<div align="center">
  <img src="docs/media/appicon.svg" alt="lich" width="88" height="88" />
  <h1>lich</h1>
  <p><strong>English</strong> · <a href="README.pt-BR.md">Português (Brasil)</a> · <a href="README.es.md">Español</a> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a> · <a href="README.ru.md">Русский</a></p>
  <p>
    <a href="https://github.com/lichdotdev/lich/releases"><img alt="Release" src="https://img.shields.io/github/v/release/lichdotdev/lich?color=4285F4&label=release" /></a>
    <img alt="Platform" src="https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-333" />
    <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-AGPL--3.0-blue" /></a>
    <a href="https://github.com/sponsors/omartelo"><img alt="Sponsor" src="https://img.shields.io/github/sponsors/omartelo?color=ea4aaa&logo=githubsponsors&label=sponsors" /></a>
  </p>
  <p>
    <strong>Run your coding agents side by side, each on its own branch.</strong><br/>
    Claude Code, Codex, opencode or the agent you already use, each in its own worktree, all in one window.
  </p>
  <h3><a href="#install">Download lich</a> · <a href="https://lichdotdev.github.io/lich/">lichdotdev.github.io/lich</a></h3>
  <img src="docs/media/session.png" alt="Four Claude Code sessions side by side on one wall, each in its own git worktree. The sidebar lists them with their branch and diff badge, with the project's running services and the profile at its foot, and the footer shows the branch, the model and the plan" width="1000" />
  <!-- sponsor-logos: company logos go here, between the screenshot and Features -->
</div>

## Features

<table>
<tr>
<td width="50%" valign="middle">

### Agents side by side

Put sessions on a wall and lich lays out the panes for you. See at a glance which one is busy, which one is waiting on you and how much context it has left.

</td>
<td width="50%"><img src="docs/media/wall-mixed.png" alt="Claude Code and Codex sessions on one wall, each on its own branch" width="100%" /></td>
</tr>
<tr>
<td width="50%" valign="middle">

### A worktree per task, no setup

Branch a session off any base and lich copies your `.env` files and gives it a free dev-server port. Your uncommitted work can come along.

</td>
<td width="50%"><img src="docs/media/worktrees.png" alt="Sessions in the sidebar, each with its own branch and diff badge" width="100%" /></td>
</tr>
<tr>
<td width="50%" valign="middle">

### Review the diff, comment the lines

Read what the agent changed next to its session, comment on the lines that need work and send every note back as one prompt.

</td>
<td width="50%"><img src="docs/media/review-comments.png" alt="A diff with two line comments queued to send to the session" width="100%" /></td>
</tr>
<tr>
<td width="50%" valign="middle">

### Pull requests without the browser

Open a pull request, ask the branch's agent to write it, review it inline and merge it, all from the same window.

</td>
<td width="50%"><img src="docs/media/pull-request-review.png" alt="A pull request open in lich with its file tree, diff and a pending review comment" width="100%" /></td>
</tr>
<tr>
<td width="50%" valign="middle">

### Make it yours

Import a theme as JSON or install one from a git repository.

[Docs →](docs/themes.md)

</td>
<td width="50%"><img src="docs/media/themes.png" alt="The Appearance settings with the theme picker, fonts and footer layout" width="100%" /></td>
</tr>
</table>

And also:

- **Sessions that delegate.** One agent hands a task to another session and gets the answer back. The same works from your shell with the `lich` command ([docs](docs/cli.md)).
- **Sandboxed agents.** Let an agent skip permission prompts inside a sandbox that can't see your keys, your credentials or your other repositories ([limits](docs/ceilings.md)).
- **Your dev servers in the sidebar.** Save what a project keeps running and start or stop it with one click.
- **Find anything.** `Ctrl`/`Cmd`+`K` jumps to a session by name or by something said in it.
- **Know when you're needed.** A desktop notification when a session is waiting on you.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/lichdotdev/lich/main/install.sh | sh
```

One line: it detects your distro, checks the download and installs lich with your package manager.

| Platform | Get it |
| --- | --- |
| **Linux** | `install.sh` above, or AUR [`lich-bin`](https://aur.archlinux.org/packages/lich-bin) (`yay -S lich-bin`). Needs `zenity` and glibc 2.34 or newer (Debian 12, Ubuntu 22.04, RHEL 9 and up). |
| **macOS** | `brew install --cask lichdotdev/tap/lich` |
| **Windows** | Installer or portable zip from [Releases](https://github.com/lichdotdev/lich/releases), or `scoop install https://github.com/lichdotdev/lich/releases/latest/download/lich.json` |

The macOS and Windows builds are not signed yet, so Gatekeeper and SmartScreen
warn on a download from Releases; Homebrew skips the Gatekeeper prompt.
Per-distro packages, the Linux tarball, clearing the quarantine flag and
upgrading from the old Homebrew formula are in [INSTALL.md](INSTALL.md).

## Getting started

1. **Open a project.** The `+` in the tab strip lists what you closed recently
   and opens a folder picker. Point it at a git repository.
2. **Pick your agent.** The first launch lists the agents it found on your
   machine. Settings › Providers sets the default and each binary's path.
3. **Start a session.** *New Session* opens your agent in the project. The `+`
   on a checkout's header opens any agent, or a plain terminal, in that checkout.
4. **Give it a worktree** *(optional)*. Create one from any base branch and lich
   opens a fresh session in it.

## Provider support

Every agent runs in a real terminal, resumes its conversation, works in the
sandbox and reaches your other sessions. What changes from one to another is
what lich can read back out of it, since each CLI records different things.

<details>
<summary>See what each agent supports</summary>

| What you get | Claude Code | Codex | Antigravity | opencode | oh-my-pi | Crush | Cursor CLI | Kiro CLI |
| --- | :-: | :-: | :-: | :-: | :-: | :-: | :-: | :-: |
| Context window in the footer | yes | yes | no | no | no | no | no | yes |
| How much of the last turn came from the prompt cache, in the footer | yes | yes | no | in the cost tooltip | in the cost tooltip | no | no | no |
| The card's tooltip names the model running, and the one it opened on if it changed | yes | yes | no | no | no | no | no | yes |
| Cost in the footer | yes | yes | no | yes | yes | yes | no | credits |
| How much of your plan is left | yes | yes | yes | OpenCode Go only | no | no | yes | no |
| A turn your plan's usage limit stopped picks up again once the limit resets | yes | yes | no | its own retries | its own retries | no | no | no |
| Spinner while a turn runs, ring when it ends | yes | yes | yes | yes | yes | no | yes | yes |
| The card names itself after the conversation | yes | after your first prompt | after your first prompt | yes | yes, from the second turn | no | yes, once the first turn ends | after your first prompt, once the first turn ends |
| A pull request the agent opens shows on its card when the turn ends | yes | yes | yes | yes | yes | no, once the window regains focus | yes | yes |
| Repaints in the new look when you switch lich between a light and a dark theme | yes | no | no | yes | yes | no | yes | no |
| Bell when the agent is blocked on you | yes | yes | no | yes | no | no | no | no |
| Ctrl+Shift+Y jumps to the next session waiting on you | yes | yes | no | yes | no | no | no | no |
| The card says when the conversation is being compacted | yes | no | no | no | no | no | no | no |
| Sidebar filter by state: Waiting, Running, Unread | yes | yes | no Waiting | yes | no Waiting | no, always Idle | no Waiting | no Waiting |
| Run by your agent inside another session (`claude -p`, `cursor-agent -p`), it stays out of that session's card | yes | yes | yes | yes | yes | yes | yes | no |
| Machine kept out of idle sleep while a turn runs | yes | yes | yes | yes | yes | no | yes | yes |
| Review tab's **Last turn**, with the agent's recap | yes | yes | yes | yes | yes | no | yes | yes |
| Search the conversation from the palette | yes | yes | yes | yes | yes | yes | no | yes |
| Resume a conversation started outside lich, from the palette's History tab | yes | yes | yes | yes | yes | Linux; macOS and Windows untested | yes | yes |
| A long paste can land as editable text instead of a folded placeholder (Settings › Providers, off by default) | yes | no | no | yes | not measured | no, it becomes a `paste_N.txt` attachment | no | yes |
| Fork a conversation into a new worktree, or in its own checkout | yes | yes | no | yes | no | no | no | no |
| Warned when you resume, from the agent's own app, a conversation another card has open | yes | on your next prompt | on your next prompt, untested | no | yes | at its next tool call | no | yes |
| Open a session at a chosen reasoning effort | yes | yes | yes | no | yes | no | in the model name | yes |
| Open a session with ultracode on | yes | no | no | no | no | no | no | no |
| Several accounts side by side as profiles, each with its own full login, and a session moved to another account carrying on in the same card (Settings › Providers) | yes | yes, with its own plugins in each profile and Personal's settings shared by all of them | no | yes, logged in from a terminal card; a session cannot move to another profile, since its conversations sit in the database that holds the login | yes, logged in from a terminal card, with its settings shared by every profile | not measured | Linux and Windows (on macOS its login is in the Keychain), logged in from a terminal card, with `mcp.json`, hooks and skills shared by every profile | Linux only (macOS keeps its login in the Keychain, and on Windows nothing moves it), logged in from a terminal card, with its conversations shared by every profile |
| A folder you trusted in your main account opens without the "trust this folder?" question in every other profile | yes | yes | no profiles | asks nothing | asks nothing | not measured | yes | asks nothing |
| A sandboxed session opens without the "trust this folder?" question | yes | yes | yes | asks nothing | asks nothing | yes, and it no longer asks to initialize the project | yes | asks nothing |
| A worktree of a folder you trusted opens without the "trust this folder?" question | yes | yes | yes | asks nothing | asks nothing | yes, and it no longer asks to initialize the project | yes | asks nothing |
| Drive a running session from `lich control` or an agent: prompt, stop, model, effort, slash command | yes | no | no | no | no | no | no | no |
| A message from another session arrives without being typed into the terminal | yes | no | no | no | no | no | no | no |
| A subagent the agent starts runs as a lich session with its own card, in the asking session's checkout, hands its full report back and closes once done; its own subagents get cards too, one level down | yes | no | no | no | no | no | no | no |
| Ask a running session a side question without stopping it (`lich ask`) | yes | no | no | no | no | no | no | no |
| Context, cost and plan usage as the CLI measured them itself | yes | no | no | no | no | no | no | no |
| The terminal's status line shows who the session owes an answer, who it is waiting on, and answers ready to collect | yes | no | no | no | no | no | no | no |
| Delegate privately from a subagent or workflow step | yes | yes | yes | yes | yes | yes | yes | yes |
| A resumed session keeps the model and effort it was on | model only | yes | no | model | yes | no | model | model; effort untested |
| A result still waits for you after you interrupt a wait for it | yes | yes | yes | yes | no | yes | yes | yes |
| A task you hand another session reports back "unanswered" as soon as that agent ends its turn without replying | yes | yes | yes | yes | yes | no, waits for the deadline | yes | yes |
| lich's own messages to the agent (handed tasks, results ready, notices) in the prompt language you pick | yes | yes | yes | yes | yes | yes | yes | yes |
| Told at spawn, in the prompt language, to fan work out as lich sessions rather than hidden subagents | yes | no | no | no | yes | no | no | no |
| The session card opens a browser its agent drives, in a tab beside the session | yes | yes | yes | the card opens it; the agent uses `lich browser`, not `browser_*` | yes | yes | yes | yes |

</details>

Every gap is deliberate, and [`docs/ceilings.md`](docs/ceilings.md) says what was
measured behind each one.

## Configuration

- **Providers.** Settings › Providers sets each agent's binary and the default
  for every project; a project can choose its own. With no path set, lich runs
  the first binary on your login shell's `PATH` and `lich doctor` warns when
  there is more than one version.
- **Footer.** Settings › Appearance: drag its items to choose what it shows.
  The cost starts hidden, since it only means something when you pay per token.
- **Worktrees.** A `.worktreeinclude` file in the project picks which gitignored
  files a new worktree gets.
- **Session hooks.** Install the [lich plugin](https://github.com/lichdotdev/lich-plugin)
  from Settings and a session names its own card and refreshes git the moment
  it writes a file.
- **Scripting.** [docs/stability.md](docs/stability.md) says which parts of lich
  are safe to script against.

## Privacy & updates

Everything runs on your machine: no account, no sign-in, no telemetry. lich
only goes to the network for:

- **Updates**: a version check against GitHub Releases at startup and hourly,
  and the lich plugin's release list at startup.
- **Plan usage**: each provider's own usage endpoint (Anthropic, OpenAI, Cursor,
  OpenCode Go), with the login the agent already uses, at most every five
  minutes. Antigravity's plan is read from `agy` itself.
- **Prices**: LiteLLM's public price table on GitHub, once, for a model the
  bundled table does not know.

The Windows installer updates in place; Homebrew, Scoop, AUR and the Linux
packages update through their package manager, and lich gives you the command.
Something broken? `lich doctor` checks your setup and `lich rage` packs a bug
report into one archive without uploading it. Settings › Help says what the log
carries (paths, project and branch names, your gh login, never a session token)
before you attach it. Bugs and feature requests go to
[Issues](https://github.com/lichdotdev/lich/issues); what changed in each
version is in [CHANGELOG.md](CHANGELOG.md).

## Build from source

lich is two binaries: `lich`, a static pure-Go backend, and `lich-native`, the
window, built with cgo from `native/`. You need **Go 1.27.0+** and
**[Task](https://taskfile.dev)**; the window also needs a C toolchain, Gio's
system packages and a `libghostty-vt` built from the commit in
`native/GHOSTTY_COMMIT`. [`native/slice.sh`](native/slice.sh) says how, and
[`docs/native-window.md`](docs/native-window.md) covers the window.

```bash
task build          # backend binary -> bin/lich
task build:native   # window -> bin/lich-native
task run            # build + run
task test           # Go suites
task package        # .deb + .rpm + Arch .pkg.tar.zst in bin/ (needs nfpm)
```

Adding another agent touches a dozen files across two repositories:
[`docs/adding-a-provider.md`](docs/adding-a-provider.md) is the map.

## Sponsors

lich is written and maintained by one person. Sponsoring pays for the time that
goes into it and keeps the project independent: there is no paid tier of the app
and there will not be one.

[**Become a sponsor**](https://github.com/sponsors/omartelo)

<!-- sponsor-names: monthly sponsors go here -->

### Backers

<!-- backers: one-time supporters go here -->

Nobody yet.

## License

[AGPL-3.0-only](LICENSE) © 2026 omartelo

lich is free software: you can use, study, modify and redistribute it under
the terms of the GNU Affero General Public License v3. Any distributed or
network-served derivative must be released under the same license. Releases
up to and including v0.9.0 remain MIT-licensed.
