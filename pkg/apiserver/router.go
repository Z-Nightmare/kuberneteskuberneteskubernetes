package apiserver

import (
	"zeusro.com/hermes/internal/core/webprovider"
	"zeusro.com/hermes/pkg/storage"
)

// RegisterRoutes 注册 Kubernetes API server 路由
func RegisterRoutes(ginEngine webprovider.MyGinEngine, store storage.Store) {
	apiServer := NewAPIServer(store)

	// Core API v1
	coreV1 := ginEngine.Api.Group("/api/v1")
	{
		// Pods
		coreV1.GET("/pods", apiServer.HandleList)
		coreV1.GET("/pods/:name", apiServer.HandleGet)
		coreV1.POST("/pods", apiServer.HandleCreate)
		coreV1.PUT("/pods/:name", apiServer.HandleUpdate)
		coreV1.PATCH("/pods/:name", apiServer.HandlePatch)
		coreV1.DELETE("/pods/:name", apiServer.HandleDelete)
		coreV1.GET("/watch/pods", apiServer.HandleWatch)

		// Namespaced Pods
		coreV1.GET("/namespaces/:namespace/pods", apiServer.HandleList)
		coreV1.GET("/namespaces/:namespace/pods/:name", apiServer.HandleGet)
		coreV1.POST("/namespaces/:namespace/pods", apiServer.HandleCreate)
		coreV1.PUT("/namespaces/:namespace/pods/:name", apiServer.HandleUpdate)
		coreV1.PATCH("/namespaces/:namespace/pods/:name", apiServer.HandlePatch)
		coreV1.DELETE("/namespaces/:namespace/pods/:name", apiServer.HandleDelete)
		coreV1.GET("/watch/namespaces/:namespace/pods", apiServer.HandleWatch)

		// Services
		coreV1.GET("/services", apiServer.HandleList)
		coreV1.GET("/services/:name", apiServer.HandleGet)
		coreV1.POST("/services", apiServer.HandleCreate)
		coreV1.PUT("/services/:name", apiServer.HandleUpdate)
		coreV1.PATCH("/services/:name", apiServer.HandlePatch)
		coreV1.DELETE("/services/:name", apiServer.HandleDelete)
		coreV1.GET("/watch/services", apiServer.HandleWatch)

		// Namespaced Services
		coreV1.GET("/namespaces/:namespace/services", apiServer.HandleList)
		coreV1.GET("/namespaces/:namespace/services/:name", apiServer.HandleGet)
		coreV1.POST("/namespaces/:namespace/services", apiServer.HandleCreate)
		coreV1.PUT("/namespaces/:namespace/services/:name", apiServer.HandleUpdate)
		coreV1.PATCH("/namespaces/:namespace/services/:name", apiServer.HandlePatch)
		coreV1.DELETE("/namespaces/:namespace/services/:name", apiServer.HandleDelete)
		coreV1.GET("/watch/namespaces/:namespace/services", apiServer.HandleWatch)

		// ConfigMaps
		coreV1.GET("/configmaps", apiServer.HandleList)
		coreV1.GET("/configmaps/:name", apiServer.HandleGet)
		coreV1.POST("/configmaps", apiServer.HandleCreate)
		coreV1.PUT("/configmaps/:name", apiServer.HandleUpdate)
		coreV1.PATCH("/configmaps/:name", apiServer.HandlePatch)
		coreV1.DELETE("/configmaps/:name", apiServer.HandleDelete)
		coreV1.GET("/watch/configmaps", apiServer.HandleWatch)

		// Namespaced ConfigMaps
		coreV1.GET("/namespaces/:namespace/configmaps", apiServer.HandleList)
		coreV1.GET("/namespaces/:namespace/configmaps/:name", apiServer.HandleGet)
		coreV1.POST("/namespaces/:namespace/configmaps", apiServer.HandleCreate)
		coreV1.PUT("/namespaces/:namespace/configmaps/:name", apiServer.HandleUpdate)
		coreV1.PATCH("/namespaces/:namespace/configmaps/:name", apiServer.HandlePatch)
		coreV1.DELETE("/namespaces/:namespace/configmaps/:name", apiServer.HandleDelete)
		coreV1.GET("/watch/namespaces/:namespace/configmaps", apiServer.HandleWatch)

		// Secrets
		coreV1.GET("/secrets", apiServer.HandleList)
		coreV1.GET("/secrets/:name", apiServer.HandleGet)
		coreV1.POST("/secrets", apiServer.HandleCreate)
		coreV1.PUT("/secrets/:name", apiServer.HandleUpdate)
		coreV1.PATCH("/secrets/:name", apiServer.HandlePatch)
		coreV1.DELETE("/secrets/:name", apiServer.HandleDelete)
		coreV1.GET("/watch/secrets", apiServer.HandleWatch)

		// Namespaced Secrets
		coreV1.GET("/namespaces/:namespace/secrets", apiServer.HandleList)
		coreV1.GET("/namespaces/:namespace/secrets/:name", apiServer.HandleGet)
		coreV1.POST("/namespaces/:namespace/secrets", apiServer.HandleCreate)
		coreV1.PUT("/namespaces/:namespace/secrets/:name", apiServer.HandleUpdate)
		coreV1.PATCH("/namespaces/:namespace/secrets/:name", apiServer.HandlePatch)
		coreV1.DELETE("/namespaces/:namespace/secrets/:name", apiServer.HandleDelete)
		coreV1.GET("/watch/namespaces/:namespace/secrets", apiServer.HandleWatch)
	}

	// Apps API v1
	appsV1 := ginEngine.Api.Group("/apis/apps/v1")
	{
		// Deployments
		appsV1.GET("/deployments", apiServer.HandleList)
		appsV1.GET("/deployments/:name", apiServer.HandleGet)
		appsV1.POST("/deployments", apiServer.HandleCreate)
		appsV1.PUT("/deployments/:name", apiServer.HandleUpdate)
		appsV1.PATCH("/deployments/:name", apiServer.HandlePatch)
		appsV1.DELETE("/deployments/:name", apiServer.HandleDelete)
		appsV1.GET("/watch/deployments", apiServer.HandleWatch)

		// Namespaced Deployments
		appsV1.GET("/namespaces/:namespace/deployments", apiServer.HandleList)
		appsV1.GET("/namespaces/:namespace/deployments/:name", apiServer.HandleGet)
		appsV1.POST("/namespaces/:namespace/deployments", apiServer.HandleCreate)
		appsV1.PUT("/namespaces/:namespace/deployments/:name", apiServer.HandleUpdate)
		appsV1.PATCH("/namespaces/:namespace/deployments/:name", apiServer.HandlePatch)
		appsV1.DELETE("/namespaces/:namespace/deployments/:name", apiServer.HandleDelete)
		appsV1.GET("/watch/namespaces/:namespace/deployments", apiServer.HandleWatch)

		// StatefulSets
		appsV1.GET("/statefulsets", apiServer.HandleList)
		appsV1.GET("/statefulsets/:name", apiServer.HandleGet)
		appsV1.POST("/statefulsets", apiServer.HandleCreate)
		appsV1.PUT("/statefulsets/:name", apiServer.HandleUpdate)
		appsV1.PATCH("/statefulsets/:name", apiServer.HandlePatch)
		appsV1.DELETE("/statefulsets/:name", apiServer.HandleDelete)
		appsV1.GET("/watch/statefulsets", apiServer.HandleWatch)

		// Namespaced StatefulSets
		appsV1.GET("/namespaces/:namespace/statefulsets", apiServer.HandleList)
		appsV1.GET("/namespaces/:namespace/statefulsets/:name", apiServer.HandleGet)
		appsV1.POST("/namespaces/:namespace/statefulsets", apiServer.HandleCreate)
		appsV1.PUT("/namespaces/:namespace/statefulsets/:name", apiServer.HandleUpdate)
		appsV1.PATCH("/namespaces/:namespace/statefulsets/:name", apiServer.HandlePatch)
		appsV1.DELETE("/namespaces/:namespace/statefulsets/:name", apiServer.HandleDelete)
		appsV1.GET("/watch/namespaces/:namespace/statefulsets", apiServer.HandleWatch)

		// DaemonSets
		appsV1.GET("/daemonsets", apiServer.HandleList)
		appsV1.GET("/daemonsets/:name", apiServer.HandleGet)
		appsV1.POST("/daemonsets", apiServer.HandleCreate)
		appsV1.PUT("/daemonsets/:name", apiServer.HandleUpdate)
		appsV1.PATCH("/daemonsets/:name", apiServer.HandlePatch)
		appsV1.DELETE("/daemonsets/:name", apiServer.HandleDelete)
		appsV1.GET("/watch/daemonsets", apiServer.HandleWatch)

		// Namespaced DaemonSets
		appsV1.GET("/namespaces/:namespace/daemonsets", apiServer.HandleList)
		appsV1.GET("/namespaces/:namespace/daemonsets/:name", apiServer.HandleGet)
		appsV1.POST("/namespaces/:namespace/daemonsets", apiServer.HandleCreate)
		appsV1.PUT("/namespaces/:namespace/daemonsets/:name", apiServer.HandleUpdate)
		appsV1.PATCH("/namespaces/:namespace/daemonsets/:name", apiServer.HandlePatch)
		appsV1.DELETE("/namespaces/:namespace/daemonsets/:name", apiServer.HandleDelete)
		appsV1.GET("/watch/namespaces/:namespace/daemonsets", apiServer.HandleWatch)
	}
}
