# Git Pull Divergence + Conflict Resolution

## Problem

`p` (pull) always runs `git pull origin <source> --ff-only`. When the current branch and the source have diverged, Git aborts with “Not possible to fast-forward” and DevScope only shows a compacted error — no path forward.

Merge (`M`) and cherry-pick can also leave the repo in a conflicted state with no Continue/Abort/Ours/Theirs flow.

## Goals

1. When ff-only pull fails due to divergence, ask the user: **Merge**, **Rebase**, or cancel.
2. When merge/rebase/cherry-pick leaves conflicts, enter a conflict mode: list conflicted files, **Ours/Theirs**, open `$EDITOR`, **Continue**, **Abort**.
3. Keep LazyGit (`L`) as escape hatch.

## Non-goals

- Full in-TUI 3-way merge editor / hunk picking
- Interactive rebase todo editing
- Changing default pull success path (still `--ff-only` first)

## Flow

```
p → git pull origin <src> --ff-only
      ├─ ok → refresh (unchanged)
      └─ not possible to ff-forward
            → modal: [m] Merge  [r] Rebase  [esc] Cancel
                 ├─ m → git pull origin <src> --no-ff
                 └─ r → git pull --rebase origin <src>
                       ├─ ok → refresh
                       └─ conflicts → conflict mode
```

Conflict mode also activates after `M` (merge) or cherry-pick (`V`) when Git leaves an in-progress operation with unmerged paths.

## Conflict mode

- Detect in-progress: `MERGE_HEAD` / `REBASE_HEAD` (or rebase-apply/rebase-merge) / `CHERRY_PICK_HEAD`
- Conflicted files: porcelain unmerged codes (`UU`, `AA`, `DD`, `AU`, `UA`, `DU`, `UD`, `AA`, etc.)
- Keys (Git tab, conflict mode):
  - `o` — checkout --ours + stage file under cursor
  - `t` — checkout --theirs + stage file under cursor
  - `e` — open file in `$EDITOR` (fallback `vi`) via `tea.ExecProcess`
  - `a` — `git add` file (mark resolved after manual edit)
  - `c` — continue (`merge --continue` / `rebase --continue` / `cherry-pick --continue`); if commit message needed, reuse existing compose modal or `GIT_EDITOR=true` / `-m` only when git requires it — prefer `git -c core.editor=true` for continue when message already exists
  - `x` — abort (`merge --abort` / `rebase --abort` / `cherry-pick --abort`)
- Banner in status / Actions: `CONFLITO (merge|rebase|cherry-pick)  o/t/e/a  c continue  x abort`
- Style unmerged files distinctly (`U` / `UU` as warning)
- Fix `gitFileStaged`: do not treat unmerged `U` as “staged for unstage”

## Collector API (new)

| Func | Behavior |
|------|----------|
| `GitPullOriginMerge` | `pull origin <b> --no-ff` |
| `GitPullOriginRebase` | `pull --rebase origin <b>` |
| `GitOpInProgress` | `""` / `"merge"` / `"rebase"` / `"cherry-pick"` |
| `GitIsConflictError` | true if output/err indicates conflicts |
| `GitIsFFImpossible` | true if ff-only abort |
| `GitCheckoutOurs` / `GitCheckoutTheirs` | checkout + `git add` |
| `GitContinue` / `GitAbort` | dispatch by in-progress kind |

## UI touchpoints

- `git_messages.go` — pull result → open strategy modal; action done → enter/leave conflict mode
- `git_prompt.go` — confirm modal variant for pull strategy (`m`/`r`)
- `app.go` — key routing when conflict mode / strategy modal
- `helpers.go` — staged/unmerged helpers + styles
- `git_tab.go` — banner / help hints
- Tests for FF detection, conflict detection, staged-unmerged fix

## Success criteria

- Diverged pull no longer dead-ends: user can merge or rebase from the TUI
- Conflicts from pull-merge, pull-rebase, merge, or cherry-pick are resolvable without leaving DevScope (or via `e` / `L`)
- Clean ff-only pulls unchanged
