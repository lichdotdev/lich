import type { Shape } from "../../catalog"
import type { tabs as en } from "../en/tabs"
export const tabs = {
  homeTab: {
    home: "홈",
  },
  notificationsButton: {
    title: "알림",
    titlePending: "알림, 대기 중 {count}개",
    caughtUp: "새 알림이 없습니다",
    dismiss: "{name} 닫기",
  },
  openProjectMenu: {
    open: "프로젝트 열기",
    recent: "최근 프로젝트",
    relocate: "위치 변경",
    openFolder: "폴더 열기…",
    moreClosed: {
      other: "닫힌 프로젝트 {count}개 더 있음, 팔레트에서 검색하세요",
    },
  },
  projectTab: {
    close: "{name} 닫기",
  },
  projectTabs: {
    pullRequests: "풀 리퀘스트",
    settings: "설정",
    closeTitle: "세션이 아직 실행 중입니다",
    closeBody: {
      other:
        "{name}을(를) 닫으면 턴 진행 중인 세션 {count}개가 중단됩니다. 프로젝트를 다시 열면 각 세션이 멈춘 곳에서 재개할 수 있지만, 진행 중이던 턴은 사라집니다.",
    },
    closeAnyway: "그래도 닫기",
  },
} satisfies Shape<typeof en>
