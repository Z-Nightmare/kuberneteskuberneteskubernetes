package apiserver

import (
	"fmt"

	"go.uber.org/fx"
	"zeusro.com/hermes/internal/core/config"
	"zeusro.com/hermes/internal/core/webprovider"
	"zeusro.com/hermes/pkg/storage"
)

// Module 提供 API server 模块
var Module = fx.Options(
	fx.Invoke(func(fiberEngine webprovider.FiberEngine, cfg config.Config) {
		store, err := storage.NewStore(cfg.Storage)
		if err != nil {
			panic(fmt.Sprintf("Failed to create store: %v", err))
		}
		RegisterRoutes(fiberEngine, store)
	}),
)
