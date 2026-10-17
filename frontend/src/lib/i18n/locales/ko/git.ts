import type { Shape } from "../../catalog"
import type { git as en } from "../en/git"
export const git = {
  baseStatus: {
    behind: {
      other: "{base}보다 커밋 {count}개 뒤처짐",
    },
    conflict: { other: "파일 {count}개에 충돌" },
  },
  fileSearch: {
    cutMatches: "{count}개 이상 일치",
    summary: "{files}에서 {matches}",
    matches: { other: "{count}개 일치" },
    cutNote: "일치하는 처음 {count}줄만 표시합니다. 검색 범위를 좁히세요.",
    tooLarge: {
      other: "1 MB를 넘는 파일 {count}개는 검색하지 않았습니다.",
    },
  },
  fileTree: {
    hidden: "숨김: {names}.",
    cut: "이 폴더에는 트리에 표시할 수 있는 것보다 많은 파일이 있습니다.",
  },
  lastTurn: {
    saidPreviousTurn: "이전 턴에서",
    noTurnWindow: "{name}은(는) 턴의 시작도 끝도 알려 주지 않아서, 구간을 정할 수 없습니다.",
  },
  carry: {
    reused:
      "{name}이(가) 이미 있어서 현재 상태 그대로 체크아웃했습니다. 커밋하지 않은 작업은 옮겨지지 않았습니다.",
    failed: "커밋하지 않은 작업을 옮기지 못했습니다: {error}",
  },
  codemirror: {
    expand: { other: "변경되지 않은 줄 {count}개 펼치기" },
    showLines: "{from}–{to}줄 표시",
    revertTitle: "이 변경 되돌리기",
    revert: "되돌리기",
  },
} satisfies Shape<typeof en>
