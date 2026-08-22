# Architecture

## Services

- `suidu-web`: Vue 3 single-page application and PWA assets.
- `suidu-api`: Go API built with Gin v1.
- PostgreSQL: durable application data and SFTPGo metadata, kept in separate databases.
- Redis: sessions, rate limiting, short-lived cache, and asynchronous task coordination.
- SFTPGo Community: local filesystem file service.

## Storage boundary

The application must access files through a storage adapter. It must not read or write SFTPGo's database tables directly. This keeps file storage replaceable while PostgreSQL remains the source of truth for Suidu content metadata.

