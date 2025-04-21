# 使用 Go 官方镜像
FROM golang:1.24-alpine as builder

WORKDIR /app

COPY . .

RUN go mod tidy && go build -o gin-server main.go

# 运行镜像
FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/gin-server .
COPY ./static ./static

EXPOSE 8080

CMD ["/app/gin-server"]