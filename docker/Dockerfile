# 后端构建阶段
FROM golang:1.24-alpine AS builder

WORKDIR /app

# 安装必要的构建工具
RUN apk add --no-cache git

# 复制Go项目文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY main.go .
COPY dl/ ./dl/
COPY tool/ ./tool/
COPY parse/ ./parse/

# 复制预构建的前端文件
COPY web/dist/ ./web/dist/

# 构建后端应用 - 使用更多优化标志
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o m3u8-downloader .

# 最终阶段：使用最小化的Alpine镜像
FROM alpine:3.19

# 安装精简版ffmpeg和必要证书
RUN apk add --no-cache ca-certificates && \
    apk add --no-cache ffmpeg

# 创建工作目录和下载目录
WORKDIR /app
RUN mkdir -p /data

# 仅复制必要的文件
COPY --from=builder /app/m3u8-downloader /app/
COPY --from=builder /app/web/dist /app/web/dist

# 暴露端口
EXPOSE 9100

# 设置环境变量
ENV GIN_MODE=release
ENV OUTPUT_DIR=/data
ENV FFMPEG_PATH=/usr/bin/ffmpeg

# 启动应用
CMD ["/app/m3u8-downloader"] 