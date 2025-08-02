FROM golang:1.24-alpine AS builder

WORKDIR /app

# 复制 go mod 文件并下载依赖
COPY go.mod go.sum ./
RUN go mod tidy

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o okx-sentry cmd/main.go

# 第二阶段：运行时镜像
FROM alpine:latest

# 安装 ca-certificates 和时区数据
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# 从构建阶段复制二进制文件和配置
COPY --from=builder /app/okx-sentry .
COPY --from=builder /app/configs ./configs

# 创建日志目录
RUN mkdir -p logs

# 暴露健康检查端口
EXPOSE 8080

# 设置时区
ENV TZ=Asia/Shanghai

CMD ["./okx-sentry"]