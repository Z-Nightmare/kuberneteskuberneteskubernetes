package core

import (
	"go.uber.org/fx"
	"zeusro.com/hermes/internal/core/config"
	"zeusro.com/hermes/internal/core/logprovider"
	"zeusro.com/hermes/internal/core/webprovider"
)

var CoreModule = fx.Options(
	fx.Provide(config.NewFileConfig),
	fx.Provide(logprovider.GetLogger),
	//todo 集成数据库
	// fx.Provide(NewDatabase),
	fx.Provide(webprovider.NewGinEngine),
)
