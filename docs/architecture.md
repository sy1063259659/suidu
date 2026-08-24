# Architecture

## Services

- `suidu-web`: Vue 3 single-page application and PWA assets.
- `suidu-api`: Go API built with Gin v1.
- PostgreSQL: durable application data and SFTPGo metadata, kept in separate databases.
- Redis: sessions, rate limiting, short-lived cache, and asynchronous task coordination.
- SFTPGo Community: local filesystem file service.

## Authentication

- PostgreSQL stores user accounts, Argon2id password hashes, roles, and disabled state.
- Redis stores opaque, expiring browser sessions and login-attempt counters.
- The first administrator is created through the API container CLI. Administrators can create, disable, and reset ordinary users from the web UI.
- Browser sessions use `HttpOnly`, `Secure`, `SameSite=Lax` cookies; passwords and session tokens are never stored in browser local storage.
- Clipboard records are scoped by `user_id`; handlers always filter reads and writes by the authenticated user.

## Public share boundary

- Authenticated owners create, list, and revoke links; owner identity is always taken from the session rather than the request body.
- PostgreSQL stores each share's owner, source item, 256-bit URL-safe random token, expiry, revocation time, and creation time. Deleting an item cascades to its shares.
- Public metadata and content routes require the opaque token but no account session. The database query resolves only rows that are not revoked and whose expiry is still in the future.
- Expired and revoked tokens use the same not-found response. Public JSON and binary responses use `Cache-Control: no-store` and attachment responses retain MIME sniffing protection.

## Storage boundary

The application must access files through a storage adapter. It must not read or write SFTPGo's database tables directly. This keeps file storage replaceable while PostgreSQL remains the source of truth for Suidu content metadata.

- PostgreSQL stores attachment kind, original file name, detected media type, size, and opaque storage key.
- The API writes binary content to the `suidu-files` volume under an owner-scoped, generated path; uploaded names are never used as storage paths.
- SFTPGo sees the same volume at `/srv/sftpgo/data`, while the API uses `/var/lib/suidu/files/suidu` through its storage adapter.
- Private downloads and previews require an authenticated, owner-scoped metadata lookup before storage is opened. Public content requires an active share lookup before the same storage adapter is opened.
- Only JPEG, PNG, GIF, WebP, AVIF, and BMP are eligible for inline rendering. Other formats, including SVG, are served as attachments with `nosniff`.
