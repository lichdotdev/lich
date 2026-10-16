import { useEffect, useRef } from "react"
import {
  invalidatePullRequest,
  lookupPullRequest,
  onPullRequestInvalidated,
} from "./pull-request-lookup"
import type { PullRequest } from "@/lib/api-types"
import type { SessionStatus } from "@/lib/session/session-events"
import { useRemoteResource } from "@/lib/use-remote-resource"

export type { PullRequest }

const UNANSWERED: PullRequest | null | undefined = undefined

// usePullRequest resolves the open GitHub PR for a path's current branch via the
// gh CLI. Unlike git status it is not polled — each lookup is a network
// round-trip — but it refetches whenever the checkout's HEAD moves, so a merge
// that closed one reaches the badge without waiting for a window focus, and when
// a turn ends there with no PR shown (useRecheckAfterTurn), for one the session
// just opened. It also refetches on focus, for a PR opened or
// merged in the browser, and on invalidatePullRequests, for a merge landed from
// the Pulls screen — which moves no HEAD at all. Callers asking about the same
// checkout share one gh call (pull-request-lookup). Returns undefined until the
// checkout's branch has been answered, and null on any error or when the branch
// has no open PR (a merged or closed one is filtered server-side). The two stay
// apart because "no PR" is acted on (the card offers to open one), and an offer
// drawn while gh is still looking flashes on a branch that has its PR.
export function usePullRequest(
  path: string,
  branch: string,
  head: string,
): PullRequest | null | undefined {
  const { data, refresh } = useRemoteResource(
    path && `${path} ${branch} ${head}`,
    () => lookupPullRequest(path, branch, head),
    {
      empty: UNANSWERED,
      refetchOnFocus: true,
      // A different checkout is a different PR: drop the old badge at once
      // rather than showing it against the new branch. A new commit on the same
      // branch keeps it, so the badge does not blink on every commit.
      resetOn: `${path} ${branch}`,
    },
  )

  useEffect(
    () =>
      onPullRequestInvalidated((changed) => {
        if (changed === null || changed === path) {
          refresh()
        }
      }),
    [refresh, path],
  )

  return data
}

// useRecheckAfterTurn asks gh again for path's pull request when a session's
// turn ends there with none shown: `gh pr create` moves neither branch nor
// HEAD, so nothing else would bring up the PR the turn opened.
export function useRecheckAfterTurn(
  path: string,
  status: SessionStatus | null,
  pr: PullRequest | null | undefined,
): void {
  const previous = useRef(status)
  useEffect(() => {
    const turnEnded = status === "done" && previous.current !== "done"
    previous.current = status
    if (turnEnded && pr === null) {
      invalidatePullRequest(path)
    }
  }, [path, status, pr])
}
