# 随渡 Suidu

随渡是一个跨端内容中转与轻量记事工具，面向个人使用场景，支持文本、图片、文件和笔记在手机与电脑之间传递。

## 技术栈

- Frontend: Vue 3, TypeScript, Vite, Naive UI, Pinia
- Backend: Go, Gin v1
- Data: PostgreSQL, Redis
- File service: SFTPGo Community with local filesystem storage
- Deployment: Docker Compose

## Repository workflow

This repository follows GitFlow:

- `main`: production-ready code
- `develop`: integration branch
- `feature/*`: feature work
- `release/*`: release preparation
- `hotfix/*`: production fixes

## Local development

### Frontend

```bash
cd frontend
npm install
npm run dev
```

### Backend

```bash
cd backend
go run ./cmd/server
```

The API health endpoint is available at `http://localhost:8080/api/health`.

### Authentication

Suidu uses PostgreSQL-backed accounts and Redis-backed sessions. The first account must be created as an administrator from the server/container shell; no permanent login password is stored in environment variables:

```bash
docker compose exec suidu-api suidu-api admin create-user
```

Sign in with that administrator account, create a separate ordinary user from the user management panel, and use the ordinary account for daily clipboard access. Every user can change their own password. If an administrator needs to recover an account, reset it with:

```bash
docker compose exec suidu-api suidu-api admin reset-password
```

Clipboard records are isolated by user. Existing records without an owner are assigned to the first administrator when the API starts after the authentication migration.

### Services

Copy `.env.example` to `.env`, set the existing server's PostgreSQL and Redis addresses, then start the application containers:

```bash
docker compose up -d --build
```

Create the `suidu` and `sftpgo` PostgreSQL databases and their users before the first deployment. The Compose file pins SFTPGo to a reviewed release tag; update it deliberately when upgrading.
