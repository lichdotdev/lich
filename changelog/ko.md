## [0.71.2] - 2026-10-19

### Fixed

- 네이티브 창이 Windows와 macOS에서 다시 열립니다. 코드와 diff 뷰를 그리려고
  fontconfig를 찾다가 바로 닫히던 문제입니다.
- 네이티브 창 터미널의 이모지가 Windows에서 컬러로 그려집니다. 이전에는 윤곽선으로만
  나왔습니다.
- macOS 설치에 제안되는 업데이트 명령이 이제 업그레이드 전에 `lichdotdev/tap`을 tap합니다. 예전 `omartelo/tap`에서 설치한 경우 "Cask 'lichdotdev/tap/lich' is unavailable" 오류로 실패했는데, Homebrew가 옮겨진 tap을 스스로 따라가지 않기 때문입니다. 이전 lich에서 업그레이드하려면 `brew tap lichdotdev/tap`으로 한 번 tap하세요.
- 네이티브 창에서 "긴 붙여넣기 펼치기"가 이제 Claude Code와 Kiro CLI의 붙여넣기를 펼칩니다. 클립보드, 리뷰 코멘트 묶음, pull request나 issue가 세션을 열 때 쓰는 프롬프트, 드롭하거나 첨부한 파일 경로가 대상입니다.
