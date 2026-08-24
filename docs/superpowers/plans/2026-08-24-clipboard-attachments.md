# Clipboard Attachments Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend Suidu clipboard records to support text, safe image previews, and arbitrary file uploads/downloads while preserving per-user isolation.

**Architecture:** PostgreSQL remains the source of truth for clipboard metadata; binary content is stored behind a file-storage adapter. Development uses an in-memory adapter, while Docker deployment mounts the SFTPGo-backed `suidu-files` volume into the API under a dedicated `suidu` directory. Authenticated APIs always load metadata by both user ID and item ID before serving or deleting content.

**Tech Stack:** Go 1.26, Gin v1, PostgreSQL/pgx, Docker Compose, Vue 3 Composition API, TypeScript, Naive UI, Axios.

---

## Chunk 1: Backend metadata and storage

### Task 1: Add the file-storage boundary

**Files:**
- Create: `backend/internal/filestore/store.go`
- Create: `backend/internal/filestore/memory.go`
- Create: `backend/internal/filestore/local.go`
- Test: `backend/internal/filestore/store_test.go`

- [ ] Define a `Store` interface for bounded writes, seekable reads, and deletes.
- [ ] Implement collision-resistant, owner-scoped keys without trusting uploaded names.
- [ ] Implement an in-memory store for local development and handler tests.
- [ ] Implement a local filesystem store that rejects escaping paths and writes files with private permissions.
- [ ] Test size limits, round trips, and invalid keys.

### Task 2: Extend clipboard metadata

**Files:**
- Modify: `backend/internal/clipboard/model.go`
- Modify: `backend/internal/clipboard/repository.go`
- Modify: `backend/internal/clipboard/memory.go`
- Modify: `backend/internal/clipboard/postgres.go`
- Modify: `backend/internal/clipboard/schema.sql`
- Test: `backend/internal/clipboard/repository_test.go`

- [ ] Add `text`, `image`, and `file` item kinds plus attachment metadata.
- [ ] Preserve existing text rows through additive PostgreSQL migrations.
- [ ] Add create-attachment and owner-scoped get operations.
- [ ] Verify user isolation and attachment metadata ordering in memory repository tests.

### Task 3: Add upload and content APIs

**Files:**
- Modify: `backend/internal/clipboard/handler.go`
- Modify: `backend/internal/clipboard/handler_test.go`
- Modify: `backend/cmd/server/main.go`

- [ ] Add `POST /api/clipboard/files` for one multipart file up to 100 MiB.
- [ ] Detect safe browser image types server-side; treat SVG and unknown content as downloads.
- [ ] Add `GET /api/clipboard/:id/content` with owner checks and safe content-disposition headers.
- [ ] Delete stored content when its clipboard record is deleted.
- [ ] Test text compatibility, upload, image classification, download, deletion, size rejection, and cross-user denial.

## Chunk 2: Frontend experience and deployment

### Task 4: Add attachment client APIs and UI

**Files:**
- Modify: `frontend/src/api/clipboard.ts`
- Create: `frontend/src/components/ClipboardItemContent.vue`
- Modify: `frontend/src/App.vue`
- Modify: `frontend/src/style.css`

- [ ] Add typed attachment fields and upload/download helpers.
- [ ] Add a multi-file drag/drop and file-picker area with progress/error feedback.
- [ ] Render bounded image previews and generic file cards.
- [ ] Keep copy actions text-only and expose authenticated download links for attachments.
- [ ] Format file sizes and retain the existing delete flow.

### Task 5: Wire the shared production volume

**Files:**
- Modify: `backend/Dockerfile.runtime`
- Modify: `docker-compose.yml`
- Modify: `.env.example`
- Modify: `frontend/nginx.conf`
- Modify: `README.md`
- Modify: `docs/architecture.md`

- [ ] Mount the existing `suidu-files` volume into the API.
- [ ] Initialize a dedicated writable directory shared with SFTPGo without exposing host paths.
- [ ] Configure the API storage root and increase the reverse-proxy upload limit.
- [ ] Document limits, persistence, migration, backup, and ownership behavior.

### Task 6: Verify and deliver through GitFlow

- [ ] Run `gofmt` on all changed Go files.
- [ ] Run `go test ./...` and `go vet ./...` under `backend`.
- [ ] Run `npm run build` under `frontend`.
- [ ] Exercise local authenticated upload/download/delete flows.
- [ ] Commit the feature branch, merge it into `develop`, and push both branches.
- [ ] Build binaries and frontend assets locally, deploy runtime-only images, and verify public health/upload behavior.
