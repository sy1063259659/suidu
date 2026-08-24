# Public Clipboard Shares Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow a signed-in user to publish one clipboard item through an expiring public link, inspect all links they created, and revoke any link immediately.

**Architecture:** Add a `clipboard_shares` persistence model that references an owned clipboard item and uses a 256-bit URL-safe random token. Authenticated routes create, list, and revoke shares; unauthenticated routes resolve only active, unexpired shares and stream attachment content through the existing file store. Vue keeps the authenticated application in `App.vue` while a focused public component handles `/s/:token` without adding a router dependency.

**Tech Stack:** Go 1.25, Gin, PostgreSQL/pgx, in-memory repository tests, Vue 3 Composition API with TypeScript, Naive UI, Axios.

---

## Chunk 1: Backend share domain and security

### Task 1: Define share model and repository contract

**Files:**
- Modify: `backend/internal/clipboard/model.go`
- Create: `backend/internal/clipboard/share.go`
- Test: `backend/internal/clipboard/repository_test.go`

- [ ] **Step 1: Write failing repository tests**

Cover creating a share for an owned text item, listing by owner, resolving by token, hiding another user's item, rejecting expiry outside 5 minutes to 365 days, and making revoked/expired tokens resolve as `ErrNotFound`.

- [ ] **Step 2: Run the focused tests and confirm failure**

Run: `go test ./internal/clipboard -run 'TestMemoryRepository.*Share'`

Expected: FAIL because share methods do not exist.

- [ ] **Step 3: Add domain types and validation**

Add:

```go
type Share struct {
    ID int64
    UserID int64
    Token string
    Item Item
    ExpiresAt time.Time
    RevokedAt *time.Time
    CreatedAt time.Time
}

type CreateShareInput struct {
    ItemID int64
    Token string
    ExpiresAt time.Time
}
```

Extend `Repository` with `CreateShare`, `ListShares`, `GetPublicShare`, and `RevokeShare`. Validate token presence, owned item, future expiry, and maximum lifetime.

- [ ] **Step 4: Implement in-memory behavior**

Store shares under the repository lock, resolve only when `RevokedAt == nil && ExpiresAt.After(now)`, and keep revoked or expired rows visible to the authenticated owner list.

- [ ] **Step 5: Run focused tests**

Run: `go test ./internal/clipboard -run 'TestMemoryRepository.*Share'`

Expected: PASS.

### Task 2: Add PostgreSQL persistence

**Files:**
- Modify: `backend/internal/clipboard/schema.sql`
- Modify: `backend/internal/clipboard/postgres.go`

- [ ] **Step 1: Add additive schema migration**

Create `clipboard_shares` with `BIGSERIAL` id, owner id, clipboard item foreign key with `ON DELETE CASCADE`, unique token, expiry, nullable revocation, and creation timestamp. Add owner/date and token lookup indexes.

- [ ] **Step 2: Implement PostgreSQL share methods**

Use ownership checks in SQL. Join `clipboard_items` for list/public results. Public lookup must enforce both `revoked_at IS NULL` and `expires_at > NOW()` in the query.

- [ ] **Step 3: Run backend tests and vet**

Run: `go test ./... && go vet ./...`

Expected: PASS.

- [ ] **Step 4: Commit backend domain**

```bash
git add backend/internal/clipboard
git commit -m "feat: add expiring clipboard shares"
```

### Task 3: Add authenticated and public HTTP routes

