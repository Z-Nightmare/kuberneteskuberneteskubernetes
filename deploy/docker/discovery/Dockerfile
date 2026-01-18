# 构建阶段
FROM golang:1.25-alpine AS builder

WORKDIR /app

# 复制 go mod 文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o discovery ./cmd/discovery/main.go

# 运行阶段
FROM alpine:latest

# 安装 ca-certificates 用于 HTTPS 请求（Consul API）
RUN apk --no-cache add ca-certificates

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/discovery .

# 复制配置文件示例（可选）
COPY --from=builder /app/cmd/discovery/config-example.yaml ./config-example.yaml

# 暴露健康检查端口（默认 7946，可通过 --service-port 参数修改）
EXPOSE 7946

ENTRYPOINT ["/app/discovery"]
