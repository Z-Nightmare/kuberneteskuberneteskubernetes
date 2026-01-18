package api

import (
	"github.com/gofiber/fiber/v2"
	"zeusro.com/hermes/internal/core/logprovider"
	"zeusro.com/hermes/internal/core/webprovider"
	"zeusro.com/hermes/internal/service"
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
	r.fiber.App.Get("/index", func(c *fiber.Ctx) error {
		return c.SendFile("./static/index.html")
	})

	r.fiber.App.Get("/translate", func(c *fiber.Ctx) error {
		return c.SendFile("./static/translate.html")
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
