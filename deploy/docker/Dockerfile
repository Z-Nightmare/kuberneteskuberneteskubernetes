FROM golang:1.14.0-alpine3.11 as build-env
ADD . /src/$org/$project-name
WORKDIR /src/$org/$project-name
RUN apk update
# 分层构建,复用缓存
RUN GOARCH=amd64 CGO_ENABLED=0 go build -mod vendor -o $app  $org/$app

FROM alpine:3.11
COPY --from=build-env /src/$org/$project-name/$app /app/$app
# COPY config.yaml /app/config.yaml
WORKDIR /app
ENTRYPOINT ["/app/$app"]
