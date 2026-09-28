# ==========================================
# 阶段 1: 构建前端 Vue 静态页面
# 前端产物为静态资源，使用原生构建机架构运行即可，避免 QEMU 模拟导致速度极其缓慢
# ==========================================
FROM --platform=$BUILDPLATFORM node:20-alpine AS frontend-builder
WORKDIR /app/web

# 优先使用官方源 (GitHub Actions 运行在海外千兆网络，直连最快)
ARG NPM_REGISTRY=https://registry.npmjs.org/
RUN npm config set registry ${NPM_REGISTRY}

# 复制依赖定义并安装
COPY web/package.json ./
RUN npm install

# 复制前端源码并执行打包构建
COPY web/ ./
RUN npm run build

# ==========================================
# 阶段 2: 构建后端 Go 二进制
# 利用 Go 语言原生的毫秒级交叉编译能力 (GOARCH)，免去 QEMU 模拟器开销
# ==========================================
FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS backend-builder
WORKDIR /app/server

ARG TARGETOS
ARG TARGETARCH
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}

# 复制 go.mod 和 go.sum 预下载依赖
COPY server/go.mod server/go.sum* ./
RUN go mod download

# 复制后端源码并跨架构静态编译 (CGO_ENABLED=0，纯静态二进制)
COPY server/ ./
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -ldflags="-s -w" -o xiaoguo-server .

# ==========================================
# 阶段 3: 最终精简运行镜像 (对应目标架构 Alpine，体积 < 30MB)
# ==========================================
FROM alpine:3.20

# 安装 HTTPS 证书与时区支持
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai

WORKDIR /app

# 从构建阶段复制产物
COPY --from=backend-builder /app/server/xiaoguo-server /app/xiaoguo-server
COPY --from=frontend-builder /app/web/dist /app/web/dist

# 创建持久化数据目录
RUN mkdir -p /app/data

# 暴露服务端口
EXPOSE 8080

# 挂载数据卷
VOLUME ["/app/data"]

# 默认环境变量
ENV PORT=8080
ENV DATA_DIR=/app/data
ENV WEB_DIR=/app/web/dist

# 启动命令
ENTRYPOINT ["/app/xiaoguo-server"]
CMD ["-port", "8080", "-data", "/app/data", "-web", "/app/web/dist"]
