# Clipboard Detail and Rich Formats Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep clipboard history compact and stable while providing a protected full-detail page with polished rendering for code and common text formats.

**Architecture:** Add an ownership-scoped item lookup endpoint for direct detail URLs. The Vue app keeps its existing single-shell architecture, tracks `/items/:id` with browser history, and delegates format detection/rendering to focused utility and presentation components. History loading becomes a layout-neutral progress rail, while long text is clamped in list previews.

**Tech Stack:** Go, Gin v1, Vue 3 Composition API, TypeScript, Naive UI, Vitest, markdown-it, highlight.js.

---

## Chunk 1: Protected detail data

### Task 1: Add an ownership-scoped item endpoint

**Files:**
- Modify: `backend/internal/clipboard/handler.go`
- Modify: `backend/internal/clipboard/handler_test.go`
- Modify: `frontend/src/api/clipboard.ts`

- [ ] Write a handler test that creates records for two users, expects `GET /api/clipboard/:id` to return the current user's item, and returns 404 for another user's item.
- [ ] Run `go test ./internal/clipboard -run TestHandlerGetsOwnedItem -v` and confirm it fails because the route is absent.
- [ ] Register `GET /clipboard/:id`, parse the ID, call `Repository.Get`, and map `ErrNotFound` to 404.
- [ ] Add `getClipboard(id)` to the frontend API module.
- [ ] Run `go test ./internal/clipboard -run TestHandlerGetsOwnedItem -v` and confirm it passes.

## Chunk 2: Format detection and rendering

### Task 2: Detect common text formats

**Files:**
- Create: `frontend/src/utils/textFormat.ts`
- Create: `frontend/src/utils/textFormat.test.ts`
- Modify: `frontend/package.json`
- Modify: `frontend/package-lock.json`

- [ ] Install Vitest, markdown-it, its TypeScript declarations, and highlight.js through npm so the lockfile records exact versions.
- [ ] Add failing table-driven tests for fenced code, JSON, Markdown, URL, Go, JavaScript/TypeScript, Python, SQL, YAML, and ordinary text.
- [ ] Run `npm test -- --run src/utils/textFormat.test.ts` and confirm failures.
- [ ] Implement deterministic detection with JSON and fenced code taking precedence over heuristic formats.
- [ ] Add `isLongText` based on both Unicode length and line count.
- [ ] Run the format tests and confirm they pass.

### Task 3: Render rich text safely

**Files:**
- Create: `frontend/src/components/RichTextContent.vue`
- Modify: `frontend/src/components/ClipboardItemContent.vue`
- Modify: `frontend/src/style.css`

- [ ] Render JSON and code in a labeled, syntax-highlighted code panel using registered highlight.js languages.
- [ ] Render Markdown with raw HTML disabled and safe link behavior.
- [ ] Render a standalone URL as a readable external-link card.
- [ ] Preserve plain text whitespace and wrapping.
- [ ] Add a `preview` prop to `ClipboardItemContent`; clamp long text previews and emit a detail action without truncating detail/public-share views.
- [ ] Add responsive styles for code overflow, compact previews, touch targets, and mobile detail layout.

## Chunk 3: Stable loading and protected detail page

### Task 4: Replace layout-changing history loading

**Files:**
- Modify: `frontend/src/App.vue`
- Modify: `frontend/src/style.css`

- [ ] Record the current causes: result-height replacement changes document height, the root does not reserve the browser scrollbar gutter, and automatic filter requests also toggle header loading/status elements.
- [ ] Remove the history spinner from normal filter/favorite changes.
- [ ] Add an absolutely positioned progress rail inside the history section so loading never occupies layout space.
- [ ] Keep existing results visible while requests are pending and reserve the page scrollbar gutter.
- [ ] Give the results area a responsive minimum height to avoid an empty-result collapse.
- [ ] Keep per-record favorite controls icon-stable and use only disabled/opacity state during the request.

### Task 5: Add the detail route experience

**Files:**
- Create: `frontend/src/components/ClipboardDetailPage.vue`
- Modify: `frontend/src/App.vue`
- Modify: `frontend/src/style.css`

- [ ] Parse `/items/:id`, load the owned item directly, and support browser back/forward with `popstate`.
- [ ] Open details through `history.pushState` without a full reload.
- [ ] Show content format, creation time, note, tags, copy/download, favorite, organize, share, and delete actions on the detail page.
- [ ] Ensure direct URLs require the existing session and show a clear 404 state for missing/foreign records.
- [ ] Keep list previews compact and provide an obvious “查看完整内容” action for long content.

## Chunk 4: Verification and delivery

### Task 6: Verify, integrate, and deploy

**Files:**
- Modify: `README.md`

- [ ] Document compact previews, private detail URLs, and recognized formats.
- [ ] Run `go test ./...`, `npm test -- --run`, `npm run build`, and `git diff --check` locally.
- [ ] Review the diff for credentials, domains, server paths, and deployment metadata.
- [ ] Commit the feature branch and merge it into `develop` with a merge commit.
- [ ] Re-run tests/build on merged `develop`, push `origin/develop`, and preserve the branch/worktree.
- [ ] Build the Linux API binary and frontend assets locally, back up the previous runtime files, and update only Suidu API/Web containers.
- [ ] Verify health, the protected item endpoint, frontend asset, production route fallback, schema continuity, and recent logs.
