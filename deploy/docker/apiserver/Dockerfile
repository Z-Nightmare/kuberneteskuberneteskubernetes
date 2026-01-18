# 构建阶段
FROM golang:1.25-alpine AS builder

WORKDIR /app

# 复制 go mod 文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o apiserver ./cmd/apiserver/main.go

# 运行阶段
FROM alpine:latest

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/apiserver .

# 复制静态文件
COPY --from=builder /app/cmd/apiserver/static ./static

# 复制配置文件示例（可选）
COPY --from=builder /app/cmd/apiserver/config-example.yaml ./config-example.yaml

EXPOSE 8080

ENTRYPOINT ["/app/apiserver"]
