package apiserver

import (
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/webprovider"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/pkg/storage"
	"go.uber.org/fx"
)

// Module 提供 API server 模块
var Module = fx.Options(
	fx.Invoke(func(fiberEngine webprovider.FiberEngine, store storage.Store) {
		RegisterRoutes(fiberEngine, store)
	}),
)
