# Git Pull Divergence + Conflict Resolution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When ff-only pull fails on divergent branches, offer Merge/Rebase; when conflicts occur, resolve via Ours/Theirs/editor/Continue/Abort in the Git tab.

**Architecture:** Extend `internal/collectors/git.go` with pull-merge/rebase, in-progress detection, and conflict helpers. Wire UI in `git_messages.go` / `git_prompt.go` / `app.go` using the existing confirm-modal pattern plus a lightweight conflict mode flag on `App`.

**Tech Stack:** Go 1.22+, Bubble Tea, git CLI via `exec.Command`.

## Global Constraints

- Portuguese status strings (match existing Git tab)
- Happy-path MVP: no interactive rebase todo UI
- Default pull remains `--ff-only` first
- Do not treat unmerged `U` as staged for toggle-unstage

---

### Task 1: Collector helpers + tests

**Files:**
- Modify: `internal/collectors/git.go`
- Modify: `internal/collectors/git_test.go`

**Interfaces:**
- Produces:
  - `func GitIsFFImpossible(err error, output string) bool`
  - `func GitIsConflictError(err error, output string) bool`
  - `func GitOpInProgress(path string) string` // "", "merge", "rebase", "cherry-pick"
  - `func GitPullOriginMerge(path, sourceBranch string) (string, error)`
  - `func GitPullOriginRebase(path, sourceBranch string) (string, error)`
  - `func GitCheckoutOurs(path, file string) error`
  - `func GitCheckoutTheirs(path, file string) error`
  - `func GitContinue(path string) (string, error)`
  - `func GitAbort(path string) (string, error)`
  - `func GitFileUnmerged(staging, worktree string) bool`

- [ ] **Step 1: Write failing tests** for `GitIsFFImpossible`, `GitIsConflictError`, `GitFileUnmerged`, and `GitOpInProgress` (temp repo with merge conflict).

- [ ] **Step 2: Implement helpers** in `git.go` using existing `gitRun` / `gitRunOutput` / `gitOutput` patterns:
  - FF: match `not possible to fast-forward` / `diverging branches` (case-insensitive)
  - Conflict: match `conflict` / `fix conflicts` / `needs merge` or `GitOpInProgress != ""` after failed op
  - In-progress: check `MERGE_HEAD`, `CHERRY_PICK_HEAD`, `REBASE_HEAD`, or dirs `rebase-merge`/`rebase-apply` under `.git`
  - Continue: `git -c core.editor=true merge|rebase|cherry-pick --continue`
  - Abort: matching `--abort`
  - Ours/Theirs: `checkout --ours|--theirs -- <file>` then `add -- <file>`

- [ ] **Step 3: Run** `go test ./internal/collectors/ -count=1 -run 'GitIs|GitOp|GitFileUnmerged|GitPullOrigin'`

---

### Task 2: UI pull strategy modal + conflict mode

**Files:**
- Modify: `internal/ui/app.go` (state fields + key routing)
- Modify: `internal/ui/git_prompt.go` (modal render + keys `m`/`r`)
- Modify: `internal/ui/git_messages.go` (pull done → modal; strategy cmds; conflict enter/leave)
- Modify: `internal/ui/helpers.go` (`gitFileStaged`, `gitStatusStyle`, unmerged helper)
- Modify: `internal/ui/git_tab.go` (banner / actions hints when conflict)
- Modify: `internal/ui/git_tab_test.go` / new small tests as needed

**Interfaces:**
- Consumes: collector funcs from Task 1
- App fields: `gitConflictOn bool`, `gitConflictKind string`, reuse `gitConfirmAction == "pull-strategy"` with `gitConfirmBranch` = source

- [ ] **Step 1: On pull error**, if `GitIsFFImpossible`, open confirm action `pull-strategy` (do not only compact-error).
- [ ] **Step 2: Modal keys** `m` → `GitPullOriginMerge`, `r` → `GitPullOriginRebase`, `esc`/`n` cancel.
- [ ] **Step 3: After merge/rebase/cherry-pick/pull-* error**, if conflict/in-progress, set `gitConflictOn` and refresh files.
- [ ] **Step 4: Conflict keys** `o`/`t`/`e`/`a`/`c`/`x` as in spec; `e` uses `tea.ExecProcess` with `$EDITOR` or `vi`.
- [ ] **Step 5: Fix** `gitFileStaged` to exclude unmerged; style `U`/`UU` as warning.
- [ ] **Step 6: Run** `go test ./internal/ui/ ./internal/collectors/ -count=1`

---

### Task 3: Docs touch-up

**Files:**
- Modify: `README.md` (Git keybindings section only if present)

- [ ] Document new keys: after failed ff pull `m`/`r`; conflict `o`/`t`/`e`/`a`/`c`/`x`.
