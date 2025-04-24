package core

import (
	"go.uber.org/fx"
	"zeusro.com/gotemplate/internal/core/config"
	"zeusro.com/gotemplate/internal/core/logprovider"
	"zeusro.com/gotemplate/internal/core/webprovider"
)

var CoreModule = fx.Options(
	fx.Provide(config.NewFileConfig),
	fx.Provide(logprovider.GetLogger),
	//todo 集成数据库
	// fx.Provide(NewDatabase),
	fx.Provide(webprovider.NewGinEngine),
)
