import type { Shape } from "../../catalog"
import type { prompts as en } from "../en/prompts"
export const prompts = {
  pullRequest: {
    conflicts:
      "풀 리퀘스트 #{number}({branch})에 {base}와(과)의 병합 충돌이 있습니다. 충돌을 해결하세요.",
    checksUnnamed: {
      other:
        "풀 리퀘스트 #{number}({branch})의 CI가 실패하고 있습니다. 실패한 체크가 {count}개 있으니, 어떤 체크인지 찾아서 고치세요.",
    },
    checksListed:
      "풀 리퀘스트 #{number}({branch})의 CI가 실패하고 있습니다. 다음 체크를 고치세요:\n\n{checks}",
    checksOmitted: {
      other: "\n\n…그리고 목록에 없는 실패한 체크가 {count}개 더 있습니다.",
    },
    create:
      "브랜치 {branch}에는 아직 풀 리퀘스트가 없습니다. `gh pr create`로 하나 여세요. 브랜치가 원격에 없으면 먼저 푸시하고, 커밋과 기본 브랜치 대비 diff를 바탕으로 제목과 본문을 쓰고, 저장소에 풀 리퀘스트 템플릿이 있으면 그것을 따르세요.",
  },
  delegate: {
    session: '"{target}" 세션에 위임: ',
    newWorktree: "새 worktree 세션에 위임: ",
    newWorktreeWithCommand:
      "새 worktree 세션에 위임(`lich open --worktree <branch> --prompt <task>` 실행): ",
  },
  reviewComments: {
    header: "리뷰 코멘트:",
  },
  issue: {
    head: "GitHub 이슈 #{number}: {title}\n{url}",
  },
} satisfies Shape<typeof en>
