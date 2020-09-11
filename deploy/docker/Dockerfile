FROM golang:1.14.0-alpine3.11 as build-env
ADD . /src/github.com/zeusro/$project-name
WORKDIR /src/github.com/zeusro/$project-name
RUN apk update
# 分层构建,复用缓存
RUN GOARCH=amd64 CGO_ENABLED=0 go build -mod vendor -o $app  github.com/zeusro/$project-name

FROM alpine:3.11
COPY --from=build-env /src/github.com/zeusro/$project-name/$app /app/$app
# COPY config.yaml /app/config.yaml
WORKDIR /app
ENTRYPOINT ["/app/$app"]
