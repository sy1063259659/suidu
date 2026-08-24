# Clipboard Search Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make large clipboard histories easy to find by searching text and file names and filtering by clipboard item type.

**Architecture:** Extend the repository list contract with a typed filter object so PostgreSQL performs filtering across the user's full history before applying the existing result limit. Mirror the same behavior in the in-memory repository for tests and fallback mode. Add a debounced Vue search toolbar that sends `q` and `kind` parameters and safely ignores stale responses.

**Tech Stack:** Go, Gin, PostgreSQL/pgx, Vue 3 Composition API, TypeScript, Naive UI, CSS.

---

## Chunk 1: Backend search contract

### Task 1: Add repository filtering

**Files:**
- Modify: `backend/internal/clipboard/model.go`
- Modify: `backend/internal/clipboard/memory.go`
- Modify: `backend/internal/clipboard/postgres.go`
- Test: `backend/internal/clipboard/repository_test.go`

- [ ] **Step 1: Write failing repository tests**

Add tests proving that keyword search is case-insensitive, matches text content and file names, respects kind and user ownership, preserves newest-first ordering, and applies the limit after filtering.

- [ ] **Step 2: Run the focused tests and verify failure**

Run: `go test ./internal/clipboard -run TestMemoryRepositoryListFilters -count=1`

Expected: FAIL because `ListFilter` and filtered listing do not exist yet.

- [ ] **Step 3: Implement the list filter contract**

Introduce:

```go
type ListFilter struct {
    Limit int
    Query string
    Kind  Kind
}
```

Change `Repository.List` to accept this value. Filter the memory items before sorting/limiting. Use a parameterized PostgreSQL query whose optional conditions search `content` and `file_name` with `ILIKE` and filter `kind`, then order by `created_at DESC, id DESC` and apply the limit.

- [ ] **Step 4: Run the clipboard package tests**

Run: `go test ./internal/clipboard -count=1`

Expected: PASS.

- [ ] **Step 5: Commit backend repository changes**

Run: `git add backend/internal/clipboard && git commit -m "feat: filter clipboard history"`

### Task 2: Expose validated search parameters

**Files:**
- Modify: `backend/internal/clipboard/handler.go`
- Test: `backend/internal/clipboard/handler_test.go`

- [ ] **Step 1: Write failing handler tests**

Add request tests for `GET /api/clipboard?q=...&kind=...`, invalid kinds, and overlong queries.

- [ ] **Step 2: Run the focused tests and verify failure**

Run: `go test ./internal/clipboard -run 'TestHandlerList(Search|Rejects)' -count=1`

Expected: FAIL because query parameters are not handled.

- [ ] **Step 3: Parse and validate query parameters**

Trim `q`, limit it to 200 Unicode characters, accept only `text`, `image`, or `file` for `kind`, preserve the existing limit validation, and pass a `ListFilter` to the repository.

- [ ] **Step 4: Run all backend tests and vet**

Run: `go test -count=1 ./... && go vet ./...`

Expected: PASS.

- [ ] **Step 5: Commit handler changes**

Run: `git add backend/internal/clipboard && git commit -m "feat: expose clipboard search filters"`

## Chunk 2: Search interface

### Task 3: Add a debounced responsive search toolbar

**Files:**
- Modify: `frontend/src/api/clipboard.ts`
- Modify: `frontend/src/App.vue`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Extend the API client**

Add a typed options object containing `limit`, `query`, and `kind`, and serialize it to the existing list endpoint as `limit`, `q`, and `kind`.

- [ ] **Step 2: Add reactive search state**

Add a search query, type filter, a 300 ms debounce, and a monotonically increasing request sequence so a slower old response cannot overwrite a newer result. Clear timers on unmount.

- [ ] **Step 3: Build the toolbar and result states**

Place a clearable search field above history and buttons for all/text/image/file. Show “no matching records” when filters produce no results, and keep ordinary “no clipboard records” for the unfiltered empty state.

- [ ] **Step 4: Make the controls responsive**

Use a flexible toolbar on desktop and a stacked layout with horizontally scrollable filter controls on narrow screens. Keep touch targets usable and avoid fixed page widths.

- [ ] **Step 5: Run the production build**

Run: `npm run build`

Expected: Vue type-check and Vite build both pass.

- [ ] **Step 6: Commit frontend changes**

Run: `git add frontend && git commit -m "feat: add clipboard search interface"`

## Chunk 3: Documentation, integration, and release

### Task 4: Document and verify the feature

**Files:**
- Modify: `README.md`
- Add: `docs/superpowers/plans/2026-08-24-clipboard-search.md`

- [ ] **Step 1: Document search behavior**

Describe case-insensitive text/file-name search, item type filtering, and the fact that filtering happens before the result limit.

- [ ] **Step 2: Run fresh full verification**

Run backend tests/vet, frontend build, `git diff --check`, and inspect `git status`.

- [ ] **Step 3: Commit documentation**

Run: `git add README.md docs/superpowers/plans/2026-08-24-clipboard-search.md && git commit -m "docs: describe clipboard search"`

- [ ] **Step 4: Merge and deploy**

Merge `feature/clipboard-search` into `develop` with a merge commit, rerun verification on the merged result, push `origin/develop`, build artifacts locally, deploy only the runtime artifacts, and verify the production health endpoint and new frontend asset.
