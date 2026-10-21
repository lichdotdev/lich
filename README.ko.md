<div align="center">
  <img src="docs/media/appicon.svg" alt="lich" width="88" height="88" />
  <h1>lich</h1>
  <p><a href="README.md">English</a> · <a href="README.pt-BR.md">Português (Brasil)</a> · <a href="README.es.md">Español</a> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.ja.md">日本語</a> · <strong>한국어</strong> · <a href="README.ru.md">Русский</a></p>
  <p><strong>이미 쓰고 있는 코딩 에이전트를 위한 터미널 중심 ADE.</strong></p>
  <p>
    프로젝트를 열고, Claude Code, Codex, opencode 같은 에이전트를 실제 터미널에서
    실행하고, worktree와 diff, 풀 리퀘스트까지 git 작업을 창을 벗어나지 않고 한눈에
    볼 수 있습니다. Electron도 브라우저도 쓰지 않습니다. Linux, Windows, macOS 모두에서
    lich는 Gio로 직접 그리는 네이티브 창으로 열립니다.
  </p>
  <p><a href="https://lichdotdev.github.io/lich/"><strong>lichdotdev.github.io/lich</strong></a></p>
  <p>
    <a href="https://github.com/lichdotdev/lich/releases"><img alt="Release" src="https://img.shields.io/github/v/release/lichdotdev/lich?color=4285F4&label=release" /></a>
    <img alt="Go" src="https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white" />
    <img alt="Window" src="https://img.shields.io/badge/window-native%20(Gio)-4285F4" />
    <img alt="Platform" src="https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-333" />
    <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-AGPL--3.0-blue" /></a>
    <a href="https://github.com/sponsors/omartelo"><img alt="Sponsor" src="https://img.shields.io/github/sponsors/omartelo?color=ea4aaa&logo=githubsponsors&label=sponsors" /></a>
  </p>
  <img src="docs/media/session.png" alt="분할 화면 하나에 나란히 띄운 Claude Code 세션 4개와 각 세션의 git worktree. 세션별 브랜치와 diff 배지를 보여 주는 사이드바, 모델과 플랜, 브랜치를 보여 주는 하단 바" width="1000" />
  <!-- sponsor-logos: company logos go here, between the screenshot and Why lich -->
</div>

> 이 문서는 영문 [README.md](README.md)를 번역한 것입니다. 기준은 영문판이며, 두 문서의 내용이 다르면 영문판을 따릅니다.

## 왜 lich인가

lich로 할 수 있는 일:

