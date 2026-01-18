package service

import (
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/config"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/logprovider"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/webprovider"
	"github.com/gofiber/fiber/v2"
)

func NewHealthService(fiber webprovider.FiberEngine, l logprovider.Logger,
	config config.Config) HealthService {
	return HealthService{
		fiber:  fiber,
		l:      l,
		config: config,
	}
}

type HealthService struct {
	fiber  webprovider.FiberEngine
	l      logprovider.Logger
	config config.Config
}

func (s HealthService) Check(ctx *fiber.Ctx) error {
	return s.CheckFiber(ctx)
}

// CheckFiber is the Fiber version
func (s HealthService) CheckFiber(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"code": 200,
	})
}
