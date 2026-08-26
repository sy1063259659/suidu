# Clipboard Timeline Search UX Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove disruptive inline expansion and add server-backed time filtering with a responsive, date-grouped clipboard timeline.

**Architecture:** Keep list cards summary-only and route long-content actions to the existing protected detail page. Extend the existing clipboard list filter with RFC3339 `from` and exclusive `to` bounds in both repositories, then group the returned records by local calendar day in the Vue UI. Combine quick date presets, a custom date range, text/type/favorite filters, and a single reset action without changing storage or authentication.

**Tech Stack:** Go 1.24, Gin, PostgreSQL, Vue 3 Composition API, TypeScript, Naive UI, Vitest.

---

## Chunk 1: Stable long-content navigation

### Task 1: Keep list previews compact

**Files:**
- Modify: `frontend/src/components/RichTextContent.vue`
- Test: `frontend/src/utils/textFormat.test.ts`

- [ ] Confirm the current button emits `viewDetail` and the parent already routes to `/items/:id`.
- [ ] Remove any state or behavior that can render full long content inside a list card; keep the clamped preview and make “查看完整内容” a detail-navigation action.
- [ ] Add an arrow affordance and accessible label indicating navigation instead of expansion.
- [ ] Verify clicking the action changes to `/items/:id` without increasing the list document height first.

### Task 2: Move detail metadata and actions above long content

**Files:**
- Modify: `frontend/src/components/ClipboardDetailPage.vue`
- Modify: `frontend/src/style.css`

- [ ] Move the precise creation time into the detail heading area so it is visible before the content.
- [ ] Move favorite, copy/download, organize, share, and delete actions directly below the heading and above the content.
- [ ] Remove the old bottom action footer instead of duplicating controls.
- [ ] Keep destructive styling visually secondary and preserve all existing loading/disabled states and event emissions.
- [ ] On mobile, wrap actions into 44px touch targets without horizontal overflow.

## Chunk 2: Server-backed time filtering

### Task 3: Add date bounds to the clipboard list API

**Files:**
- Modify: `backend/internal/clipboard/model.go`
- Modify: `backend/internal/clipboard/handler.go`
- Modify: `backend/internal/clipboard/memory.go`
- Modify: `backend/internal/clipboard/postgres.go`
- Modify: `backend/internal/clipboard/handler_test.go`
- Modify: `backend/internal/clipboard/repository_test.go`
- Create: `backend/internal/clipboard/postgres_test.go`

- [ ] Add failing handler tests for valid `from`/`to`, invalid RFC3339 values, and `from >= to`.
- [ ] Extend `ListFilter` with `CreatedFrom time.Time` and `CreatedBefore time.Time`.
- [ ] Parse optional RFC3339 query parameters; reject invalid or reversed ranges with HTTP 400.
- [ ] Apply `created_at >= from` and `created_at < to` in PostgreSQL.
- [ ] Apply identical bounds in the memory repository.
- [ ] Add repository tests proving `from` is inclusive, `to` is exclusive, cross-day ordering is preserved, and unrelated users remain isolated.
- [ ] Extract a testable PostgreSQL list-query/argument contract and verify both RFC3339 bounds reach the correct SQL predicates and argument positions without requiring a production database.
- [ ] Run `go test ./...` and confirm all tests pass.

## Chunk 3: Timeline filters and grouped results

### Task 4: Add time-range controls

**Files:**
- Modify: `frontend/src/api/clipboard.ts`
- Create: `frontend/src/utils/timeline.ts`
- Create: `frontend/src/utils/timeline.test.ts`
- Modify: `frontend/src/App.vue`

- [ ] Add failing Vitest cases with an explicit `now` argument for today, recent seven days, custom inclusive calendar ranges converted to exclusive API bounds, and local-day grouping labels; do not depend on the machine timezone or wall clock.
- [ ] Extend `ListClipboardOptions` with ISO `createdFrom` and `createdBefore` parameters.
- [ ] Add quick filters with local-calendar semantics: 今天 is `[today 00:00, tomorrow 00:00)`, 近 7 天 includes today plus the previous six whole days, and 近 30 天 includes today plus the previous 29 whole days.
- [ ] Add a clearable Naive UI date-range picker for custom ranges.
- [ ] Selecting a preset clears the custom picker; selecting a complete custom range sets the time mode to custom; clearing the picker resets time to 全部时间.
- [ ] Make every date change use the existing debounced list request and combine with query, type, and favorite filters.
- [ ] Add one “清除筛选” action when any filter is active; it resets query, type, favorite, preset, and custom range together.

### Task 5: Render a responsive chronological timeline

**Files:**
- Modify: `frontend/src/App.vue`
- Modify: `frontend/src/style.css`

- [ ] Group records into 今天、昨天、or a localized full date.
- [ ] Render a single vertical line with a dot and item count per day; use separate bordered lists inside each day group.
- [ ] Keep all cards in descending time order and show the precise time in card metadata.
- [ ] On mobile, keep one column, 44px controls, horizontally scrollable quick presets, and no decorative alternating layout.
- [ ] Respect `prefers-reduced-motion` and avoid height animations for record lists.

## Chunk 4: Verification and delivery

### Task 6: Verify, review, merge, and deploy

**Files:**
- Verify all modified frontend and backend files.

- [ ] Run `go test ./...`.
- [ ] Run `npm test -- --run`, `npm run build`, and `npm audit --omit=dev`.
- [ ] Verify desktop and 390px layouts have no horizontal overflow.
- [ ] Verify long-content navigation does not expand the list page.
- [ ] Verify detail creation time and all item actions are visible above long content on desktop and mobile.
- [ ] Verify combined query/type/favorite/time filters and custom range requests in the browser.
- [ ] Request code review and resolve blocking findings.
- [ ] Before pushing the public repository, scan the exact branch diff for credentials, domains, IPs, server paths, connection strings, private keys, and deployment metadata; only source-safe changes may proceed.
- [ ] Commit on `feature/timeline-search-ux`, merge with `--no-ff` into `develop`, and push `origin/develop`.
- [ ] Build frontend and Linux API locally, deploy only Suidu API/Web runtime images, and verify health, assets, routes, filters, containers, and logs.
