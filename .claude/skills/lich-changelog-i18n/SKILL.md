---
name: lich-changelog-i18n
description: Translate the latest release's CHANGELOG.md section into every interface language lich ships, writing changelog/<locale>.md, the files the What's new popup reads. Use when cutting a release (right after its section moves out of [Unreleased]), when TestTranslations in internal/patchnotes fails, or when a language is added.
---

# lich changelog translations

The What's new popup shows the running build's `CHANGELOG.md` section (`internal/patchnotes`). A user whose
interface language is not English reads it from `changelog/<locale>.md` instead, embedded in the binary
next to `CHANGELOG.md`. Each file holds **only the latest release**: a build only ever reads its own version,
and git keeps the old translations. `CHANGELOG.md` stays English and canonical; the release page reads it.

## When

Cutting a release (`CLAUDE.md` › Releases): once the section is under its `## [X.Y.Z] - YYYY-MM-DD` heading and
its text is final, run this before tagging, in the same commit. A fix to the released English text afterwards
means running it again.

## Steps

1. **Slice the source.** The release is the first `## [` heading in `CHANGELOG.md` below `## [Unreleased]`.
   Its section runs from that heading to the next `## ` heading.
2. **One file per locale, except English.** The locales are `prompt.Langs` (`internal/prompt/prompt.go`),
   file names are their tags as written there: `pt-BR.md`, `es.md`, `zh-CN.md`, `ja.md`, `ko.md`, `ru.md`.
   A language added to lich gets its file in the same PR.
3. **Overwrite each file** with the translated section and nothing else: no `# Changelog` title, no
   `[Unreleased]`, no older release, no compare links.
4. **Translate from the English**, never from another translation. The locales are independent, so each
   one can go to its own subagent in parallel, on a cheap model (Sonnet); hand each the English section and
   the rules below.
5. **Run the check** and finish only when it is green (needs nothing built):
   `go test -run TestTranslations ./internal/patchnotes`

## What stays in English, byte for byte

The parser and both windows read these, so `TestTranslations` fails when one is touched:

- The version heading, date included: `## [0.69.0] - 2026-10-16`.
- The group headings: `### Added`, `### Changed`, `### Deprecated`, `### Removed`, `### Fixed`,
  `### Security`. The windows translate the label on screen and color it by the English word.
- The alert openers: `> [!IMPORTANT]`, `> [!WARNING]`, `> [!NOTE]`. The text under them is translated, each
  line still starting with `> `.
- Every `code` span, exactly as written. Their order inside an item may follow the language's grammar.

## How to translate

- **Same structure.** Same groups in the same order, same number of `- ` items in each, same alert blocks.
  An item may wrap over as many lines as it needs; a blank line between items is fine; a blank line inside
  an item ends it.
- **The bold lead-in stays bold.** `**The native window opens on Windows.**` becomes one `**…**` in the
  translation, and the rest of the item follows it as plain text. No bold, italics or links the English
  does not have.
- **Names stay names:** lich, the providers (Claude Code, Codex, Antigravity, opencode, oh-my-pi, Crush,
  Cursor CLI, Kiro CLI), OS and app names (Linux, Windows, macOS, Wayland, Terminal.app), commands and key
  names, plus the rest of `lich-i18n`'s literal list.
- **Interface words match the interface.** A setting, menu or button named in the English (Settings ›
  Notifications, Remove worktree, Undo) is written the way that language's catalog shows it on
  screen: look it up in `native/i18n/locales/<lang>/` instead of translating it afresh. `›` stays as the path separator.
- **Plain release-note register** for that language, written for a developer: what changed for them, in
  the same tone and length as the English. Do not add, drop or explain.

## If the check fails

- `version headings = …`: the file holds another release, or more than one. Rewrite it from step 1.
- `N groups` / `N items`: an item was merged, split or lost, often a blank line inside an item.
- `group N is "…"`: a `### ` heading was translated.
- `code spans …` or `bold markers …`: a code span was changed or a `**` was lost; the message prints the
  item.

The check is the contract. Fix the translation, never the test.
