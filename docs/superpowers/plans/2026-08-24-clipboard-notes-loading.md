# Clipboard Notes and Stable Loading Implementation Plan

**Goal:** Add searchable private notes to clipboard records and remove duplicate, layout-shifting favorite loading states.

## 1. Stabilize clipboard loading UI

- Prove the duplicate state from the two history loading branches bound to `loading`.
- Keep one compact toolbar spinner.
- Preserve the history area's height while an empty filtered result is loading.
- Keep the favorite star button visually stable while its request is pending.

## 2. Add private searchable notes to the backend

- Add a `note` column with a safe default for existing records.
- Add note metadata to the domain model and PATCH endpoint.
- Normalize notes and enforce a 500-character Unicode limit.
- Search notes together with content, file name, and tags.
- Strip notes from public share responses.
- Cover normalization, search, validation, authorization, and public privacy with Go tests.

## 3. Add note editing and display to the Vue frontend

- Extend the clipboard API model and metadata update call.
- Turn the tag editor into a record organization dialog with a multiline note and tags.
- Display notes on records and mention notes in the search placeholder.
- Keep the dialog and record layout responsive.

## 4. Verify, integrate, and deploy

- Run backend tests and frontend production build locally.
- Review the diff for accidental deployment/server metadata.
- Commit the feature branch, merge it into `develop`, and push `develop`.
- Build deploy artifacts locally, update only Suidu API/web containers, and validate health, schema, site assets, and logs.