- **이미 쓰고 있는 에이전트를 그대로 실행합니다.** [Claude Code](https://www.anthropic.com/claude-code),
  [Codex](https://github.com/openai/codex), Antigravity,
  [opencode](https://github.com/sst/opencode), oh-my-pi,
  [Crush](https://github.com/charmbracelet/crush),
  [Cursor CLI](https://cursor.com/docs/cli),
  [Kiro CLI](https://kiro.dev/docs/cli/) 모두 같은 방식으로 실행됩니다. 바이너리 경로를
  한 번씩만 지정해 두면 기본값을 쓰거나 세션마다 골라 쓸 수 있습니다. 세션에서 lich가
  *읽어 올* 수 있는 정보는 프로바이더마다 다르며,
  [프로바이더 지원](#프로바이더-지원) 표에 정리해 두었습니다.
- **실제 터미널을 그대로 씁니다.** 프로젝트마다 PTY 기반 셸을 여러 개 띄울 수 있고,
  렌더링은 GPU가 맡습니다. 스크롤백은 검색할 수 있고 창을 닫았다가 다시 열어도
  사라지지 않습니다. 셸에 `lazygit`, `k9s`, `pnpm dev` 같은 엔트리포인트를 지정하면
  Linux, macOS, Windows 어디서든 매번 그 도구가 바로 열립니다. 셸 rc나 PowerShell
  프로필을 먼저 불러오지 않기 때문에, 내 환경에서 동작하는 엔트리포인트는 공유받은
  사람의 환경에서도 똑같이 동작합니다. 하단 바는 `cd`를 따라가며 브랜치를 보여 줍니다.
  Claude Code, Codex, Kiro CLI 세션이라면 사용 중인 모델과 컨텍스트 윈도우를, Claude
  Code, Codex, Antigravity, Cursor CLI와 OpenCode Go 구독이라면 플랜의 롤링 윈도우가
  얼마나 남았는지를 보여 주고, 원하면 세션에서 쓴 비용도 표시합니다(비용을 기록하는
  프로바이더만 해당).
- **한 세션에 다른 세션의 일을 맡깁니다.** 다른 카드에 작업을 넘기면 양쪽에서 무엇이
  돌고 있든 그 카드의 에이전트가 직접 답을 써서 돌려줍니다. 에이전트는 세션을 시작할 때
  받은 도구(Claude Code와 Codex는 MCP)나 플러그인이 제공하는 도구로 다른 세션에
  접근합니다. 이 기능은 전부 어느 셸에서나 `lich` 명령으로도 쓸 수 있고 `--json`도
  지원하므로, 에이전트 없이 스크립트만으로 세션을 조작할 수 있습니다([`docs/cli.md`](docs/cli.md)).
- **여러 세션을 한꺼번에 지켜봅니다.** 지금 보고 있는 세션 옆 분할 화면에 어떤 세션이든
  올려 두면 패널 수와 창 크기에 맞춰 lich가 알아서 배치합니다. 세션 8개라면 울트라와이드
  모니터에서는 한 줄에 4개씩, 노트북에서는 2개씩 놓입니다. 다른 세션에 일을 넘긴 세션은
  클릭 한 번으로 자신을 중심으로 분할 화면을 구성합니다. 분할 화면마다 이름이 붙고,
  프로젝트 하나에 원하는 만큼 둘 수 있으며, 사이드바에 생기는 블록에서 접거나 이름을
  바꾸거나 해체할 수 있습니다.
- **번거로운 설정 없이 worktree를 만듭니다.** 원하는 베이스 브랜치에서 worktree를 만들면
  lich가 gitignore된 `.env*` 파일을 채워 넣고, 다른 체크아웃도 머신의 다른 프로세스도
  쓰지 않는 개발 서버 포트를 배정합니다. 체크아웃의 커밋하지 않은 작업에서 시작하면 untracked 파일을
  포함한 변경 사항도 함께 옮겨 갑니다.
- **세션을 격리해서 실행합니다.** 에이전트를 OS 샌드박스 안에서 열 수 있습니다. 해당
  프로바이더의 상태만 담긴 빈 홈 디렉터리가 새로 주어지고, 머신의 나머지 부분은 읽기
  전용이며, 쓰기는 세션을 연 체크아웃에만 허용됩니다. ssh 키, 클라우드 자격 증명,
  디스크에 있는 다른 저장소는 샌드박스 안에서 아예 보이지 않습니다. 에이전트가 권한
  확인을 건너뛰도록 허용할 때 위험을 상쇄해 주는 장치입니다. Linux에서는 bubblewrap,
  macOS에서는 `sandbox-exec`를 씁니다. 악성 코드를 막아 주는 보안 경계는 아니며, 무엇을
  막지 못하는지는 [`docs/ceilings.md`](docs/ceilings.md)에 정리되어 있습니다.
- **diff를 읽는 바로 그 자리에서 리뷰합니다.** diff 독이 작업 중인 변경 사항을
  실시간 파일 트리 옆에 보여 줍니다. 원하면 공백만 바뀐 변경은 접어 두고, 바뀐 이미지나
  PDF는 변경 전후를 함께 보여 줍니다. 선택 영역을 우클릭하면 해당 줄에 코멘트를 달 수
  있고, 모아 둔 코멘트는 프롬프트 하나로 묶여 세션에 붙여 넣어지며 전송되지는 않습니다.
  코드 탭에서는 체크아웃을 둘러보고 파일 내용을 검색할 수 있습니다.
- **풀 리퀘스트까지 여기서 끝냅니다.** 저장소의 열린 풀 리퀘스트 목록을 보고, 그중 하나를
  별도의 worktree에 체크아웃해 diff를 읽고, 인라인으로 리뷰하고, 베이스 브랜치가 실제로
  허용하는 방식으로 병합합니다. 아직 풀 리퀘스트가 없는 브랜치에서는 **에이전트로
  만들기**를 누르면 그 브랜치의 세션에 PR을 열어 달라고 요청하며, Enter를 누르기 전에는
  아무것도 전송되지 않습니다.

그 밖에도 JSON으로 가져오거나 git 저장소에서 설치하는 [테마](docs/themes.md), 세션 이름이나
대화 내용으로 바로 이동하는 `Ctrl`/`Cmd`+`K` 팔레트, 세션이 내 응답을 기다릴 때 뜨는 데스크톱
알림, lich가 아예 시작되지 않을 때 쓰는 `lich rage` / `lich doctor`가 있습니다.

현재 활발하게 개발하고 있습니다. 버그와 기능 요청은
[이슈](https://github.com/lichdotdev/lich/issues)에 남겨 주세요. 버전별 변경 사항은
[CHANGELOG.md](CHANGELOG.md)에서 볼 수 있습니다.

## 프로바이더 지원

모든 프로바이더는 실제 터미널에서 실행되고, 대화 id로 재개되며, 같은 샌드박스로 격리되고,
다른 세션에 접근하는 방법도 똑같이 제공받습니다. 차이는 lich가 세션에서 *읽어 올* 수 있는
정보에 있습니다. lich는 각 CLI가 기록하는 내용을 읽는데, 기록하는 내용이 똑같은 CLI는 하나도
없기 때문입니다.

| 기능 | Claude Code | Codex | Antigravity | opencode | oh-my-pi | Crush | Cursor CLI | Kiro CLI |
| --- | :-: | :-: | :-: | :-: | :-: | :-: | :-: | :-: |
| 하단 바에 컨텍스트 윈도우 표시 | 예 | 예 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 예 |
| 카드 툴팁에 실행 중인 모델 표시 (시작할 때와 바뀌었으면 시작 모델도) | 예 | 예 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 예 |
| 하단 바에 비용 표시 | 예 | 예 | 아니요 | 예 | 예 | 예 | 아니요 | 크레딧 |
| 남은 플랜 사용량 | 예 | 예 | 예 | OpenCode Go만 | 아니요 | 아니요 | 예 | 아니요 |
| 플랜 사용 한도에 걸려 멈춘 턴이 한도가 초기화되면 다시 이어짐 | 예 | 예 | 아니요 | CLI가 직접 재시도 | CLI가 직접 재시도 | 아니요 | 아니요 | 아니요 |
| 턴이 실행되는 동안 스피너, 끝나면 벨 | 예 | 예 | 예 | 예 | 예 | 아니요 | 예 | 예 |
| 카드 이름이 대화에 맞춰 붙음 | 예 | 첫 프롬프트로 | 첫 프롬프트로 | 예 | 예, 두 번째 턴부터 | 아니요 | 예, 첫 턴이 끝날 때 | 첫 프롬프트로, 첫 턴이 끝날 때 |
| 에이전트가 연 풀 리퀘스트가 턴이 끝나면 카드에 표시됨 | 예 | 예 | 예 | 예 | 예 | 아니요, 창이 다시 포커스를 받을 때 표시됨 | 예 | 예 |
| lich를 라이트/다크 테마로 전환하면 바뀐 모양으로 다시 그려짐 | 예 | 아니요 | 아니요 | 예 | 예 | 아니요 | 예 | 아니요 |
| 에이전트가 내 응답을 기다리며 막혔을 때 벨 | 예 | 예 | 아니요 | 예 | 아니요 | 아니요 | 아니요 | 아니요 |
| Ctrl+Shift+Y로 내 응답을 기다리는 다음 세션으로 이동 | 예 | 예 | 아니요 | 예 | 아니요 | 아니요 | 아니요 | 아니요 |
| 대화가 압축(compact)되는 중이면 카드에 표시됨 | 예 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 |
| 사이드바 상태 필터: 대기 중, 실행 중, 읽지 않음 | 예 | 예 | 대기 중 없음 | 예 | 대기 중 없음 | 아니요, 항상 유휴 | 대기 중 없음 | 대기 중 없음 |
| 다른 세션 안에서 에이전트가 실행한 경우(`claude -p`, `cursor-agent -p`) 그 세션의 카드에 끼어들지 않음 | 예 | 예 | 예 | 예 | 예 | 예 | 예 | 아니요 |
| 턴이 실행되는 동안 머신이 유휴 절전에 들어가지 않음 | 예 | 예 | 예 | 예 | 예 | 아니요 | 예 | 예 |
| 리뷰 탭의 **마지막 턴**과 에이전트의 요약 | 예 | 예 | 예 | 예 | 예 | 아니요 | 예 | 예 |
| 팔레트에서 대화 내용 검색 | 예 | 예 | 예 | 예 | 예 | 예 | 아니요 | 예 |
| lich 밖에서 시작한 대화를 팔레트의 기록 탭에서 재개 | 예 | 예 | 예 | 예 | 예 | Linux 지원, macOS와 Windows는 미검증 | 예 | 예 |
| 긴 붙여넣기를 접힌 자리 표시자 대신 편집 가능한 텍스트로 넣기(설정 › 프로바이더, 기본값은 꺼짐) | 예 | 아니요 | 아니요 | 예 | 측정 안 됨 | 아니요, `paste_N.txt` 첨부 파일이 됨 | 아니요 | 예 |
| 대화를 새 worktree로 포크, 같은 체크아웃에서도 포크 | 예 | 예 | 아니요 | 예 | 아니요 | 아니요 | 아니요 | 아니요 |
| 다른 카드에 열려 있는 대화를 에이전트 앱에서 직접 재개하면 경고 | 예 | 다음 프롬프트에서 | 다음 프롬프트에서, 미검증 | 아니요 | 예 | 다음 도구 호출 때 | 아니요 | 예 |
| 원하는 reasoning effort로 세션 열기 | 예 | 예 | 예 | 아니요 | 예 | 아니요 | 모델 이름으로 지정 | 예 |
| ultracode를 켠 채로 세션 열기 | 예 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 |
| 여러 계정을 프로필로 나란히 두고 프로필마다 따로 로그인하며, 다른 계정으로 옮긴 세션이 같은 카드에서 이어짐(설정 › 프로바이더) | 예 | 예, 플러그인은 프로필마다 따로 두고 개인 프로필의 설정은 모든 프로필이 공유 | 아니요 | 예, 터미널 카드에서 로그인. 대화가 로그인 정보와 같은 데이터베이스에 있어서 세션을 다른 프로필로 옮길 수 없음 | 예, 터미널 카드에서 로그인하며 설정은 모든 프로필이 공유 | 측정 안 됨 | Linux와 Windows(macOS에서는 로그인 정보가 키체인에 있음), 터미널 카드에서 로그인하며 `mcp.json`, hooks, skills는 모든 프로필이 공유 | Linux만(macOS는 로그인 정보를 키체인에 두고, Windows에서는 그 위치를 옮길 방법이 없음), 터미널 카드에서 로그인하며 대화는 모든 프로필이 공유 |
| 기본 계정에서 신뢰한 폴더는 다른 모든 프로필에서도 "trust this folder?" 질문 없이 열림 | 예 | 예 | 프로필 없음 | 아무것도 묻지 않음 | 아무것도 묻지 않음 | 측정 안 됨 | 예 | 아무것도 묻지 않음 |
| 샌드박스 안의 세션은 "trust this folder?" 질문 없이 열림 | 예 | 예 | 예 | 아무것도 묻지 않음 | 아무것도 묻지 않음 | 예, 프로젝트 초기화 여부도 더는 묻지 않음 | 예 | 아무것도 묻지 않음 |
| 신뢰한 폴더의 worktree는 "trust this folder?" 질문 없이 열림 | 예 | 예 | 예 | 아무것도 묻지 않음 | 아무것도 묻지 않음 | 예, 프로젝트 초기화 여부도 더는 묻지 않음 | 예 | 아무것도 묻지 않음 |
| `lich control`이나 에이전트로 실행 중인 세션 조작: 프롬프트, 중지, 모델, effort, 슬래시 명령 | 예 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 |
| 다른 세션이 보낸 메시지가 터미널에 타이핑되지 않고 전달됨 | 예 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 |
| 에이전트가 시작한 서브에이전트가 전용 카드를 가진 lich 세션으로 요청한 세션의 체크아웃에서 실행되고, 전체 보고서를 돌려준 뒤 끝나면 닫힘. 그 서브에이전트가 시작한 서브에이전트도 한 단계 아래에 카드를 가짐 | 예 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 |
| 실행 중인 세션을 멈추지 않고 따로 질문하기(`lich ask`) | 예 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 |
| CLI가 직접 측정한 컨텍스트, 비용, 플랜 사용량 | 예 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 |
| 터미널 상태 줄에 세션이 답해야 할 상대, 기다리고 있는 상대, 받아 갈 수 있는 답이 표시됨 | 예 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 | 아니요 |
| 서브에이전트나 workflow 단계에서 비공개로 위임 | 예 | 예 | 예 | 예 | 예 | 예 | 예 | 예 |
| 재개한 세션이 쓰던 모델과 effort를 유지 | 모델만 | 예 | 아니요 | 모델 | 예 | 아니요 | 모델 | 모델, effort는 미검증 |
| 결과를 기다리다 중단해도 결과를 나중에 받을 수 있음 | 예 | 예 | 예 | 예 | 아니요 | 예 | 예 | 예 |
| 다른 세션에 넘긴 작업은 그 에이전트가 답하지 않고 턴을 끝내는 즉시 "unanswered"로 보고됨 | 예 | 예 | 예 | 예 | 예 | 아니요, 마감 시각까지 기다림 | 예 | 예 |
| lich가 에이전트에게 보내는 메시지(넘긴 작업, 준비된 결과, 알림)가 선택한 프롬프트 언어로 전달됨 | 예 | 예 | 예 | 예 | 예 | 예 | 예 | 예 |
| 세션을 시작할 때 숨은 서브에이전트 대신 lich 세션으로 작업을 나누라는 안내를 프롬프트 언어로 받음 | 예 | 아니요 | 아니요 | 아니요 | 예 | 아니요 | 아니요 | 아니요 |

빈칸은 모두 의도된 것이며, CLI가 보고하는 정보를 lich가 숨기는 경우는 없습니다. 빈칸마다
무엇을 측정했는지는 [`docs/ceilings.md`](docs/ceilings.md)에 정리되어 있습니다.

## 설치

한 줄이면 됩니다. 배포판을 감지하고 체크섬을 검증한 다음, 패키지 매니저로 네이티브 패키지와
의존성을 설치합니다.

```bash
curl -fsSL https://raw.githubusercontent.com/lichdotdev/lich/main/install.sh | sh
```

| 플랫폼 | 설치 방법 | 실행에 필요한 것 |
| --- | --- | --- |
| **Linux** | 위의 `install.sh` 또는 AUR [`lich-bin`](https://aur.archlinux.org/packages/lich-bin) (`yay -S lich-bin`) | `zenity`, 그리고 패키지에 포함된 창을 실행하려면 glibc 2.34 이상(Debian 12, Ubuntu 22.04, RHEL 9 이상) |
| **macOS** | `brew install --cask lichdotdev/tap/lich` | 없음. Apple Silicon과 Intel 모두 창이 앱에 포함되어 있음 |
| **Windows** | [Releases](https://github.com/lichdotdev/lich/releases)의 설치 프로그램이나 포터블 zip, 또는 Scoop: `scoop install https://github.com/lichdotdev/lich/releases/latest/download/lich.json` | 없음. 어느 쪽이든 창이 포함되어 있음 |

배포판별 수동 설치 패키지, Linux tarball, Windows 포터블 zip은 [INSTALL.md](INSTALL.md)를
참고하세요. macOS와 Windows 바이너리는 서명되지 않았기 때문에, 공증과 서명이 적용될 때까지
Gatekeeper와 SmartScreen이 경고를 띄웁니다. Homebrew로 설치하면 Gatekeeper 경고가 뜨지
않지만, Releases 페이지에서 받은 파일은 quarantine 플래그를 직접 지워야 합니다.
macOS에서는 cask가 `Lich.app`을 설치하므로 `/Applications`에 lich 아이콘이 생기고, 실행
중에는 Dock에도 표시됩니다. 예전 formula에서 업그레이드하려면 먼저 `brew uninstall lich`를
실행해야 하며, 그 이유는 [INSTALL.md](INSTALL.md)에 있습니다.

## 시작하기

1. **설치**한 다음 `lich`를 실행합니다.
2. **프로젝트를 엽니다.** 탭 줄의 `+`를 누르면 최근에 닫은 항목이 나오고 OS 폴더 선택기가
   열립니다. 여기서 git 저장소를 고르세요.
3. **에이전트를 지정합니다.** 처음 실행하면 머신에서 찾은 에이전트 목록이 나옵니다.
   설정 › 프로바이더에서 바이너리 경로를 각각 지정하고 기본값을 고를 수 있습니다.
   프로젝트마다 같은 화면에서 기본값을 따를지, 다른 프로바이더를 쓸지 정할 수 있습니다.
4. **세션을 시작합니다.** *새 세션*을 누르면 프로젝트에서 에이전트를 실행하는 터미널이
   열립니다. 체크아웃 헤더마다 `+` 메뉴도 있어서, 활성화된 프로바이더나 일반 터미널을 바로
   그 체크아웃에서 열 수 있습니다. 헤더 자체를 클릭하면 그 체크아웃의 세션을 접거나 펼칩니다.
5. **worktree를 만듭니다** *(선택)*. 원하는 베이스 브랜치에서 하나 만들면 lich가 필요한
   파일을 채워 넣고 새 세션으로 바로 연결해 줍니다.

## 설정

- **프로바이더**: 설정 › 프로바이더에서 프로바이더별 바이너리 경로와 모든 프로젝트에 적용할
  기본값을 지정합니다. 같은 화면의 현재 프로젝트 행에서 이 선택을 덮어쓸 수 있고, **지우기**를
  누르면 덮어쓴 값이 사라져 이후 기본값을 바꾸면 자동으로 반영됩니다. 경로를 지정하지 않으면
  세션은 로그인 셸의 `PATH`에서 가장 먼저 찾은 프로바이더 바이너리를 실행합니다.
  바이너리 블록에 그 파일과 버전이 표시되고 `PATH`에 있는 다른 버전도 함께
  나열되며, `lich doctor`도 이를 경고합니다. 특정 바이너리를 쓰려면 바이너리 블록에서 경로를
  지정하세요(설정 키는 `provider.<id>.bin`, 예: `provider.codex.bin`).
  lich가 플랜 정보를 읽을 수 있으면 프로바이더 섹션 맨 위에 남은 플랜 사용량이 표시됩니다.
  하단 바에 표시할 항목은 설정 › 모양에서 끌어다 놓아 정합니다. 비용은 토큰당 과금될 때만
  의미가 있으므로 처음에는 숨겨져 있습니다.
- **worktree**: 어떤 gitignore된 파일을 새 worktree에 복사할지는 프로젝트 체크아웃의
  `.worktreeinclude` 파일로 조정합니다.
- **세션 훅**: 설정에서
  [lich 플러그인](https://github.com/lichdotdev/lich-plugin)을 설치하면 세션이 자기 카드의
  제목을 직접 정하고, 파일을 쓰는 즉시 git 상태를 새로 고칩니다.
- **안정성**: lich에서 스크립트로 안심하고 다뤄도 되는 부분과 릴리스마다 바뀔 수 있는 부분은
  [docs/stability.md](docs/stability.md)에 정리되어 있습니다.

## 개인 정보와 업데이트

모든 것이 내 머신에서 실행됩니다. 계정도, 로그인도, 텔레메트리도 없으며, 백엔드는 토큰으로
인증하는 loopback 리스너입니다. lich가 스스로 네트워크에 접속하는 경우는 아래가 전부입니다.

- **업데이트**: 시작할 때와 매시간 GitHub Releases에서 새 버전을 확인하고, 시작할 때 GitHub에서
  lich 플러그인 릴리스 목록을 확인합니다.
- **플랜 사용량**: 각 프로바이더의 사용량 엔드포인트(Anthropic, OpenAI, Cursor, OpenCode Go)에
  세션의 CLI가 이미 쓰고 있는 로그인 정보로 접속하며, 최대 5분에 한 번입니다. Antigravity의
  플랜은 `agy`에서 직접 읽습니다.
- **가격**: 번들된 가격표에 없는 모델이 있을 때 한 번, GitHub에 공개된 LiteLLM 가격표를
  받아 옵니다.

Windows 설치 프로그램으로 설치한 빌드는 lich가 직접 업데이트합니다. Homebrew, Scoop, AUR,
Linux 패키지로 설치했다면 해당 패키지 매니저로 업데이트하며, 실행할 명령은 lich가 알려 줍니다.
버그 리포트에 로그 파일을 첨부하기 전에 설정 › 도움말에서 로그에 무엇이 담기는지(경로,
프로젝트와 브랜치 이름, gh 로그인 계정이며 세션 토큰은 절대 담기지 않음) 확인할 수 있고,
`lich rage`는 아무것도 업로드하지 않고 리포트를 아카이브 하나로 모아 줍니다.

## 소스에서 빌드

lich는 바이너리 두 개로 이루어집니다. 백엔드 `lich`는 순수 Go(Go 1.27, `CGO_ENABLED=0`,
정적 바이너리 하나)이며, 토큰으로 인증하는 loopback 리스너(HTTP RPC + WebSockets)로 세션을
서빙합니다. 창 `lich-native`는 `native/`에 있는 별도의 Go 모듈로, cgo로 빌드합니다. 창은
[Gio](https://gioui.org)가 그리고, 터미널은 `libghostty-vt`가, 코드와 diff 화면은
[godemirror](https://github.com/lichdotdev/godemirror)가 맡습니다. 필요한 것은 **Go 1.27.0+**와
**[Task](https://taskfile.dev)**이며, 창을 빌드하려면 C 툴체인과 Gio의 시스템 패키지, 그리고
`native/GHOSTTY_COMMIT`에 적힌 Ghostty 커밋에서 Zig 0.16으로 빌드한 `libghostty-vt`가 추가로
필요합니다. 빌드 방법은 [`native/slice.sh`](native/slice.sh)에, 창에 대한 설명은
[`docs/native-window.md`](docs/native-window.md)에 있습니다.

```bash
task build          # 백엔드 바이너리 -> bin/lich
task build:native   # 창 -> bin/lich-native
task run            # 빌드 + 실행
task test           # Go 테스트 스위트
```

Linux 릴리스를 로컬에서 패키징하려면
[nfpm](https://nfpm.goreleaser.com/)이 필요합니다.

```bash
task package   # bin/에 .deb + .rpm + Arch .pkg.tar.zst
```

lich가 실행하는 에이전트 CLI 8종에 하나를 더 추가하려면 두 저장소에 걸쳐 파일 십여 개를
고쳐야 하며, 이만큼 손이 가는 변경은 이것 하나뿐입니다. 어디를 고쳐야 하는지는
[`docs/adding-a-provider.md`](docs/adding-a-provider.md)에 정리되어 있습니다.

## 후원

lich는 한 사람이 만들고 유지 관리합니다. 후원금은 프로젝트에 들이는 시간을 뒷받침하고
프로젝트가 독립성을 지키게 해 줍니다. 앱에 유료 요금제는 없으며 앞으로도 생기지 않습니다.

[**후원하기**](https://github.com/sponsors/omartelo)

<!-- sponsor-names: monthly sponsors go here -->

### 후원해 주신 분들

<!-- backers: one-time supporters go here -->

아직 없습니다.

## 라이선스

[AGPL-3.0-only](LICENSE) © 2026 omartelo

lich는 자유 소프트웨어입니다. GNU Affero General Public License v3 조건에 따라 사용, 연구,
수정, 재배포할 수 있습니다. 배포하거나 네트워크로 제공하는 파생 저작물은 같은 라이선스로
공개해야 합니다. v0.9.0 이하 릴리스는 계속 MIT 라이선스를 따릅니다.
