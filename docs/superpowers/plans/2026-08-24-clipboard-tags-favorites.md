# Clipboard Tags and Favorites Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let users organize clipboard records with searchable private tags and quickly retrieve important records with favorites.

**Architecture:** Store normalized tags and a favorite flag on each clipboard record, update them through one ownership-scoped metadata endpoint, and extend existing backend search/filtering. The Vue list will expose a favorite toggle, tag editor, tag chips, and a favorites-only filter; public share responses will explicitly strip private organization metadata.

**Tech Stack:** Go, Gin, PostgreSQL/pgx, Vue 3 Composition API, TypeScript, Naive UI, CSS.

---

## Chunk 1: Metadata model and persistence

### Task 1: Add tags and favorites to repositories

**Files:**
- Modify: `backend/internal/clipboard/schema.sql`
- Modify: `backend/internal/clipboard/model.go`
- Modify: `backend/internal/clipboard/repository.go`
- Modify: `backend/internal/clipboard/memory.go`
- Modify: `backend/internal/clipboard/postgres.go`
- Test: `backend/internal/clipboard/repository_test.go`

- [ ] Write failing tests for tag normalization, metadata ownership, tag search, and favorites-only filtering.
- [ ] Run `go test ./internal/clipboard -run 'TestMemoryRepository(Metadata|ListFilters)' -count=1` and confirm failure.
- [ ] Add `tags TEXT[]` and `favorite BOOLEAN` migrations, model fields, validation limits, `UpdateMetadata`, tag search, and favorite filtering.
- [ ] Run `go test ./internal/clipboard -count=1` and confirm pass.
- [ ] Commit with `git commit -m "feat: store clipboard tags and favorites"`.

### Task 2: Add the private metadata API

**Files:**
- Modify: `backend/internal/clipboard/handler.go`
- Test: `backend/internal/clipboard/handler_test.go`

- [ ] Write failing tests for `PATCH /api/clipboard/:id`, invalid tags, ownership isolation, favorites filtering, and public-share privacy.
- [ ] Run focused handler tests and confirm failure.
- [ ] Add the PATCH route, validated update handler, `favorite=true` list parameter, and sanitize tags/favorite from public share responses.
- [ ] Run `go test -count=1 ./... && go vet ./...` and confirm pass.
- [ ] Commit with `git commit -m "feat: expose private clipboard organization"`.

## Chunk 2: Vue organization controls

### Task 3: Add favorite and tag controls

**Files:**
- Modify: `frontend/src/api/clipboard.ts`
- Modify: `frontend/src/App.vue`
- Modify: `frontend/src/style.css`

- [ ] Extend API types and add `updateClipboardMetadata` plus the favorites list parameter.
- [ ] Add a star action that updates favorite state and removes unstarred items from favorites-only results.
- [ ] Add a tag-edit modal using Naive UI dynamic tags, enforce client limits, and render tag chips on records.
- [ ] Add a favorites-only filter and include tags in the search placeholder and active-filter behavior.
- [ ] Make tags, actions, filters, and the modal responsive at narrow widths.
- [ ] Run `npm run build` and confirm pass.
- [ ] Commit with `git commit -m "feat: organize clipboard records"`.

## Chunk 3: Verification and release

### Task 4: Document, integrate, and deploy

**Files:**
- Modify: `README.md`
- Add: `docs/superpowers/plans/2026-08-24-clipboard-tags-favorites.md`

- [ ] Document tag limits, search behavior, favorites filtering, and public-share privacy.
- [ ] Run fresh backend tests/vet, frontend production build, `git diff --check`, and status inspection.
- [ ] Commit documentation.
- [ ] Merge into `develop`, repeat verification on the merge result, and push `origin/develop`.
- [ ] Build artifacts locally, deploy only runtime artifacts, then verify containers, health, asset hash, and protected endpoints.
