package service

import "go.uber.org/fx"

var Modules = fx.Options(
	fx.Provide(NewHealthService),
	//todo 有新的服务需要添加到这里
)
