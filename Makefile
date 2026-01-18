BUILD_TIME 			  := $(shell LC_TIME=zh_CN.UTF-8 date +"%Y-%m-%d %H:%M:%S %A")
#BUILD_TIME            := $(shell date +%Y-%m-%dT%H:%M:%S%z 2>/dev/null || powershell -Command "Get-Date -Format o")
PREFIX		  ?= zeusro
APP_NAME      ?= $app:latest
IMAGE		  ?= $(PREFIX)/$(APP_NAME)
MIRROR_IMAGE  ?= registry.cn-shenzhen.aliyuncs.com/mirror/$app:latest
MODULE		  ?= $org/$project-name
ARCH		  ?= amd64

auto_commit:
	git add .
	git commit -am "$(BUILD_TIME)"
	# git remote add template git@github.com:zeusro/go-template.git
	# git pull template master
	git pull
	git push

buildAndRun:
	GOARCH=$(ARCH) CGO_ENABLED=0 go build
	# ./$app

fix-dep:
	go mod tidy
	go mod vendor

# usage: dep="github.com/stretchr/testify/assert" make get
get:
	go get -u -v $(dep)
	go mod tidy && go mod verify && go mod vendor

init:
	@if [ -z "$(project)" ]; then \
		echo "❌ 请先设置 project 环境变量，例如：make init project=dir-monitor"; \
		exit 1; \
	else \
		echo "✅ 初始化 Go 项目：$(project)"; \
		go mod init $(project); \
		go mod tidy; \
	fi

mirror: pull
	docker build -t $(MIRROR_IMAGE) -f deploy/docker/Dockerfile .

pull:
	git reset --hard HEAD
	git pull

release-mirror: mirror
	docker push $(MIRROR_IMAGE)

rebuild: pull	
	docker build -t $(IMAGE) -f deploy/docker/Dockerfile .

test:
	mkdir -p artifacts/report/coverage
	go test -v -cover -coverprofile c.out.tmp ./...
	cat c.out.tmp | grep -v "_mock.go" > c.out
	go tool cover -html=c.out -o artifacts/report/coverage/index.html	

update-dep: update-mod fix-dep

up:
	docker-compose build --force-rm --no-cache
	docker-compose up

update-mod:
	# type your dep
	
