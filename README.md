# 小果短剧 (Xiaoguo) Web 应用 (Go + Vue 3)

从 `guoapp` 多站点短剧系统中单独提取**红果短剧（Hongguo）**模块，打造的独立全栈 Web 应用。

---

## 🌟 核心功能与亮点

- 🎬 **精选推荐与分类流**：支持真人剧、漫剧、AI剧、动漫等多种分类，支持无感无限滚动分页与游标拉取。
- 🏆 **全网实时热榜**：热播榜、真人剧榜、漫剧榜、AI剧榜实时同步，前三名殿堂级卡片展示。
- 🔍 **智能搜索与联想词**：输入即联想，支持多季短剧系列整合（第X季自动补全）。
- ⚡ **超低延迟 CENC 自动解密**：
  - 红果短剧原生加密采用 `cenc-aes-ctr` (ISO/IEC 23001-7)。
  - Go 后端内置高性能纯内存解密引擎，**单集解密仅需约 17 毫秒**。
  - 转换为标准无加密 MP4，完美支持在任何浏览器（手机、电脑、微信、平板、电视）中以 HTML5 `<video>` 原生无缝播放。
- 🎛️ **影院级播放器控制**：
  - 多档清晰度无缝切换（1080P 超清 / 720P 高清 / 540P 标清 / 480P 流畅）。
  - 倍速播放（0.75x、1.0x、1.25x、1.5x、2.0x）。
  - 快捷键盘控制（空格键暂停/播放、左右方向键快进快退 5 秒、上下方向键调音量、F 键全屏、M 键静音）。
  - 选集面板自动分段（针对 80~100+ 集短剧提供 1-30、31-60 等分页折叠）。
  - 自动续播下一集。
  - **一键下载**：点击下载即可直接保存解密后的标准 MP4 视频到本地。
- 💬 **实时弹幕互动**：接入红果官方真实弹幕评论流，支持弹幕开关、发射自定义互动弹幕。
- 📚 **我的追剧与播放历史**：本地持久化保存用户的追剧收藏和播放进度（精确到秒数），随时无缝断点续播。
- 📱 **响应式自适应设计**：手机端专属底部快捷栏，触控手感流畅，PC 端沉浸式大屏影院布局。

---

## 📁 目录结构

```
xiaoguo/
├── server/                    # Go 后端工程
│   ├── go.mod                 # Go 依赖配置
│   ├── main.go                # 后端服务入口 (端口 8080)
│   ├── internal/
│   │   ├── hongguo/           # 红果短剧核心引擎
│   │   │   ├── client.go      # HTTP 客户端与官方请求封装
│   │   │   ├── sign.go        # X-Gorgon / X-Khronos 请求签名
│   │   │   ├── comment_sign.go# X-Argus / X-Ladon 弹幕评论签名
│   │   │   ├── catalog.go     # 推荐与分类流
│   │   │   ├── rankings.go    # 四大榜单解析
│   │   │   ├── detail.go      # 短剧详情与章节列表解析
│   │   │   ├── playback.go    # 播放地址与密钥解析
│   │   │   ├── cenc.go        # CENC (AES-CTR) 17ms 秒级解密器
│   │   │   ├── danmaku.go     # 官方弹幕拉取
│   │   │   ├── search.go      # 搜索与搜索联想
│   │   │   └── stream_cache.go# 视频缓存流式分发与图片防盗链代理
│   │   ├── storage/           # 追剧收藏与历史记录存储
│   │   └── api/               # RESTful API 路由与控制器
│   └── data/                  # 本地数据与缓存目录
├── web/                       # Vue 3 前端工程
│   ├── src/
│   │   ├── api/               # 前端 API 请求模块
│   │   ├── assets/            # 全局样式与变量
│   │   ├── components/        # 公用组件 (Navbar, VideoPlayer, DanmakuOverlay, DramaCard 等)
│   │   ├── views/             # 视图页面 (HomeView, PlayerView, RankingsView 等)
│   │   ├── router/            # 路由定义
│   │   ├── App.vue            # 根组件
│   │   └── main.js            # 入口脚本
│   └── dist/                  # 前端编译产物
├── start.bat                  # 一键启动脚本
├── start_dev.bat              # 前后端热重载开发启动脚本
└── build.bat                  # 一键构建脚本
```

---

## 🚀 快速启动

### 方式一：一键启动（推荐）

双击项目根目录下的 **`start.bat`** 即可：
- 自动检测并编译运行后端服务
- 自动检测并编译前端静态资源
- 自动打开浏览器访问 **`http://localhost:8080`**

### 方式二：手动运行

#### 1. 编译前端（若已构建在 `web/dist` 则可跳过）：
```bash
cd web
pnpm install # 或 npm install
pnpm run build
```

#### 2. 运行 Go 后端服务：
```bash
cd server
go run main.go -port 8080 -web ../web/dist
```

浏览器打开：`http://localhost:8080`

### 方式三：前后端热重载开发模式

双击运行 **`start_dev.bat`**：
- 后端监听 `http://localhost:8080`
- 前端 Vite 开发服务器运行于 `http://localhost:5173`（带 API 代理与实时热重载）

---

## 🔌 API 接口概览

| 接口路径 | 方法 | 说明 |
| :--- | :--- | :--- |
| `/api/genres` | GET | 获取短剧分类（真人剧、漫剧、AI剧、动漫） |
| `/api/catalog` | GET | 获取推荐/分类瀑布流（支持 `genre`, `offset`, `session_id` 分页） |
| `/api/ranking-boards` | GET | 获取榜单列表（热播榜、真人剧榜、漫剧榜、AI剧榜） |
| `/api/rankings` | GET | 获取指定榜单剧集排行（支持 `board`, `page` 参数） |
| `/api/detail` | GET | 获取短剧详情及完整分集列表（参数 `id`） |
| `/api/play` | GET | 解析视频源与各分辨率视频流（参数 `series_id`, `video_id`） |
| `/api/stream` | GET | 获取解密后的标准 MP4 流（原生支持 Range 分段播放与进度拖动） |
| `/api/danmaku` | GET | 获取视频对应时间段的弹幕（参数 `series_id`, `video_id`） |
| `/api/search` | GET | 搜索短剧（参数 `keyword`） |
| `/api/search/suggestions`| GET | 搜索联想补全词（参数 `query`） |
| `/api/proxy/image` | GET | 封面图片防盗链代理（参数 `url`） |
| `/api/favorites` | GET/POST/DELETE | 用户追剧清单管理 |
| `/api/history` | GET/POST/DELETE | 用户观看进度历史管理 |

