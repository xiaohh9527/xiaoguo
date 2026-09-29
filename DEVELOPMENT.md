# 小果短剧 (Xiaoguo) 编程开发规范与架构约定

本文档为 **小果短剧 (Xiaoguo)** 项目的标准化编程、开发规范与架构设计约定，供人类开发者与 AI Coding Agent 共同遵循。

---

## 1. 项目定位与全栈架构

小果短剧是一个解耦、高性能、支持多端消费的独立全栈短剧平台，由四个核心部分构成：

```
                    ┌─────────────────────────┐
                    │    小果 Go 后端服务     │
                    │   (xiaoguo-server)      │
                    │   提供 API + 1080P 解密 │
                    └────────────┬────────────┘
                                 │
         ┌───────────────────────┼───────────────────────┐
         │                       │                       │
         ▼                       ▼                       ▼
┌──────────────────┐   ┌───────────────────┐   ┌───────────────────┐
│  Vue 3 Web 前端  │   │ AList-TvBox 插件  │   │  Docker 容器镜像  │
│  (手机/PC/平板)  │   │  (小果短剧.py)    │   │ (跨架构多端一键部署)│
└──────────────────┘   └───────────────────┘   └───────────────────┘
```

1. **后端服务 (`server/`)**：基于 Go 1.23 开发的高性能 REST API 服务，负责接口请求签名、CENC（AES-128-CTR）媒体解密流、图片防盗链代理与用户播放数据持久化。
2. **Web 前端 (`web/`)**：基于 Vue 3 + Vite 构建的现代化响应式 Web 应用，支持暗色影院模式、分集折叠、键盘控制、实时弹幕及多分辨率切换。
3. **TV 插件 (`小果短剧.py`)**：严格遵循 AList-TvBox / TVBox 规范的 Python 扩展源，纯后端驱动，支持后台可视化表单编辑与 1080P 超清流直连。
4. **容器发布 (`Dockerfile`, `docker-compose.yml`)**：跨架构（AMD64 + ARM64）静态编译 Alpine 镜像，体积 < 30MB，支持小主机/NAS/单板计算机开箱即用。

---

## 2. 核心业务逻辑与技术红线

在修改或扩展本项目时，**必须严格遵守以下关键技术红线**（违背以下规则将导致功能不可用或线上故障）：

### 🔴 红线 1：64 位短剧 ID 绝对禁止使用 Float64 解析（精度丢失防护）
- **现象与机理**：短剧 `series_id` 为 19 位超大整数（如 `7686843591821364249`）。Go 标准 `json.Unmarshal` 默认将数字解包为 `float64`。IEEE 754 双精度浮点仅有 53 位尾数，解析 19 位整型会导致末尾 3~4 位被截断归零（如 `...000`），从而引发 404 与 HTTP 500 报错。
- **强制约束**：后端所有处理外部 API 响应的 JSON 解析器，**必须显式调用 `decoder.UseNumber()`**，将数字以 `json.Number`（原生字符串）形式存储，禁止使用未指定配置的 `json.Unmarshal` 解析至 `map[string]any`。

### 🔴 红线 2：图片格式防 HEIC 陷阱与防盗链代理
- **现象与机理**：官方 App API 检测到客户端为 Android 9+（API >= 28）时，会默认下发 `.heic` 格式封面。Chrome、Edge、Safari 及大多数电视播放器原生不支持 `<img>` 标签直接渲染 HEIC，导致封面全部变灰报错。
- **强制约束**：
  1. 模拟 App 接口参数时，必须锁定 `"os_api": "25"` 和 `"os_version": "7.1.2"`，强制服务端下发标准 `.image`（JPEG）格式；
  2. 所有第三方图片地址必须由后端的 `/api/proxy/image?url=...` 进行防盗链代理与内容嗅探，确保返回标准 `image/jpeg` 响应头；
  3. 插件或前端若检测到 URL 中带有 `.heic`，必须自动切除后缀模板参数。

### 🔴 红线 3：搜索排重与元数据合并机制
- **约束要求**：
  1. 搜索结果由“搜索联想词”与“网页综合搜索”合并而成；
  2. 必须使用 `addOrMerge` 逻辑同时以 `SourceID` 与标准化片名 `normalizeSearchText(Title)` 双重排重；
  3. 严禁返回同名同内容的双胞胎卡片，遇重复项时保留最全的集数、封面与标签数据。

