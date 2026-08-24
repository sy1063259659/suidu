# Paste Images and Safe Attachment Delete Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let signed-in users paste screenshots directly into the clipboard page and guarantee that an attachment deletion reports success only after its stored source file is removed.

**Architecture:** The Vue page listens for paste events only while the clipboard view is active, extracts clipboard files, and sends them through the same upload/progress function used by the drag-and-drop control. The Go delete handler removes the storage object before metadata; storage errors keep metadata intact so the operation can be retried, while an already-missing object is treated as successfully removed.

**Tech Stack:** Go, Gin, in-memory file store tests, Vue 3 Composition API, TypeScript, Naive UI, Axios.

---

## Chunk 1: Reliable attachment deletion

### Task 1: Make source-file deletion mandatory

**Files:**
- Modify: `backend/internal/clipboard/handler.go`
- Modify: `backend/internal/clipboard/handler_test.go`

- [ ] **Step 1: Add a failing handler test**

Create a file-store test double whose `Delete` returns an error. Upload/create an attachment, call `DELETE /api/clipboard/:id`, assert HTTP 500, and assert the repository record still exists.

- [ ] **Step 2: Run focused test and confirm failure**

Run: `go test ./internal/clipboard -run TestHandlerKeepsAttachmentWhenSourceDeleteFails`

Expected: FAIL because the current handler deletes metadata and returns 204.

- [ ] **Step 3: Change delete ordering**

Delete `item.StorageKey` first. Continue for `filestore.ErrNotFound`, return HTTP 500 for any other file-store error, and only then delete repository metadata. This makes retries converge if metadata deletion later fails.

- [ ] **Step 4: Verify backend**

Run: `gofmt -w internal/clipboard/*.go && go test ./... && go vet ./...`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend
git commit -m "fix: require attachment source deletion"
```

## Chunk 2: Paste screenshots to upload

### Task 2: Reuse upload flow for pasted files

**Files:**
- Modify: `frontend/src/App.vue`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Extract a shared upload function**

Move size validation, progress updates, API upload, result insertion, and errors into `submitFileUpload(file, uploadId, callbacks?)`. Keep Naive UI's custom request adapter thin.

- [ ] **Step 2: Add clipboard paste handling**

Register a document `paste` listener in `onMounted` and remove it in `onUnmounted`. When authenticated and on the clipboard view, collect `clipboardData.files`; if files exist, prevent default and upload each one. Do nothing when the clipboard contains text only.

- [ ] **Step 3: Add visible guidance and feedback**

Update the upload hint to mention `Ctrl+V`/`⌘V` screenshot paste. Existing aggregate progress and error controls report pasted uploads as well.

- [ ] **Step 4: Verify frontend**

Run: `npm run build`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend
git commit -m "feat: paste clipboard images to upload"
```

## Chunk 3: Completion

### Task 3: Document, verify, integrate, and deploy

**Files:**
- Modify: `README.md`
- Add: `docs/superpowers/plans/2026-08-24-paste-images-safe-delete.md`

- [ ] **Step 1: Document paste and delete semantics**

State that screenshot paste uploads immediately and deleting an attachment removes its stored object and cascading share records.

- [ ] **Step 2: Run fresh full verification**

Run backend tests/vet, frontend production build, `git diff --check`, and confirm a clean feature branch.

- [ ] **Step 3: Commit documentation**

```bash
git add README.md docs/superpowers/plans/2026-08-24-paste-images-safe-delete.md
git commit -m "docs: describe clipboard paste uploads"
```

- [ ] **Step 4: Integrate automatically**

Because the user explicitly set a standing workflow, merge the verified feature branch into `develop`, push `origin/develop`, build artifacts locally, deploy runtime images, and verify the public service. Preserve branch and worktree; do not delete files.
