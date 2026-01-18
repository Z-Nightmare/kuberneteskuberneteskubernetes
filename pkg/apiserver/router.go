package apiserver

import (
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/webprovider"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/pkg/storage"
)

// RegisterRoutes 注册 Kubernetes API server 路由
func RegisterRoutes(fiberEngine webprovider.FiberEngine, store storage.Store) {
	apiServer := NewAPIServer(store)

	// Core API v1
	coreV1 := fiberEngine.Api.Group("/api/v1")
	{
		// Pods
		coreV1.Get("/pods", apiServer.HandleList)
		coreV1.Get("/pods/:name", apiServer.HandleGet)
		coreV1.Post("/pods", apiServer.HandleCreate)
		coreV1.Put("/pods/:name", apiServer.HandleUpdate)
		coreV1.Patch("/pods/:name", apiServer.HandlePatch)
		coreV1.Delete("/pods/:name", apiServer.HandleDelete)
		coreV1.Get("/watch/pods", apiServer.HandleWatch)

		// Namespaced Pods
		coreV1.Get("/namespaces/:namespace/pods", apiServer.HandleList)
		coreV1.Get("/namespaces/:namespace/pods/:name", apiServer.HandleGet)
		coreV1.Post("/namespaces/:namespace/pods", apiServer.HandleCreate)
		coreV1.Put("/namespaces/:namespace/pods/:name", apiServer.HandleUpdate)
		coreV1.Patch("/namespaces/:namespace/pods/:name", apiServer.HandlePatch)
		coreV1.Delete("/namespaces/:namespace/pods/:name", apiServer.HandleDelete)
		coreV1.Get("/watch/namespaces/:namespace/pods", apiServer.HandleWatch)

		// Services
		coreV1.Get("/services", apiServer.HandleList)
		coreV1.Get("/services/:name", apiServer.HandleGet)
		coreV1.Post("/services", apiServer.HandleCreate)
		coreV1.Put("/services/:name", apiServer.HandleUpdate)
		coreV1.Patch("/services/:name", apiServer.HandlePatch)
		coreV1.Delete("/services/:name", apiServer.HandleDelete)
		coreV1.Get("/watch/services", apiServer.HandleWatch)

		// Namespaced Services
		coreV1.Get("/namespaces/:namespace/services", apiServer.HandleList)
		coreV1.Get("/namespaces/:namespace/services/:name", apiServer.HandleGet)
		coreV1.Post("/namespaces/:namespace/services", apiServer.HandleCreate)
		coreV1.Put("/namespaces/:namespace/services/:name", apiServer.HandleUpdate)
		coreV1.Patch("/namespaces/:namespace/services/:name", apiServer.HandlePatch)
		coreV1.Delete("/namespaces/:namespace/services/:name", apiServer.HandleDelete)
		coreV1.Get("/watch/namespaces/:namespace/services", apiServer.HandleWatch)

		// ConfigMaps
		coreV1.Get("/configmaps", apiServer.HandleList)
		coreV1.Get("/configmaps/:name", apiServer.HandleGet)
		coreV1.Post("/configmaps", apiServer.HandleCreate)
		coreV1.Put("/configmaps/:name", apiServer.HandleUpdate)
		coreV1.Patch("/configmaps/:name", apiServer.HandlePatch)
		coreV1.Delete("/configmaps/:name", apiServer.HandleDelete)
		coreV1.Get("/watch/configmaps", apiServer.HandleWatch)

		// Namespaced ConfigMaps
		coreV1.Get("/namespaces/:namespace/configmaps", apiServer.HandleList)
		coreV1.Get("/namespaces/:namespace/configmaps/:name", apiServer.HandleGet)
		coreV1.Post("/namespaces/:namespace/configmaps", apiServer.HandleCreate)
		coreV1.Put("/namespaces/:namespace/configmaps/:name", apiServer.HandleUpdate)
		coreV1.Patch("/namespaces/:namespace/configmaps/:name", apiServer.HandlePatch)
		coreV1.Delete("/namespaces/:namespace/configmaps/:name", apiServer.HandleDelete)
		coreV1.Get("/watch/namespaces/:namespace/configmaps", apiServer.HandleWatch)

		// Secrets
		coreV1.Get("/secrets", apiServer.HandleList)
		coreV1.Get("/secrets/:name", apiServer.HandleGet)
		coreV1.Post("/secrets", apiServer.HandleCreate)
		coreV1.Put("/secrets/:name", apiServer.HandleUpdate)
		coreV1.Patch("/secrets/:name", apiServer.HandlePatch)
		coreV1.Delete("/secrets/:name", apiServer.HandleDelete)
		coreV1.Get("/watch/secrets", apiServer.HandleWatch)

		// Namespaced Secrets
		coreV1.Get("/namespaces/:namespace/secrets", apiServer.HandleList)
		coreV1.Get("/namespaces/:namespace/secrets/:name", apiServer.HandleGet)
		coreV1.Post("/namespaces/:namespace/secrets", apiServer.HandleCreate)
		coreV1.Put("/namespaces/:namespace/secrets/:name", apiServer.HandleUpdate)
		coreV1.Patch("/namespaces/:namespace/secrets/:name", apiServer.HandlePatch)
		coreV1.Delete("/namespaces/:namespace/secrets/:name", apiServer.HandleDelete)
		coreV1.Get("/watch/namespaces/:namespace/secrets", apiServer.HandleWatch)
	}

	// Apps API v1
	appsV1 := fiberEngine.Api.Group("/apis/apps/v1")
	{
		// Deployments
		appsV1.Get("/deployments", apiServer.HandleList)
		appsV1.Get("/deployments/:name", apiServer.HandleGet)
		appsV1.Post("/deployments", apiServer.HandleCreate)
		appsV1.Put("/deployments/:name", apiServer.HandleUpdate)
		appsV1.Patch("/deployments/:name", apiServer.HandlePatch)
		appsV1.Delete("/deployments/:name", apiServer.HandleDelete)
		appsV1.Get("/watch/deployments", apiServer.HandleWatch)

		// Namespaced Deployments
		appsV1.Get("/namespaces/:namespace/deployments", apiServer.HandleList)
		appsV1.Get("/namespaces/:namespace/deployments/:name", apiServer.HandleGet)
		appsV1.Post("/namespaces/:namespace/deployments", apiServer.HandleCreate)
		appsV1.Put("/namespaces/:namespace/deployments/:name", apiServer.HandleUpdate)
		appsV1.Patch("/namespaces/:namespace/deployments/:name", apiServer.HandlePatch)
		appsV1.Delete("/namespaces/:namespace/deployments/:name", apiServer.HandleDelete)
		appsV1.Get("/watch/namespaces/:namespace/deployments", apiServer.HandleWatch)

		// StatefulSets
		appsV1.Get("/statefulsets", apiServer.HandleList)
		appsV1.Get("/statefulsets/:name", apiServer.HandleGet)
		appsV1.Post("/statefulsets", apiServer.HandleCreate)
		appsV1.Put("/statefulsets/:name", apiServer.HandleUpdate)
		appsV1.Patch("/statefulsets/:name", apiServer.HandlePatch)
		appsV1.Delete("/statefulsets/:name", apiServer.HandleDelete)
		appsV1.Get("/watch/statefulsets", apiServer.HandleWatch)

		// Namespaced StatefulSets
		appsV1.Get("/namespaces/:namespace/statefulsets", apiServer.HandleList)
		appsV1.Get("/namespaces/:namespace/statefulsets/:name", apiServer.HandleGet)
		appsV1.Post("/namespaces/:namespace/statefulsets", apiServer.HandleCreate)
		appsV1.Put("/namespaces/:namespace/statefulsets/:name", apiServer.HandleUpdate)
		appsV1.Patch("/namespaces/:namespace/statefulsets/:name", apiServer.HandlePatch)
		appsV1.Delete("/namespaces/:namespace/statefulsets/:name", apiServer.HandleDelete)
		appsV1.Get("/watch/namespaces/:namespace/statefulsets", apiServer.HandleWatch)

		// DaemonSets
		appsV1.Get("/daemonsets", apiServer.HandleList)
		appsV1.Get("/daemonsets/:name", apiServer.HandleGet)
		appsV1.Post("/daemonsets", apiServer.HandleCreate)
		appsV1.Put("/daemonsets/:name", apiServer.HandleUpdate)
		appsV1.Patch("/daemonsets/:name", apiServer.HandlePatch)
		appsV1.Delete("/daemonsets/:name", apiServer.HandleDelete)
		appsV1.Get("/watch/daemonsets", apiServer.HandleWatch)

		// Namespaced DaemonSets
		appsV1.Get("/namespaces/:namespace/daemonsets", apiServer.HandleList)
		appsV1.Get("/namespaces/:namespace/daemonsets/:name", apiServer.HandleGet)
		appsV1.Post("/namespaces/:namespace/daemonsets", apiServer.HandleCreate)
		appsV1.Put("/namespaces/:namespace/daemonsets/:name", apiServer.HandleUpdate)
		appsV1.Patch("/namespaces/:namespace/daemonsets/:name", apiServer.HandlePatch)
		appsV1.Delete("/namespaces/:namespace/daemonsets/:name", apiServer.HandleDelete)
		appsV1.Get("/watch/namespaces/:namespace/daemonsets", apiServer.HandleWatch)
	}
}
