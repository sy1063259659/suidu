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

## Storage boundary

The application must access files through a storage adapter. It must not read or write SFTPGo's database tables directly. This keeps file storage replaceable while PostgreSQL remains the source of truth for Suidu content metadata.
