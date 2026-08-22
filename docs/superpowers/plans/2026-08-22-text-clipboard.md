# Text Clipboard Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a text clipboard loop that lets a user paste or read browser clipboard text, persist it, browse recent entries, copy an entry, and delete it.

**Architecture:** The Vue client calls a Gin REST API. The API uses a repository interface with a PostgreSQL implementation for deployment and an in-memory implementation for local development without configured infrastructure. Clipboard records are stored as durable text rows; browser clipboard access remains an explicit user action.

**Tech Stack:** Vue 3, TypeScript, Naive UI, Axios, Lucide Vue; Go, Gin v1, pgx/v5, PostgreSQL.

---

### Task 1: Backend clipboard persistence

**Files:**
- Create: `backend/internal/clipboard/model.go`
- Create: `backend/internal/clipboard/repository.go`
- Create: `backend/internal/clipboard/memory.go`
- Create: `backend/internal/clipboard/postgres.go`
- Create: `backend/internal/clipboard/schema.sql`
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/go.mod`
- Test: `backend/internal/clipboard/repository_test.go`

- [ ] Define the clipboard item model and repository contract.
- [ ] Add memory repository behavior for development and deterministic tests.
- [ ] Add PostgreSQL repository with schema initialization and ordered recent-item queries.
- [ ] Add tests for create, list ordering, empty content rejection, and delete.

### Task 2: Backend HTTP API

**Files:**
- Create: `backend/internal/clipboard/handler.go`
- Create: `backend/internal/clipboard/handler_test.go`
- Modify: `backend/cmd/server/main.go`

- [ ] Add `GET /api/clipboard` with a bounded `limit` query.
- [ ] Add `POST /api/clipboard` with JSON validation.
- [ ] Add `DELETE /api/clipboard/:id` with not-found handling.
- [ ] Add handler tests for success and invalid requests.

### Task 3: Clipboard web experience

**Files:**
- Create: `frontend/src/api/clipboard.ts`
- Modify: `frontend/package.json`
- Modify: `frontend/src/App.vue`
- Modify: `frontend/src/style.css`

- [ ] Add typed API functions for list, create, and delete.
- [ ] Add textarea entry, browser clipboard read, submit, refresh, copy, and delete actions.
- [ ] Show loading, empty, error, and clipboard-permission states.
- [ ] Keep the layout responsive and use clear icon+text actions.

### Task 4: Verification and GitFlow integration

**Files:**
- Modify: `README.md`

- [ ] Document the clipboard endpoints and `SUIDU_DATABASE_URL` development behavior.
- [ ] Run `go test ./...`.
- [ ] Run `npm run build` and `npm audit --omit=dev`.
- [ ] Commit on `feature/text-clipboard`, merge into `develop`, and push.

