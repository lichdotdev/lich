import type { Shape } from "../../catalog"
import type { common as en } from "../en/common"
export const common = {
  count: {
    file: { other: "파일 {count}개" },
    commit: { other: "커밋 {count}개" },
  },
  time: {
    justNow: "방금",
  },
  action: {
    cancel: "취소",
    save: "저장",
    create: "만들기",
  },
  checkAgainButton: {
    check: "다시 확인",
    checking: "확인 중…",
  },
  errorBoundary: {
    stoppedRendering: "{label} 렌더링이 중단되었습니다",
    reload: "창 새로 고침",
    retry: "다시 시도",
  },
  stepper: {
    default: "기본값",
  },
  toolMissing: {
    notInstalled: "{label}이(가) 설치되어 있지 않습니다",
    install: "{bin} 설치",
  },
  pickerDialog: {
    navigate: "이동",
    filter: "필터",
    close: "닫기",
  },
  ui: {
    dialog: {
      close: "닫기",
    },
  },
} satisfies Shape<typeof en>
