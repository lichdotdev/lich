---
name: lich-release-candidate
description: Publish a lich release candidate (vX.Y.Z-rc.N) through the release workflow, for the beta update channel. Use when asked to cut, tag or publish an RC or a pre-release, from reborn or any branch other than main's release flow, or when an RC's release run fails.
---

# Publishing a release candidate

An RC is a tag. Pushing it runs `.github/workflows/release.yml`, the same workflow a stable release runs.
That workflow builds and tests on Linux, Windows and both Macs, then publishes a GitHub **pre-release**.
Only installs on the beta channel (Settings › Updates › Channel) are offered it. Everything below is
where an RC differs from the stable flow in `CLAUDE.md` › Releases.

## What an RC leaves alone

- `CHANGELOG.md` stays as it is: no new `## [X.Y.Z]` section and no `changelog/<locale>.md` translation.
  The release notes are the `[Unreleased]` section at the tag. A section cut early would ship as the
  stable release's notes.
- The AUR, the Scoop manifest and the tap's `lich` cask. The `aur` job skips any tag with a `-`, the
  `homebrew` job updates only `lich@beta` for one, and `releases/latest`, which `install.sh` and Scoop read,
  never names a pre-release.
- Alert blocks. A `> [!CAUTION]` in `[Unreleased]` would make the RC a required update for every beta
  install. Alerts are written when the stable section is cut.

## Steps

1. **Pick the commit.** Use the tip of the branch the RC tests (today `reborn`), pushed to origin, with its
   CI green on that exact commit: `git fetch origin && gh run list --branch <branch> -L 5`. The local
   gate (`CLAUDE.md` › Local Gate) is green on it too, backend with `-race`. Done when one SHA satisfies
   all three.
2. **Name the tag.** Use `vX.Y.Z-rc.N`, where X.Y.Z is the stable release it leads to and N is one past
   the newest RC of that version: `git tag --list 'vX.Y.Z-rc.*' --sort=-v:refname | head -1`. Use exactly
   this shape: the updater only counts `X.Y.Z-label.N` as a pre-release (`semver.IsPreRelease`) and orders
   N numerically. Any other shape is invisible to the beta channel.
3. **Tag and push only the tag.** Make it lightweight, as every lich tag is: `git tag vX.Y.Z-rc.N <sha> &&
   git push origin vX.Y.Z-rc.N`.
4. **Watch the run.** `gh run list --workflow release.yml -L 3`, then `gh run watch <id>`. Expect it to be
   slower than a release from `main`: a tag from another branch cannot read the libghostty-vt cache saved
   on `main`, so the Intel mac rebuilds it (~14 min). Done when the run is green, or red with its failing
   job named (go to "When the run fails").
5. **Check the release.** `gh release view vX.Y.Z-rc.N --json isPrerelease,assets`. Done when
   `isPrerelease` is true and the assets are `checksums.txt`, `lich.json`, `PKGBUILD` and
   `lich-vX.Y.Z-rc.N-` + each of `amd64.deb`, `x86_64.rpm`, `x86_64.pkg.tar.zst`, `linux-amd64.tar.zst`,
   `windows-amd64.zip`, `windows-amd64-setup.exe`, `darwin-amd64.zip`, `darwin-arm64.zip`. Also run
   `gh release view --json tagName` with no tag: it must still name the last stable release.
   The tap's `Casks/lich@beta.rb` names the RC's version:
   `gh api repos/lichdotdev/homebrew-tap/contents/Casks/lich@beta.rb -q .content | base64 -d | grep version`.

## How beta installs take it

From `internal/appupdate`, for the record when a tester asks:

| Install | What updating does |
|---|---|
| Windows installer | The prompt's button runs the RC's `setup.exe` |
| deb, rpm | `install.sh` pinned with `LICH_TAG=vX.Y.Z-rc.N` |
| Arch | `makepkg -si` on the RC's `PKGBUILD`, under the AUR's `lich-bin` name |
| Homebrew | `brew upgrade --cask lichdotdev/tap/lich@beta`, swapping `lich` for it the first time |
| Scoop, tarball | The release page only |

## When the run fails

- **Failed before the `release` job published:** no install has seen the tag. Fix on the branch, then
  delete the tag (`git push --delete origin vX.Y.Z-rc.N && git tag -d vX.Y.Z-rc.N`) and go back to step 1
  with the same name.
- **Failed after the release was published:** beta installs may already have it. Keep the tag and the
  release, fix on the branch, and cut the next N.
- A failure on one OS is the reason RCs go through CI. Read the job log before changing anything:
  `gh api --allow-escape-sequences repos/lichdotdev/lich/actions/jobs/<job id>/logs`.
