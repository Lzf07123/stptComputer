# 计算机协会招新答题系统

现场高并发答题：实时计分 + 实时排行榜 + 大屏特效 + 加群活码。
技术栈：Go + gorilla/websocket + Redis + PostgreSQL + React 18 (Vite) + Nginx。
详细设计见 [docs/实现方案.md](docs/实现方案.md)。

## 一键启动（全程 Docker，不污染本机环境）

```bash
cp .env.example .env   # 修改 ADMIN_PASSWORD
docker compose -f deploy/docker-compose.yml up -d --build
```

- 前端 + API 入口：http://localhost:8080
- 大屏：http://localhost:8080/screen
- 后台：http://localhost:8080/admin（口令为 .env 中的 ADMIN_PASSWORD）

停止：`docker compose -f deploy/docker-compose.yml down`（加 `-v` 清空数据库）

## 页面

| 路径 | 说明 |
|---|---|
| `/` | 吸睛首页（粒子 + 入场动画 + 双入口） |
| `/join` | 加群二维码（活码，30s 轮询自动切换） |
| `/quiz` | 答题（昵称领 token → 答题 → 成绩） |
| `/result` | 个人成绩 + 实时 Top 10 |
| `/screen` | 大屏投屏（实时排行 + 特效 + 公告） |
| `/admin` | 后台管理（题库、公告、特效、二维码、**首页背景上传轮播**） |

## 关键 API

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/session` | 昵称 → token |
| GET | `/api/questions` | 拉题（按用户乱序） |
| POST | `/api/answer` | 提交（request_id 幂等） |
| GET | `/api/rank?top=50` | 排行榜 |
| GET | `/api/me` | 我的成绩/排名 |
| GET | `/ws` | WebSocket 推送（可匿名=大屏） |
| POST | `/api/admin/*` | 后台（X-Admin-Token 头） |

## 目录结构

```
backend/    Go 后端（cmd/server + internal/{config,store,rank,publish,quiz,ws,httpapi}）
frontend/   React 前端（6 页面 + WS 客户端 + 幂等重试）
deploy/     docker-compose.yml + nginx.conf + web Dockerfile
docs/       实现方案
```

## 开发调试

前端开发模式（可选项）：`cd frontend && npm run dev`，已配置 Vite 代理到 `localhost:8080`。
后端只改代码后：`docker compose -f deploy/docker-compose.yml up -d --build app web`。
