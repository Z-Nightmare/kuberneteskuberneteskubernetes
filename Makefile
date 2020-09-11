now 		  := $(shell date)
PREFIX		  ?= zeusro
APP_NAME      ?= $app:latest
IMAGE		  ?= $(PREFIX)/$(APP_NAME)
MIRROR_IMAGE  ?= registry.cn-shenzhen.aliyuncs.com/amiba/$app:latest
ARCH		  ?=amd64

auto_commit:   
	git add .
	git commit -am "$(now)"
	git push

buildAndRun:
	GOARCH=$(ARCH) CGO_ENABLED=0 go build
	./common-bandwidth-auto-switch

fix-dep:
	go mod tidy
	go mod vendor

# usage dep="github.com/stretchr/testify/assert" make get
get:
	go get -u -v $(dep)
	go mod tidy && go mod verify && go mod vendor

init:
	go mod init 

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
	