**Files:**
- Modify: `backend/internal/clipboard/handler.go`
- Modify: `backend/internal/clipboard/handler_test.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Write failing handler tests**

Test:

```text
POST /api/clipboard/:id/shares       201 and a 43-character token
GET  /api/shares                     owner-only list
POST /api/shares/:id/revoke          204 and immediate invalidation
GET  /api/public/shares/:token       200 for active, 404 for expired/revoked
GET  /api/public/shares/:token/content streams an active attachment
```

Also test invalid durations, other-user item IDs, malformed tokens, and `Cache-Control: no-store`.

- [ ] **Step 2: Run focused tests and confirm failure**

Run: `go test ./internal/clipboard -run 'TestHandler.*Share'`

Expected: FAIL because routes are absent.

- [ ] **Step 3: Implement secure token generation and handlers**

Generate 32 random bytes with `crypto/rand`, encode with `base64.RawURLEncoding`, accept `expiresInSeconds` in `[300, 31536000]`, and return the token only as share data. Never accept an owner ID from the client.

- [ ] **Step 4: Register routes at correct trust boundaries**

Register public resolution routes on `/api` before authentication middleware and authenticated management routes on the protected router group.

- [ ] **Step 5: Run backend verification**

Run: `gofmt -w internal/clipboard/*.go cmd/server/*.go && go test ./... && go vet ./...`

Expected: PASS.

- [ ] **Step 6: Commit HTTP layer**

```bash
git add backend
git commit -m "feat: expose public clipboard share API"
```

## Chunk 2: Vue share experience

### Task 4: Add typed share API and public page

**Files:**
- Create: `frontend/src/api/shares.ts`
- Create: `frontend/src/components/PublicSharePage.vue`
- Modify: `frontend/src/components/ClipboardItemContent.vue`

- [ ] **Step 1: Add API types and methods**

Provide `ClipboardShare`, `PublicClipboardShare`, create/list/revoke methods, `publicShareContentUrl`, and a helper that builds `/s/:token` using `window.location.origin`.

- [ ] **Step 2: Make clipboard content rendering URL-injectable**

Add an optional `contentUrl` prop/function so the same safe text/image/file presentation works for authenticated and public views.

- [ ] **Step 3: Build the public view**

Read token from the path supplied by the parent, load the public API, show active item/expiry, copy text, preview safe images, and expose a download button. Render a concise unavailable state for missing, expired, or revoked links.

- [ ] **Step 4: Run frontend build**

Run: `npm run build`

Expected: PASS.

### Task 5: Add share creation and management UI

**Files:**
- Modify: `frontend/src/App.vue`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Route public paths before authentication UI**

When `window.location.pathname` matches `/s/<token>`, render `PublicSharePage` and skip `loadSession`.

- [ ] **Step 2: Add per-item share creation**

Add a share action beside copy/download. Open a Naive UI modal with expiry presets `1 小时`, `1 天`, `7 天`, `30 天`; after creation show a copyable absolute URL.

- [ ] **Step 3: Add share management view**

Add a header entry available to all signed-in users. List links with item summary, created/expiry times, and status tags for active, expired, and revoked. Active rows support copy/open and a confirmed revoke action.

- [ ] **Step 4: Add responsive styles**

Keep the current visual system, give the public page a focused centered layout, and stack share actions on narrow screens.

- [ ] **Step 5: Run frontend verification**

Run: `npm run build`

Expected: PASS.

- [ ] **Step 6: Commit frontend**

```bash
git add frontend
git commit -m "feat: add clipboard share experience"
```

## Chunk 3: Documentation and completion

### Task 6: Document and verify the complete feature

**Files:**
- Modify: `README.md`
- Modify: `docs/architecture.md`

- [ ] **Step 1: Document public sharing behavior**

Describe routes at a high level, expiry/revocation semantics, bearer-link privacy, and the 365-day maximum.

- [ ] **Step 2: Run all verification commands**

Run:

```bash
cd backend && go test ./... && go vet ./...
cd ../frontend && npm run build
git diff --check
git status --short
```

Expected: all commands pass; only intentional files are changed.

- [ ] **Step 3: Commit documentation**

```bash
git add README.md docs/architecture.md docs/superpowers/plans/2026-08-24-public-clipboard-shares.md
git commit -m "docs: describe public clipboard sharing"
```

- [ ] **Step 4: Complete the development branch**

Use `superpowers:finishing-a-development-branch`, verify again, and present integration options. Do not remove the worktree or branch without explicit user approval.