### 🔴 红线 4：全集 1080P 流式解密与分块播放
- **约束要求**：
  1. 视频流原生采用 CENC (AES-128-CTR) 加密，由后端 `DecryptCENCMP4` 统一内存解密并重写 MP4 Box 盒子头（`encv` -> `orig_fmt`，`sinf` -> `free`）；
  2. 流媒体服务必须设置 `Accept-Ranges: bytes`，完整支持 HTTP 206 范围请求，确保电视和浏览器能快速拖动进度条；
  3. 后端 `/play` 和 `/api/stream` 接口必须同时支持 `vid`、`id`、`sid|vid`、`sid*vid` 等多种调用参数。

---

## 3. AList-TvBox 插件开发规范

`小果短剧.py` 专为 AList-TvBox 插件体系定制，必须满足：

1. **纯后端接口驱动（禁止独立网页爬取模式）**：
   - 插件自身不进行脆弱易封禁的网页端 HTML/Scraper 抓取；
   - 列表、榜单、搜索、详情、播放均 100% 依赖小果后端（`self.server_url`）提供的接口，确保 1080P 全集播放稳定性。
2. **声明后台表单配置 Schema (`PLUGIN_CONFIG_SCHEMA`)**：
   - 必须在文件头部声明 `//@config-schema` 与 `PLUGIN_CONFIG_SCHEMA` 字典；
   - 支持在 AList-TvBox 网页管理后台直接通过**表单编辑**可视化配置：
     - `server_url`：小果后端服务地址（默认 `http://127.0.0.1:8080`）
     - `quality`：默认播放清晰度（默认 `1080`）
     - `proxy_cover`：海报后端代理开关（默认 `true`）
3. **函数签名与返回值规范**：
   - `categoryContent(self, tid, pg, filter, extend)`：必须保留 4 个参数；
   - `searchContent(self, key, quick, pg="1")`：必须支持分页与 quick 参数；
   - `playerContent(self, flag, id, vipFlags)`：输出标准 `{parse: 0, playUrl: "", url: url, jx: 0, header: ...}`；
   - 所有接口返回的字典中**必须显式包含 `"parse": 0, "jx": 0`**，禁止省略，防止被 AList-TvBox 误触发第三方解析。
4. **双文件同步**：
   - 仓库内保持 `小果短剧.py` 与 `红果短剧.py` 逻辑完全一致，方便不同用户旧版命名兼容。

---

## 4. Docker 容器化与跨架构构建规范

1. **构建极速化（避免 QEMU 模拟 Node.js）**：
   - 前端静态构建阶段必须使用 `--platform=$BUILDPLATFORM node:20-alpine`，在构建机原生 x86 上秒级完成，禁止在 ARM 模拟器中运行 V8 引擎；
   - 后端使用 `--platform=$BUILDPLATFORM golang:1.23-alpine`，利用 Go 内置的 `CGO_ENABLED=0 GOARCH=$TARGETARCH` 原生交叉编译，单次编译仅需 2~3 秒。
2. **运行环境精简化**：
   - 基于 `alpine:3.20`，安装 `ca-certificates`（保障 HTTPS 正常通信）和 `tzdata`（`TZ=Asia/Shanghai`）；
   - 最终镜像体积必须控制在 **30MB 以内**。
3. **持久化与环境变量**：
   - 数据目录统一挂载至 `/app/data`；
   - 后端启动优先读取 `PORT`、`DATA_DIR`、`WEB_DIR` 环境变量。

---

## 5. 工程规范与 Git 工作流

1. **仓库洁净原则（Strict Cleanliness）**：
   - 严格通过 `.gitignore` 排除本地构建物与缓存：
     - 禁止提交：`node_modules/`、`.gocache/`、`*.exe`、`server/data/cache/`、`guoapp/`；
     - `guoapp/` 为历史参考项目，独立存在于本地，绝不推送到 GitHub 远端仓库。
2. **Git 提交信息规范 (Conventional Commits)**：
   - `feat:` 新增功能
   - `fix:` 修复缺陷（如精度截断、404、500 等）
   - `perf:` 性能与构建提速优化
   - `docs:` 文档与规范调整
   - `refactor:` 代码重构
3. **自动化同步流**：
   - 代码修改与本地验证通过后，自动同步更新 `小果短剧.py` 并推送到 GitHub 远端仓库的 `main` 分支；
   - GitHub Actions 监听 `main` 分支变动，全自动打包并推送多架构镜像至 GitHub Container Registry（`ghcr.io`）。

