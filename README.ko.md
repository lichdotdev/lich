<div align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="frontend/public/appicon.png" />
    <img src="frontend/public/appicon-light.png" alt="lich" width="88" height="88" />
  </picture>
  <h1>lich</h1>
  <p><a href="README.md">English</a> · <a href="README.pt-BR.md">Português (Brasil)</a> · <a href="README.es.md">Español</a> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.ja.md">日本語</a> · <strong>한국어</strong> · <a href="README.ru.md">Русский</a></p>
  <p><strong>이미 쓰고 있는 코딩 에이전트를 위한, 터미널 우선 ADE(에이전트 개발 환경).</strong></p>
  <p>
    프로젝트를 열고, Claude Code, Codex, opencode 같은 에이전트를 진짜 터미널에서
    실행하고, git(worktree, diff, pull request)을 창을 벗어나지 않고 한눈에
    둡니다. 정적 Go 바이너리 하나, Electron 없음: UI는 Linux, Windows, macOS
    어디서나 lich에 내장된 Chromium에서 열립니다.
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
  <img src="docs/media/session.png" alt="한 분할 화면에 나란히 놓인 네 개의 Claude Code 세션, 각자 자기 git worktree에서 실행 중. 사이드바는 세션마다 브랜치와 diff 배지를 보여 주고, 푸터는 모델, 플랜, 브랜치를 보여 준다" width="1000" />
  <!-- sponsor-logos: company logos go here, between the screenshot and Why lich -->
</div>

> 이 문서는 영문 [README.md](README.md)를 번역한 것입니다. 영문판이 유일한 기준이며, 두 문서가 다르면 영문판을 따릅니다.

## 왜 lich인가

lich로 할 수 있는 일:

