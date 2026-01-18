package api

import (
	"os"
	"path/filepath"

	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/logprovider"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/webprovider"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/service"
	"github.com/gofiber/fiber/v2"
)

type IndexRoutes struct {
	logger logprovider.Logger
	fiber  webprovider.FiberEngine
	health service.HealthService
	hermes service.TranslateService
	// m middleware.JWTMiddleware
}

func NewIndexRoutes(logger logprovider.Logger, fiber webprovider.FiberEngine,
	s service.HealthService, herms service.TranslateService) IndexRoutes {
	return IndexRoutes{
		logger: logger,
		fiber:  fiber,
		health: s,
		hermes: herms,
	}
}

func (r IndexRoutes) SetUp() {
	indexHTML := resolveStaticFilePath("index.html")
	translateHTML := resolveStaticFilePath("translate.html")

	r.fiber.App.Get("/index", func(c *fiber.Ctx) error {
		return c.SendFile(indexHTML)
	})

	r.fiber.App.Get("/translate", func(c *fiber.Ctx) error {
		return c.SendFile(translateHTML)
	})
	r.fiber.App.Post("/translate", r.hermes.Translate)

	index := r.fiber.App.Group("/api")
	{
		//http://localhost:8080/api/health
		index.Options("/health", r.health.CheckFiber)
		index.Get("/health", r.health.CheckFiber)
		index.Options("/healthz", r.health.CheckFiber)
		index.Get("/healthz", r.health.CheckFiber)
	}
}

func resolveStaticFilePath(filename string) string {
	// 优先使用环境变量（便于容器化/多实例部署）
	if dir := os.Getenv("STATIC_DIR"); dir != "" {
		return filepath.Join(dir, filename)
	}

	// 默认按“从项目根目录启动”的方式寻找静态资源目录
	candidates := []string{
		filepath.Join("cmd", "apiserver", "static"), // 新目录
		filepath.Join("cmd", "web", "static"),       // 兼容旧目录（迁移期间）
		"static",                                    // 兼容从模块目录启动（如 cd cmd/apiserver && go run .）
	}

	for _, dir := range candidates {
		p := filepath.Join(dir, filename)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// 最后兜底：让 Fiber 自己报错（路径依旧是相对路径）
	return filepath.Join("static", filename)
}
