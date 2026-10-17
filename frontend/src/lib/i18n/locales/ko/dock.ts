import type { Shape } from "../../catalog"
import type { dock as en } from "../en/dock"
export const dock = {
  rightDock: {
    fileBrowser: "파일 브라우저",
    reviewChanges: "변경 사항 리뷰",
    tabCode: "코드",
    tabReview: "리뷰",
    exitFullScreen: "전체 화면 종료",
    fullScreen: "전체 화면",
    closePanel: "패널 닫기",
    fileTree: "파일 트리",
    reviewPanel: "리뷰 패널",
    resizePanel: "패널 크기 조절",
  },
  searchResults: {
    prompt: "이 체크아웃의 모든 파일 내용 검색",
    searching: "검색 중…",
    noMatch: "“{query}”을(를) 포함한 파일이 없습니다",
  },
  filesPanel: {
    searchPlaceholder: "파일 내용 검색",
    filterPlaceholder: "이름으로 필터",
    searchLabel: "파일 내용 검색",
    filterLabel: "이름으로 파일 필터",
    matchesLabel: "검색 대상",
    byName: "이름",
    byText: "내용",
    readFailed: "이 폴더를 읽을 수 없습니다",
    loading: "불러오는 중…",
    noMatch: "일치하는 파일이 없습니다",
    empty: "파일이 없습니다",
    backToTree: "파일 트리로 돌아가기",
    readOnly: "읽기 전용",
  },
} satisfies Shape<typeof en>