- **이미 가진 에이전트를 그대로 실행합니다.** [Claude Code](https://www.anthropic.com/claude-code),
  [Codex](https://github.com/openai/codex), Antigravity,
  [opencode](https://github.com/sst/opencode), oh-my-pi,
  [Crush](https://github.com/charmbracelet/crush),
  [Cursor CLI](https://cursor.com/docs/cli),
  [Kiro CLI](https://kiro.dev/docs/cli/)가 모두 여기서 같은 방식으로 실행됩니다. lich에
  각 바이너리 경로를 한 번만 알려 주고, 기본값을 고르거나 세션마다 선택하세요. lich가
  세션에서 *읽어 올* 수 있는 정보는 프로바이더마다 다르며,
  [프로바이더 지원](#프로바이더-지원)에 표로 정리되어 있습니다.
- **진짜 터미널을 유지합니다.** PTY 기반 셸을 프로젝트마다 여러 개 두고 GPU로
  렌더링하며, 검색 가능한 스크롤백은 페이지를 완전히 새로 고쳐도 남아 있습니다. 셸에
  엔트리포인트(`lazygit`, `k9s`, `pnpm dev`)를 지정하면 Linux, macOS, Windows
  어디서나 매번 그 도구로 바로 열립니다. 셸 rc나 PowerShell 프로필을 먼저 불러오지
  않으므로, 내 환경에서 동작하는 엔트리포인트는 공유받은 사람에게서도 동작합니다.
  푸터는 `cd`를 따라가며 브랜치를 보여 주고, Claude Code, Codex, Kiro CLI 세션이라면
  사용 중인 모델과 컨텍스트 윈도우도 보여 줍니다. Claude Code, Codex, Antigravity,
  Cursor CLI와 OpenCode Go 구독은 플랜의 롤링 윈도우가 얼마나 남았는지 보여 주고,
  원하면 세션이 쓴 비용도 보여 줍니다(비용을 기록하는 프로바이더에 한해).
- **한 세션이 다른 세션을 위해 일하게 합니다.** 작업을 다른 카드에 넘기면 양쪽에서
  무엇이 돌든 그 카드의 에이전트가 답을 돌려 씁니다. 에이전트는 spawn 시점에 받은
  도구(Claude Code와 Codex는 MCP)나 플러그인이 가져온 도구로 다른 세션에 닿습니다.
  이 모든 기능은 어느 셸에서나 `lich` 명령으로도 쓸 수 있고 `--json`도 지원하므로,
  스크립트가 에이전트 없이 세션을 조작할 수 있습니다([`docs/cli.md`](docs/cli.md)).
- **여러 세션을 한 번에 지켜봅니다.** 지금 보고 있는 세션 옆의 분할 화면에 어떤 세션이든
  올려 두면, lich가 패널 수와 창 크기에 맞춰 레이아웃을 직접 잡습니다. 여덟 개라면
  울트라와이드에서는 가로로 네 개씩, 노트북에서는 두 개씩 놓입니다. 다른 세션에 일을
  넘긴 세션은 클릭 한 번으로 자기 주위에 분할 화면을 꾸립니다. 분할 화면마다 이름이 있고, 프로젝트는
  분할 화면을 원하는 만큼 둘 수 있으며, 각 분할 화면은 사이드바에 접고, 이름을 바꾸고, 해체할 수 있는
  블록을 하나씩 가집니다.
- **설정 없이 worktree를 분기합니다.** 아무 base 브랜치에서나 worktree를 만들면 lich가
  gitignore된 `.env*` 파일을 채워 넣고, 다른 체크아웃이나 머신의 어떤 프로세스도 쓰지
  않는 dev 서버 포트를 배정하고, 에이전트가 시작되기 전에 프로젝트별 설정 스크립트를
  실행합니다. 체크아웃의 커밋되지 않은 작업에서 시작하면 그 변경 사항도 untracked 파일까지
  함께 따라옵니다.
- **세션을 격리해서 실행합니다.** 에이전트를 OS 샌드박스 안에서 열 수 있습니다. 해당
  프로바이더 자신의 상태만 담긴 새 빈 홈, 읽기 전용인 나머지 머신, 그리고 세션을 연
  체크아웃에 대한 쓰기 권한만 주어집니다. ssh 키, 클라우드 자격 증명, 디스크의 다른 모든
  저장소는 아예 보이지 않습니다. 에이전트가 권한 확인을 건너뛰게 할 때의 안전장치입니다.
  Linux는 bubblewrap, macOS는 `sandbox-exec`를 씁니다. 악의적인 코드에 대한 경계는
  아니며, 무엇을 막지 못하는지는 [`docs/ceilings.md`](docs/ceilings.md)에 적혀 있습니다.
- **diff를 읽는 그 자리에서 리뷰합니다.** CodeMirror 독이 작업 중인 변경 사항을 실시간
  파일 트리 옆에 보여 주고, 원하면 공백만 바뀐 변경은 접어 두며, 바뀐 이미지나 PDF는
  전후를 나란히 보여 줍니다. 선택 영역을 우클릭해 그 줄에 코멘트를 달면, 모은 코멘트가
  하나의 프롬프트로 세션에 붙여 넣어지고 전송은 하지 않습니다. Code 탭에서는 체크아웃을
  둘러보고 파일 내용을 검색할 수 있습니다.
- **pull request도 여기서 내보냅니다.** 저장소의 열린 pull request 목록을 보고, 하나를
  자기 worktree로 체크아웃한 뒤, diff를 읽고, 인라인으로 리뷰하고, base 브랜치가 실제로
  허용하는 방식으로 머지합니다. 아직 pull request가 없는 브랜치에서는 **Create with
  agent**가 그 브랜치의 세션에 pull request를 열어 달라고 요청하며, Enter를 누르기
  전에는 아무것도 전송되지 않습니다.

그 밖에: JSON으로 가져오거나 git 저장소에서 설치하는 [테마](docs/themes.md), 이름이나
대화 내용으로 이동하는 `Ctrl`/`Cmd`+`K` 팔레트, 세션이 나를 기다릴 때 뜨는 데스크톱 알림,
그리고 아예 시작되지 않을 때를 위한 `lich rage` / `lich doctor`가 있습니다.

활발히 개발 중입니다. 버그와 기능 요청은
[Issues](https://github.com/omartelo/lich/issues)에 남겨 주시고, 버전별 변경 사항은
[CHANGELOG.md](CHANGELOG.md)에 있습니다.

## 프로바이더 지원

모든 프로바이더는 진짜 터미널에서 spawn되고, 대화 id로 재개되며, 같은 샌드박스로
격리되고, 다른 세션에 닿는 방법도 똑같이 받습니다. 다른 점은 lich가 세션에서 *읽어 올*
수 있는 정보입니다. lich는 각 CLI가 기록하는 내용을 읽는데, 같은 것을 기록하는 CLI는
하나도 없기 때문입니다.

| 제공되는 기능 | Claude Code | Codex | Antigravity | opencode | oh-my-pi | Crush | Cursor CLI | Kiro CLI |
| --- | :-: | :-: | :-: | :-: | :-: | :-: | :-: | :-: |
| 푸터의 컨텍스트 윈도우 | 예 | 예 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 예 |
| 푸터의 비용 | 예 | 예 | 아니요 | 예 | 예 | 예 | 아니요 | 크레딧 |
| 플랜 잔여량 | 예 | 예 | 예 | OpenCode Go만 | 아니요 | 아니요 | 예 | 아니요 |
| 플랜 사용 한도로 멈춘 턴이 한도가 초기화되면 다시 이어짐 | 예 | 예 | 아니요 | 자체 재시도 | 자체 재시도 | 아니요 | 아니요 | 아니요 |
| 턴이 도는 동안 스피너, 끝나면 벨 | 예 | 예 | 예 | 예 | 예 | 아니요 | 예 | 예 |
| 에이전트가 연 pull request가 턴이 끝날 때 카드에 표시됨 | 예 | 예 | 예 | 예 | 예 | 아니요, 창이 다시 포커스를 받을 때 | 예 | 예 |
| lich를 라이트/다크 테마로 바꾸면 새 모양으로 다시 그려짐 | 예 | 아니요 | 아니요 | 예 | 예 | 아니요 | 예 | 아니요 |
| 에이전트가 나 때문에 막혔을 때 벨 | 예 | 예 | 아니요 | 예 | 아니요 | 아니요 | 아니요 | 아니요 |
| Ctrl+Shift+Y로 나를 기다리는 다음 세션으로 이동 | 예 | 예 | 아니요 | 예 | 아니요 | 아니요 | 아니요 | 아니요 |
| 대화가 압축(compact)되는 중이면 카드에 표시 | 예 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 |
| 사이드바 상태 필터: Waiting, Running, Unread | 예 | 예 | Waiting 없음 | 예 | Waiting 없음 | 아니요, 항상 Idle | Waiting 없음 | Waiting 없음 |
| 다른 세션 안에서 에이전트가 실행한 경우(`claude -p`, `cursor-agent -p`) 그 세션의 카드에 끼어들지 않음 | 예 | 예 | 예 | 예 | 예 | 예 | 예 | 아니요 |
| 턴이 도는 동안 머신이 유휴 절전에 들어가지 않음 | 예 | 예 | 예 | 예 | 예 | 아니요 | 예 | 예 |
| Review 탭의 **Last turn**과 에이전트의 요약 | 예 | 예 | 예 | 예 | 예 | 아니요 | 예 | 예 |
| 팔레트에서 대화 검색 | 예 | 예 | 예 | 예 | 예 | 예 | 아니요 | 예 |
| lich 밖에서 시작한 대화를 팔레트의 History 탭에서 재개 | 예 | 예 | 예 | 예 | 예 | Linux; macOS와 Windows는 미검증 | 예 | 예 |
| 긴 붙여 넣기를 접힌 자리 표시자 대신 편집 가능한 텍스트로 넣기(Settings › Providers, 기본값 꺼짐) | 예 | 아니요 | 아니요 | 예 | 측정 안 됨 | 아니요, `paste_N.txt` 첨부 파일이 됨 | 아니요 | 예 |
| 대화를 새 worktree로 포크 | 예 | 예 | 아니요 | 예 | 아니요 | 아니요 | 아니요 | 아니요 |
| 원하는 reasoning effort로 세션 열기 | 예 | 예 | 예 | 아니요 | 예 | 아니요 | 모델 이름으로 | 예 |
| ultracode를 켠 채로 세션 열기 | 예 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 |
| 여러 계정을 프로필로 나란히 두고 각자 완전한 로그인을 가지며, 다른 계정으로 옮긴 세션이 같은 카드에서 계속됨(네이티브 창, 실험적 기능: Settings › Providers) | 예 | 예, 프로필마다 자체 플러그인을 두고 Personal의 설정은 모든 프로필이 공유 | 아니요 | 예, 터미널 카드에서 로그인; 대화가 로그인 정보와 같은 데이터베이스에 있어서 세션을 다른 프로필로 옮길 수 없음 | 예, 터미널 카드에서 로그인하며 설정은 모든 프로필이 공유 | 측정 안 됨 | Linux와 Windows(macOS에서는 로그인이 키체인에 있음), 터미널 카드에서 로그인하며 `mcp.json`, hooks, skills는 모든 프로필이 공유 | Linux만(macOS는 로그인을 키체인에 두고, Windows에서는 그 위치를 옮길 방법이 없음), 터미널 카드에서 로그인하며 대화는 모든 프로필이 공유 |
| 기본 계정에서 신뢰한 폴더는 다른 모든 프로필에서도 "trust this folder?" 질문 없이 열림 | 예 | 예 | 프로필 없음 | 아무것도 묻지 않음 | 아무것도 묻지 않음 | 측정 안 됨 | 측정 안 됨 | 아무것도 묻지 않음 |
| 샌드박스 안의 세션은 "trust this folder?" 질문 없이 열림 | 예 | 아니요 | 아니요 | 아무것도 묻지 않음 | 아무것도 묻지 않음 | 아니요, 프로젝트 초기화 여부를 물음 | 아니요 | 아무것도 묻지 않음 |
| `lich control`이나 에이전트로 실행 중인 세션 조작: 프롬프트, 중지, 모델, effort, 슬래시 명령 | 예 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 |
| 다른 세션이 보낸 메시지가 터미널에 타이핑되지 않고 도착 | 예 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 |
| 에이전트가 시작한 서브에이전트가 자기 카드를 가진 lich 세션으로 요청한 세션의 체크아웃에서 실행되고, 전체 보고서를 돌려준 뒤 끝나면 닫힘; 그 서브에이전트의 서브에이전트도 한 단계 아래에 카드를 가짐 | 예 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 |
| 실행 중인 세션을 멈추지 않고 곁가지 질문하기(`lich ask`) | 예 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 |
| CLI가 직접 측정한 컨텍스트, 비용, 플랜 사용량 | 예 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 |
| 터미널 상태 줄에 세션이 답을 빚진 상대, 기다리는 상대, 수거할 준비가 된 답이 표시됨 | 예 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 |
| 서브에이전트나 workflow 단계에서 비공개로 위임 | 예 | 예 | 예 | 예 | 예 | 예 | 예 | 예 |
| 재개한 세션이 쓰던 모델과 effort를 유지 | 모델만 | 예 | 아니요 | 모델 | 예 | 아니요 | 모델 | 모델; effort는 미검증 |
| 결과를 기다리다 중단해도 결과가 계속 기다리고 있음 | 예 | 예 | 예 | 예 | 아니요 | 예 | 예 | 예 |
| 다른 세션에 넘긴 작업이, 그 에이전트가 답하지 않고 턴을 끝내는 즉시 "unanswered"로 보고됨 | 예 | 예 | 예 | 예 | 예 | 아니요, 마감 시각까지 기다림 | 예 | 예 |
| lich가 에이전트에게 보내는 메시지(넘긴 작업, 준비된 결과, 알림)가 선택한 프롬프트 언어로 전달됨 | 예 | 예 | 예 | 예 | 예 | 예 | 예 | 예 |
| spawn 시 프롬프트 언어로, 숨은 서브에이전트 대신 lich 세션으로 작업을 나누라는 안내를 받음 | 예 | 아니요 | 아니요 | 아니요 | 예 | 아니요 | 아니요 | 아니요 |

모든 빈칸은 의도된 것이며, CLI가 보고하는 정보를 lich가 감추는 경우는 하나도 없습니다.
각 빈칸 뒤에서 무엇을 측정했는지는 [`docs/ceilings.md`](docs/ceilings.md)에 있습니다.

## 설치

한 줄이면 됩니다. 배포판을 감지하고, 체크섬을 검증하고, 네이티브 패키지와 의존성을 패키지
매니저로 설치합니다:

```bash
curl -fsSL https://raw.githubusercontent.com/omartelo/lich/main/install.sh | sh
```

| 플랫폼 | 받는 방법 | 실행 시 필요한 것 |
| --- | --- | --- |
| **Linux** | 위의 `install.sh`, 또는 AUR [`lich-bin`](https://aur.archlinux.org/packages/lich-bin) (`yay -S lich-bin`) | `zenity`, 그리고 패키지에 포함된 창을 위한 glibc 2.34 이상(Debian 12, Ubuntu 22.04, RHEL 9 이상) |
| **macOS** | `brew install --cask lichdotdev/tap/lich` | 없음, Apple Silicon과 Intel 모두 앱에 창이 포함됨 |
| **Windows** | [Releases](https://github.com/omartelo/lich/releases)의 설치 프로그램이나 포터블 zip, 또는 Scoop: `scoop install https://github.com/omartelo/lich/releases/latest/download/lich.json` | 없음, 어느 쪽이든 창이 포함됨 |

배포판별 수동 패키지, Linux tarball, Windows 포터블 zip: [INSTALL.md](INSTALL.md). macOS와
Windows 바이너리는 서명되지 않았으므로, 공증/서명이 적용되기 전까지 Gatekeeper와
SmartScreen이 경고를 띄웁니다. Homebrew로 설치하면 Gatekeeper 경고를 피할 수 있고,
Releases 페이지에서 받은 파일은 quarantine 플래그를 직접 지워야 합니다.
macOS에서 cask는 `Lich.app`을 설치하므로, lich는 `/Applications`에 자기 아이콘을 갖고
실행 중에는 Dock에도 나타납니다. 예전 formula에서 업그레이드하려면 먼저
`brew uninstall lich`가 필요하며, 이유는 [INSTALL.md](INSTALL.md)에 있습니다.

## 시작하기

1. **설치**하고 `lich`를 실행합니다.
2. **프로젝트를 엽니다**: 탭 줄의 `+`가 최근에 닫은 항목을 보여 주고 OS 폴더 선택기를
   엽니다. git 저장소를 고르세요.
3. **lich에 에이전트를 알려 줍니다**: 첫 실행 때 머신에서 찾은 에이전트 목록이 나옵니다.
   Settings › Providers에서 각 바이너리 경로를 지정하고 기본값을 고를 수 있습니다.
   프로젝트는 같은 화면에서 기본값을 따르거나 다른 프로바이더를 고를 수 있습니다.
4. **세션을 시작합니다**: *New Session*이 프로젝트에서 에이전트를 실행하는 터미널을
   spawn합니다. 각 체크아웃 헤더에도 `+` 메뉴가 있어 활성화된 아무 프로바이더나 일반
   터미널을 바로 그 체크아웃에서 열 수 있습니다. 헤더 자체를 클릭하면 그 세션들을 접거나
   펼칩니다.
5. **worktree를 분기합니다** *(선택)*: 아무 base 브랜치에서 하나 만드세요. lich가 내용을
   채워 넣고 새 세션으로 데려다 줍니다.

## 설정

- **Providers**: Settings › Providers에서 각 프로바이더의 바이너리 경로와 모든 프로젝트의
  기본값을 지정합니다. 그 화면의 현재 프로젝트 행에서 이 선택을 덮어쓸 수 있고, **Clear**는
  덮어쓰기를 없애 이후 기본값 변경이 자동으로 반영되게 합니다. 경로를 지정하지 않으면 세션은
  로그인 셸의 `PATH`에서 처음 찾은 해당 프로바이더 바이너리를 실행합니다. 네이티브 창에서는
  Binary 블록이 그 파일과 버전을 보여 주고, `PATH`에 있는 다른 버전도 나열하며, `lich doctor`도
  이를 경고합니다. 하나를 고르려면 Binary에서 경로를 지정하세요(설정 키는
  `provider.<id>.bin`, 예: `provider.codex.bin`).
  프로바이더 섹션은 lich가 읽을 수 있을 때 플랜이 얼마나 남았는지부터 보여 줍니다.
  푸터에 무엇을 표시할지는 Settings › Appearance에서 항목을 끌어 정합니다. 비용 표시는
  토큰당 과금될 때만 의미가 있으므로 처음에는 숨겨져 있습니다.
- **Worktrees**: 프로젝트 체크아웃의 `.lich/setup-worktree.sh`는 새 worktree의 터미널에서
  에이전트보다 먼저 실행됩니다. New worktree 대화 상자가 이 스크립트를 보여 주고, 저장소에
  없으면 감지한 제안을 내놓습니다. `.worktreeinclude` 파일로 어떤 gitignore된 파일을
  복사할지 조정합니다.
- **Session hooks**: Settings에서
  [lich plugin](https://github.com/lichdotdev/lich-plugin)을 설치하면, 세션이 자기 카드의
  제목을 직접 정하고 파일을 쓰는 순간 git 상태를 새로 고칩니다.
- **안정성**: lich의 어느 부분을 스크립트에서 안심하고 써도 되는지, 어느 부분이 아무
  릴리스에서나 바뀔 수 있는지는 [docs/stability.md](docs/stability.md)에 적혀 있습니다.

## 개인 정보와 업데이트

모든 것이 내 머신에서 실행됩니다. 계정도, 로그인도, 텔레메트리도 없습니다. 백엔드는 토큰으로
인증되는 loopback 리스너입니다. lich가 스스로 네트워크에 접속하는 경우는 다음뿐입니다:

- **업데이트**: 시작할 때와 매시간 GitHub Releases에서 버전을 확인하고, 시작할 때 GitHub의
  lich plugin 릴리스 목록을 확인합니다.
- **플랜 사용량**: 각 프로바이더 자체의 사용량 엔드포인트(Anthropic, OpenAI, Cursor,
  OpenCode Go)에 세션의 CLI가 이미 쓰는 로그인으로, 최대 5분에 한 번 접속합니다.
  Antigravity의 플랜은 `agy` 자체에서 읽습니다.
- **가격**: 번들된 가격표에 없는 모델에 대해 한 번, GitHub에 있는 LiteLLM의 공개 가격표를
  받습니다.

Windows 설치 프로그램 빌드는 제자리에서 업데이트됩니다. Homebrew, Scoop, AUR, Linux 패키지로
설치했다면 해당 패키지 매니저로 업데이트하며, lich가 명령을 알려 줍니다.
버그 리포트에 로그 파일을 첨부하기 전에, Settings › Help가 로그에 무엇이 담기는지(경로,
프로젝트와 브랜치 이름, gh 로그인, 세션 토큰은 절대 아님) 알려 주며, `lich rage`는 그 리포트를
아무것도 업로드하지 않고 하나의 아카이브로 모읍니다.

## 소스에서 빌드

순수 Go 백엔드(Go 1.27, `CGO_ENABLED=0`)가 내장된 React 18 / TypeScript / Vite 프런트엔드를
토큰으로 인증되는 loopback 리스너(HTTP RPC + WebSockets)로 제공합니다. 터미널은 WebGL addon을
쓰는 xterm.js이고, 코드와 diff 화면은 CodeMirror 6입니다. Chromium 셸은 결정 기록으로 남아
있습니다: [`docs/chromium-shell.md`](docs/chromium-shell.md). 필요한 것은 **Go 1.27.0+**,
**Node + pnpm**, **[Task](https://taskfile.dev)**이며, Go 바이너리에는 C 툴체인이 필요 없습니다.
창(`shell/`, CEF 기반 Rust)에는 Rust 툴체인이 추가로 필요하고, Linux에서는 Chromium 개발
라이브러리, Windows에서는 MSVC도 필요합니다. 목록은 [CONTRIBUTING.md](CONTRIBUTING.md)에 있습니다.

```bash
task dev      # 핫 리로드 개발 모드(Vite는 :9245)
task build    # 프로덕션 바이너리 -> bin/lich
task run      # 빌드 + 실행
task test     # Go + 프런트엔드 테스트 스위트
```

Linux 릴리스를 로컬에서 패키징하려면
([nfpm](https://nfpm.goreleaser.com/) 필요):

```bash
task package   # bin/에 .deb + .rpm + Arch .pkg.tar.zst
```

lich가 실행하는 여덟 개에 에이전트 CLI를 하나 더 추가하는 일은 두 저장소에 걸쳐 십여 개 파일을
건드리는 유일한 변경입니다. 지도는 [`docs/adding-a-provider.md`](docs/adding-a-provider.md)입니다.

## 후원

lich는 한 사람이 작성하고 유지 관리합니다. 후원은 여기에 들어가는 시간을 지원하고 프로젝트를
독립적으로 유지해 줍니다. 앱에는 유료 등급이 없으며 앞으로도 없을 것입니다.

[**후원자 되기**](https://github.com/sponsors/omartelo)

<!-- sponsor-names: monthly sponsors go here -->

### 후원해 주신 분들

<!-- backers: one-time supporters go here -->

아직 아무도 없습니다.

## 라이선스

[AGPL-3.0-only](LICENSE) © 2026 omartelo

lich는 자유 소프트웨어입니다. GNU Affero General Public License v3의 조건에 따라 사용, 연구,
수정, 재배포할 수 있습니다. 배포되거나 네트워크로 제공되는 파생물은 같은 라이선스로 공개해야
합니다. v0.9.0까지(포함)의 릴리스는 MIT 라이선스로 남아 있습니다.
