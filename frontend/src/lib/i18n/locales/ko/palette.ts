import type { Shape } from "../../catalog"
import type { palette as en } from "../en/palette"
export const palette = {
  commandPalette: {
    title: "명령 팔레트",
    placeholder: "세션, 프로젝트, 대화 내용으로 이동하거나 >를 입력해 작업 실행…",
    searchLabel: "세션과 프로젝트 검색",
    resultsLabel: "결과",
    filterLabel: "결과 필터",
    forgetFailed: "{label}을(를) 목록에서 지우지 못했습니다: {error}",
    resumeFailed: "{label}을(를) 재개하지 못했습니다: {error}",
    actionUnavailable: "여기서는 {label}을(를) 사용할 수 없습니다",
    showBeside: "옆에 표시",
    nothingClosed: "아직 닫은 항목이 없습니다",
    nothingClosedHint:
      "세션을 닫으면 worktree가 삭제될 때까지 브랜치, 에이전트, 대화와 함께 여기서 기다립니다.",
    noMatches: "{query}와(과) 일치하는 항목이 없습니다",
    noMatchesInTab: "{tab}에서 {query}와(과) 일치하는 항목이 없습니다",
    shownOfTotal: "{shown} / {total}",
    tab: {
      All: "전체",
      Sessions: "세션",
      Projects: "프로젝트",
      Messages: "메시지",
      History: "기록",
    },
    checkoutGone: "체크아웃 없음",
    matches: { other: "{count}개 일치" },
    sessions: { other: "세션 {count}개" },
    relocate: "위치 변경",
    reopen: "다시 열기",
  },
} satisfies Shape<typeof en>
