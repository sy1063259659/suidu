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

### Services

Copy `.env.example` to `.env`, update the credentials, then start the full stack:

```bash
docker compose up -d --build
```

The Compose file pins SFTPGo to a reviewed release tag; update it deliberately when upgrading.
