package apiserver

import (
	"go.uber.org/fx"
	"zeusro.com/hermes/internal/core/webprovider"
	"zeusro.com/hermes/pkg/storage"
)

// Module 提供 API server 模块
var Module = fx.Options(
	fx.Invoke(func(ginEngine webprovider.MyGinEngine) {
		store := storage.NewMemoryStore()
		RegisterRoutes(ginEngine, store)
	}),
)
