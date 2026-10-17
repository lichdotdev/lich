import type { Shape } from "../../catalog"
import type { terminal as en } from "../en/terminal"
export const terminal = {
  dropHint: {
    attachTo: "{label}에 첨부",
    confined: "체크아웃 밖의 파일은 사본으로 전달됩니다",
    pasted: "경로가 프롬프트에 붙여 넣어집니다",
  },
  exitBanner: {
    restart: "다시 시작",
    close: "닫기",
  },
  searchBar: {
    placeholder: "찾기",
    label: "터미널 검색",
    previous: "이전 일치 항목",
    next: "다음 일치 항목",
    close: "검색 닫기",
  },
  drop: {
    notAttached: "첨부되지 않음: {files}",
  },
  host: {
    stopShowing: "{name} 표시 중지",
    paneName: "{name} 창",
    rememberFailed: "선택을 기억하지 못했습니다: {error}",
    showBeside: "이 세션 옆에 다른 세션 표시",
    noRoom: "창을 더 추가할 공간이 없습니다",
  },
  showBesidePicker: {
    title: "옆에 표시",
    placeholder: "옆에 표시할 세션 검색…",
    search: "옆에 표시할 세션 검색",
    results: "옆에 표시할 수 있는 세션",
    pick: "표시",
    noMatch: "{query}와(과) 일치하는 세션이 없습니다",
    onWall: "{group}에 있음",
  },
  view: {
    restartFailed: "세션을 다시 시작하지 못했습니다: {error}",
    startFailed: "세션을 시작하지 못했습니다: {error}",
    pasteSettingFailed:
      "긴 붙여넣기를 펼칠지 읽을 수 없어서 이번에는 한 번만 붙여 넣었습니다: {error}",
  },
  sessionExit: {
    ended: "세션 종료됨",
    endedWithCode: "세션이 코드 {code}(으)로 종료됨",
  },
  dropFiles: {
    folderConfined: "샌드박스 세션의 체크아웃 밖에 있는 폴더는 넘길 수 없습니다. 파일을 놓으세요",
    folderMissing: "이 세션이나 홈 아래에서 폴더를 찾을 수 없습니다. 파일을 놓으세요",
  },
  copyToast: {
    copied: {
      other: "{count}자를 클립보드에 복사함",
    },
  },
} satisfies Shape<typeof en>
