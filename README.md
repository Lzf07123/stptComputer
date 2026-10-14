# 计算机协会招新答题系统

现场高并发答题：实时计分、全量排行榜、大屏特效、加群活码、后台运营。
技术栈：Go 1.22 + gorilla/websocket + Redis + PostgreSQL + React 18 (Vite) + Nginx，全程 Docker 化。

> 设计文档见 [docs/实现方案.md](docs/实现方案.md)

## 功能一览

- **吸睛首页**：协会会徽 + 照片墙横向轮播（后台批量上传）+ 科技感叠加特效（粒子网络/网格/光晕/扫描光束）
- **在线答题**：一人一设备一身份；难度自选（全部/容易/简单/困难）；每题即时判分；断网不丢分（幂等 + 本地缓冲重试）；刷新断线可恢复
- **排行榜**：全量排名（未答题也排、名次无限）、大屏最多显示 99 名、头部显示「我的排名」、参与/答题/在线人次统计
- **身份规则**：设备指纹去重（同设备换昵称需先放弃）；放弃身份需二次确认并告知副作用；放弃后昵称永久封存、榜单留痕；可多轮答题取历史最佳成绩
- **后台管理**（口令验证后可见）：系统概览（资源/访问）、答题控制/特效/公告、群二维码图片切换、题库增删停/批量导入 CSV、成绩导出 CSV、首页背景/页脚信息（ICP+公安备案）配置

## 快速开始（全程 Docker，不污染本机）

```bash
cp .env.example .env   # 可选：覆盖默认配置（变量见下表；模板仅占位值，不含真实配置）
docker compose -f deploy/docker-compose.yml up -d --build
```

| 入口 | 说明 |
|---|---|
| http://localhost:8080 | 首页 |
| /quiz | 答题 |
| /result | 个人成绩 |
| /screen | 排行榜大屏（投屏） |
| /admin | 后台管理（口令见 .env） |
| /healthz | 健康检查 |

停止：`docker compose -f deploy/docker-compose.yml down`（加 `-v` 清空数据库与上传文件）

## 环境变量（.env，不入库；模板见 .env.example）

| 变量 | 默认 | 说明 |
|---|---|---|
| `ADMIN_PASSWORD` | admin123 | 后台口令（务必修改） |
| `QUESTION_SECONDS` | 20 | 每题倒计时（秒） |
| `QUESTION_COUNT` | 10 | 每轮抽题数 |
| `WEB_PORT` | 8080 | 对外端口 |

### 基础镜像与软件源（构建/部署阶段均可变）

| 变量 | 默认 | 作用 |
|---|---|---|
| `GOLANG_IMAGE` | golang:1.22-alpine | 后端构建镜像 |
| `ALPINE_IMAGE` | alpine:3.20 | 后端运行镜像 |
| `NODE_IMAGE` | node:20-alpine | 前端构建镜像 |
| `NGINX_IMAGE` | nginx:1.27-alpine | Web/反代镜像 |
| `POSTGRES_IMAGE` | postgres:15-alpine | 数据库镜像 |
| `REDIS_IMAGE` | redis:7-alpine | 缓存镜像 |
| `SOURCE_MIRROR` | （空） | 软件源统一开关：空=官方源；`cn`=国内镜像组合（Go 走 goproxy.cn、npm 走 npmmirror、apk 走阿里云） |

- 镜像变量为完整引用，**可带 registry 前缀**（如 `mirror.example.com/golang:1.22-alpine`），适配内网仓库。
- 国内加速（写入 `.env` 后重新 `up -d --build`）：

```bash
SOURCE_MIRROR=cn
#GOLANG_IMAGE=docker.1ms.run/library/golang:1.22-alpine  # 如需镜像仓库前缀
```

页脚/备案/二维码/背景图等运行期内容一律在后台配置，持久化于 postgres 与 uploads 两个 Docker 数据卷，不走环境变量。

## 后台使用速查

1. 访问 `/admin`，输入后台口令验证（错误口令无法进入）
2. **概览**：注册用户/参与答题/累计答题/当前在线/题库/背景/内存/协程/运行时长/DB/Redis
3. **控制台**：开始/结束答题、特效广播、公告广播、群二维码上传切换、**导出成绩 CSV**
4. **题库**：**批量导入**（下载模板，列：题目,选项A~F,正确答案(A-F),难度,分值；「示例」开头行自动跳过）、新增题目、启用/停用
5. **外观**：页脚信息（拥有人/开源协议/ICP+公安备案号与图标，图标后台上传）、首页背景批量上传（原比例、横向滚动）

## 关键 API（全部 JSON；`*` 需 X-Admin-Token 头）

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | /api/session | 昵称+设备指纹 → 身份（同设备同昵称自动恢复） |
| POST | /api/quiz/round | 开始一轮（body: difficulty 可选） |
| GET | /api/quiz/current | 恢复进行中的一轮 |
| POST | /api/answer | 提交（request_id 幂等） |
| GET | /api/me | 我的最佳成绩与全量排名 |
| GET | /api/rank?top=99 | 排行榜 |
| GET | /api/stats | 参与/答题/在线人次 |
| GET | /api/backgrounds · /api/qrcode · /api/config | 前台资源 |
| GET | /ws?token=… | WebSocket 实时推送（匿名=大屏） |
| POST | /api/identity/abandon | 放弃身份（需本机 device_fp 归属校验） |
| GET* | /api/admin/overview | 系统概览 |
| GET* | /api/admin/export/scores | 导出成绩 CSV |
| POST* | /api/admin/questions/import | 批量导入题目 CSV |
| GET* | /api/admin/questions/template | 下载导入模板 |
| POST* | /api/admin/{quiz-state,announce,effect,qrcode,uploads,footer,…} | 后台操作 |

## 目录结构

```
backend/    Go 后端（cmd/server + internal/{config,store,rank,publish,quiz,ws,httpapi}）
frontend/   React 18 + Vite（6 页面 + WS 客户端 + 幂等重试）
deploy/     docker-compose.yml + nginx.conf + web 构建镜像
docs/       实现方案
```

## 云端部署要点

- 已备案域名 + 外部反向代理：TLS 在外部反代终止，本机 Nginx 只走 HTTP；**两层代理都要透传 WebSocket Upgrade 头**（见 docs §8-§9）
- 前端构建产物建议发布到 OSS+CDN（现场"进门一分钟"静态流量集中）
- 上线前：`.env` 改强口令 → 后台录入真实题库/二维码/备案号 → 压测（§1.3 指标）→ 现场演练

## 开发调试

- 前端热更：`cd frontend && npm run dev`（已配代理到 :8080；仅开发环境使用，不污染部署）
- 后端改代码后：`docker compose -f deploy/docker-compose.yml up -d --build app web`
- 查看日志：`docker compose -f deploy/docker-compose.yml logs -f app`

## License

MIT © 计算机协会 · 汕头职业技术学院
