build:
	go build -o app
	chmod +x ./app

run: build
	./app

debug: build
	# todo debug 模式改为控制台输出日志
	./app --debug