# IdeaFlow / easyStock

单体仓库内含 **easystock-api**（Go + Tushare）与 **easyStock**（React + Vite）前端，用于选股与板块展示类场景。前端通过 **`/api/*`** 调用后端；业务数据依赖 **Tushare Pro**，未配置 Token 时接口会返回 **503**（无内置行情 mock）。

---

## 目录结构

| 路径 | 说明 |
|------|------|
| `easystock-api/` | HTTP API，默认监听 `:4000` |
| `easyStock/` | 前端 SPA，开发时 Vite `:5173`，生产构建为静态文件 |
| `docker-compose.yml` | 一键编排 `api` + `web`（Nginx 反代静态与 `/api`） |

---

## 前置条件

- **开发**：Go **1.22+**，Node **18+**（推荐 **20**，与 `easyStock/docker/Dockerfile` 一致）
- **部署**：Docker 与 Docker Compose v2（可选但推荐）

---

## 本地开发（不用 Docker）

### 1. 配置 Tushare（必填才有数据）

在 `easystock-api/` 下复制环境变量示例并填写 Token：

```bash
cp easystock-api/.env.example easystock-api/.env
# 编辑 easystock-api/.env，设置 TUSHARE_TOKEN=你的token
```

`go run` 时会自动尝试加载 `easystock-api/.env` 与当前目录下的 `.env`（不覆盖已在 Shell 里 export 的变量）。

### 2. 启动 API

```bash
cd easystock-api
go run ./cmd/server
```

日志出现 `listening on :4000` 即成功。自检：

```bash
curl -s http://127.0.0.1:4000/api/health
```

若返回 `"tushare":true`，说明 Token 已生效。

### 3. 启动前端

另开终端：

```bash
cd easyStock
npm install
npm run dev
```

浏览器访问 **http://127.0.0.1:5173**。Vite 已将 **`/api`** 代理到 **http://127.0.0.1:4000**（见 `easyStock/vite.config.ts`）。

也可以在 `easyStock` 目录用 `npm run dev:api` 在本机拉起 API（等价于进入 `easystock-api` 执行 `go run`，需已安装 Go）。

### 4. 可选：直连 API 地址

若前端与 API 不同源且不用代理，构建或开发时设置：

```bash
export VITE_API_URL=http://127.0.0.1:4000
npm run dev
```

---

## 生产部署（Docker Compose，推荐）

在仓库根目录：

```bash
docker compose build
```

编辑 `docker-compose.yml`，在 `api.environment` 中取消注释并设置 **`TUSHARE_TOKEN`**（或用 Compose 的 `env_file` 指向仅含密钥的文件，**勿提交 Git**）。

启动：

```bash
docker compose up -d
```

| 服务 | 说明 | 默认映射 |
|------|------|-----------|
| **web** | Nginx 托管前端静态资源，并把 **`/api/*`** 转发到 `api:4000`（见 `easyStock/docker/nginx.conf`） | **8080 → 80** |
| **api** | Go 二进制，Tushare 后端 | **4000 → 4000** |

访问前端：**http://主机:8080**（页面内请求走同源 **`/api`**，无需配置 `VITE_API_URL`）。

健康检查（API 容器或宿主机映射端口）：

```bash
curl -s http://127.0.0.1:4000/api/health
```

---

## 仅部署后端（自建 Nginx / 网关）

1. 在 `easystock-api` 目录构建 Linux 二进制，例如：

   ```bash
   cd easystock-api
   CGO_ENABLED=0 go build -o server ./cmd/server
   ```

2. 运行时可设置 `PORT`、**`TUSHARE_TOKEN`** 等（参见 `easystock-api/.env.example`）。

3. 前端执行 `npm run build`，将 `easyStock/dist` 挂到任意静态服务器，并对 **`/api/`** 做反向代理到后端地址（与 `easyStock/docker/nginx.conf` 同理）。

---

## 环境变量摘要（API）

| 变量 | 说明 |
|------|------|
| **`TUSHARE_TOKEN`** | Tushare Pro Token；未设置则依赖实时数据的接口返回 **503** |
| **`PORT`** | 监听端口，默认 `4000` |
| **`TUSHARE_TRADE_DATE`** | 可选，指定交易日 `YYYYMMDD`；不设则按交易日历推算 |
| **`TUSHARE_PICK_LIMIT`** / **`TUSHARE_PICKS_CACHE_MINUTES`** | 推荐列表条数与缓存 |
| **`TUSHARE_SECTOR_*`** | 板块聚合缓存与列表规模，见 `easystock-api/.env.example` |

---

## 常见问题

- **浏览器报代理错误 / `ECONNREFUSED 127.0.0.1:4000`**：先启动 **easystock-api**，再跑 `npm run dev`。
- **接口返回 `TUSHARE_TOKEN is required`**：为 API 进程配置 Token 后重启。
- **Docker 前端能打开但无数据**：确认 **`api` 服务**已注入 **`TUSHARE_TOKEN`**，且 `curl :4000/api/health` 中 **`tushare` 为 `true`**。

---

## 许可证与密钥

请勿将含 **`TUSHARE_TOKEN`** 或真实密钥的 `.env` 提交版本库；仓库根 `.gitignore` 已忽略常见 `.env` 路径。
