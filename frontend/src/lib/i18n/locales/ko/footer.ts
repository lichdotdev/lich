import type { Shape } from "../../catalog"
import type { footer as en } from "../en/footer"
export const footer = {
  item: {
    attach: { label: "파일 첨부", example: "파일 첨부" },
    files: { label: "파일 탐색기", example: "파일" },
    changes: { label: "변경 사항" },
    pr: { label: "풀 리퀘스트" },
    checkout: { label: "브랜치" },
    path: { label: "작업 디렉터리" },
    model: { label: "모델", example: "모델" },
    context: { label: "컨텍스트 창" },
    plan: { label: "플랜 사용량" },
    cost: { label: "비용" },
    handsOn: { label: "직접 작업 시간" },
    clock: { label: "날짜 및 시간" },
  },
} satisfies Shape<typeof en>
