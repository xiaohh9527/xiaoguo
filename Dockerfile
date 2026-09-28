# ==========================================
# 阶段 1: 构建前端 Vue 静态页面
# ==========================================
FROM node:20-alpine AS frontend-builder
WORKDIR /app/web

# 配置 npm 加速镜像源 (构建时支持国内镜像源)
ARG NPM_REGISTRY=https://registry.npmmirror.com
RUN npm config set registry ${NPM_REGISTRY}

# 复制依赖定义并安装
COPY web/package.json ./
RUN npm install

# 复制前端源码并执行打包构建
COPY web/ ./
RUN npm run build

# ==========================================
# 阶段 2: 构建后端 Go 可执行二进制
# ==========================================
FROM golang:1.23-alpine AS backend-builder
WORKDIR /app/server

# 配置 GOPROXY 代理加速
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}

# 复制 go.mod 和 go.sum 预下载依赖
COPY server/go.mod server/go.sum* ./
RUN go mod download

# 复制后端源码并静态编译 (CGO_ENABLED=0，跨 Linux 发行版通用)
COPY server/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o xiaoguo-server .

# ==========================================
# 阶段 3: 最终精简运行镜像 (基于 Alpine，体积 < 30MB)
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
