import { useEffect, useState } from "react"
import { FileText, Terminal } from "lucide-react"
import { ProjectService } from "@/lib/rpc"
import type { WorktreeSetup } from "@/lib/api-types"
import { Trans } from "@/components/common/Trans"
import { Button } from "@/components/ui/button"
import { Label } from "@/components/ui/label"
import { useT } from "@/lib/i18n/i18n"
import { errorText } from "@/lib/utils"

const setupFile = ".lich/setup-worktree.sh"

// ScriptEditor is the textarea the setup row opens into, with the two variables
// a script can count on named under it.
function ScriptEditor({
  draft,
  onDraft,
  onCancel,
  onSave,
}: {
  draft: string
  onDraft: (value: string) => void
  onCancel: () => void
  onSave: () => void
}) {
  const t = useT()
  const id = "worktree-setup"
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={id} className="text-xs uppercase tracking-wide">
        {t("sidebar.worktreeSetupRow.setupEditorLabel", { file: setupFile })}
      </Label>
      <textarea
        id={id}
        value={draft}
        onChange={(event) => onDraft(event.target.value)}
        rows={3}
        ref={(element) => element?.focus()}
        spellCheck={false}
        placeholder="pnpm install"
        className="w-full resize-none rounded-md border border-input bg-transparent px-2.5 py-1.5 font-mono text-xs outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
      />
      <div className="flex items-center justify-between">
        <span className="font-mono text-xs text-muted-foreground">
          $LICH_WORKTREE_PORT · $LICH_PROJECT_DIR
        </span>
        <div className="flex gap-1">
          <Button variant="ghost" size="xs" onClick={onCancel}>
            {t("sidebar.worktreeSetupRow.discard")}
          </Button>
          <Button variant="ghost" size="xs" onClick={onSave}>
            {t("common.action.save")}
          </Button>
        </div>
      </div>
    </div>
  )
}

// ConfiguredRow is the setup script the repository ships: its first line, and
// the file it lives in.
function ConfiguredRow({ value }: { value: string }) {
  const [first, ...rest] = value.split("\n")
  return (
    <div className="flex min-w-0 items-center gap-2 text-xs">
      <Terminal className="size-3.5 shrink-0 text-muted-foreground" aria-hidden />
      <span className="min-w-0 truncate font-mono">
        {first}
        {rest.length > 0 && " …"}
      </span>
      <span className="ml-auto flex shrink-0 items-center gap-1 text-muted-foreground">
        <FileText className="size-3" aria-hidden />
        <span className="font-mono">{setupFile}</span>
      </span>
    </div>
  )
}

// WorktreeSetupRow is the New-worktree dialog's pre-flight block: the setup
// script a fresh worktree will run, or a lockfile-detected suggestion when the
// repository ships none. Read from the main checkout; Use/Save write the file
// through the backend. It never blocks Create.
export function WorktreeSetupRow({ projectPath }: { projectPath: string }) {
  const t = useT()
  const [info, setInfo] = useState<WorktreeSetup | null>(null)
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState("")
  const [error, setError] = useState("")

  useEffect(() => {
    let stale = false
    // A failed read keeps the block absent: the dialog's job is the branch pick,
    // and a script lich cannot read is a script it will not run either.
    ProjectService.WorktreeSetup(projectPath)
      .then((loaded) => {
        if (!stale) {
          setInfo(loaded)
        }
      })
      .catch(() => {})
    return () => {
      stale = true
    }
  }, [projectPath])

  const save = async (value: string) => {
    setError("")
    try {
      await ProjectService.SaveWorktreeSetup(projectPath, value)
      setEditing(false)
      setInfo(await ProjectService.WorktreeSetup(projectPath))
    } catch (err) {
      setError(errorText(err))
    }
  }

  const edit = (value: string) => {
    setDraft(value)
    setEditing(true)
  }

  if (!info) {
    return null
  }

  if (editing) {
    return (
      <div className="flex flex-col gap-1">
        <ScriptEditor
          draft={draft}
          onDraft={setDraft}
          onCancel={() => setEditing(false)}
          onSave={() => void save(draft)}
        />
        {error && <span className="text-xs break-words text-destructive">{error}</span>}
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-1.5">
      {info.script ? (
        <ConfiguredRow value={info.script} />
      ) : (
        info.suggestion && (
          <div className="flex min-w-0 items-center gap-2 text-xs text-muted-foreground">
            <Terminal className="size-3.5 shrink-0" aria-hidden />
            <span className="min-w-0 truncate">
              <Trans
                k="sidebar.worktreeSetupRow.detected"
                params={{
                  detected: info.detected,
                  command: <span className="font-mono text-foreground">{info.suggestion}</span>,
                }}
              />
            </span>
            <div className="ml-auto flex shrink-0 gap-1">
              <Button variant="ghost" size="xs" onClick={() => void save(info.suggestion ?? "")}>
                {t("sidebar.worktreeSetupRow.use")}
              </Button>
              <Button variant="ghost" size="xs" onClick={() => edit(info.suggestion ?? "")}>
                {t("sidebar.worktreeSetupRow.edit")}
              </Button>
            </div>
          </div>
        )
      )}
      {error && <span className="text-xs break-words text-destructive">{error}</span>}
    </div>
  )
}
