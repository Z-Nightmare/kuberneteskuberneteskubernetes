package core

import (
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/config"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/logprovider"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/webprovider"
	"go.uber.org/fx"
)

var CoreModule = fx.Options(
	fx.Provide(config.NewFileConfig),
	fx.Provide(logprovider.GetLogger),
	//todo 集成数据库
	// fx.Provide(NewDatabase),
	fx.Provide(webprovider.NewFiberEngine),
)
