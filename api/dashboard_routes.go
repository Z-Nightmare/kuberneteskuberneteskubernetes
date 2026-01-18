package api

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/logprovider"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/webprovider"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
	"go.uber.org/fx"
)

type DashboardRoutes struct {
	logger logprovider.Logger
	fiber  webprovider.FiberEngine
	hub    *ResourceHub
}

func NewDashboardRoutes(
	logger logprovider.Logger,
	fiber webprovider.FiberEngine,
	hub *ResourceHub,
) DashboardRoutes {
	return DashboardRoutes{
		logger: logger,
		fiber:  fiber,
		hub:    hub,
	}
}

func (r DashboardRoutes) SetUp() {
	dashboardHTML := resolveWebStaticFilePath("dashboard.html")

	r.fiber.App.Get("/", func(c *fiber.Ctx) error {
		return c.SendFile(dashboardHTML)
	})
	r.fiber.App.Get("/dashboard", func(c *fiber.Ctx) error {
		return c.SendFile(dashboardHTML)
	})

	// WebSocket endpoint for live resource updates.
	r.fiber.App.Get("/ws/resources", websocket.New(func(c *websocket.Conn) {
		id := uuid.NewString()
		ch, unsubscribe := r.hub.Subscribe(id)
		defer unsubscribe()

		// Send initial snapshot.
		if payload, err := r.hub.SnapshotJSON(); err == nil {
			_ = c.WriteMessage(websocket.TextMessage, payload)
		} else {
			_ = c.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"type":"snapshot","generatedAt":"%s","nodes":[],"pods":[],"counts":{"nodes":0,"pods":0},"error":{"message":"%s"}}`, time.Now().Format(time.RFC3339), err.Error())))
		}

		// Keep a reader running so we detect client close quickly.
		readDone := make(chan struct{})
		go func() {
			defer close(readDone)
			for {
				if _, _, err := c.ReadMessage(); err != nil {
					return
				}
			}
		}()

		for {
			select {
			case <-readDone:
				return
			case payload, ok := <-ch:
				if !ok {
					return
				}
				if err := c.WriteMessage(websocket.TextMessage, payload); err != nil {
					return
				}
			}
		}
	}))
}

func resolveWebStaticFilePath(filename string) string {
	// 优先使用环境变量（便于容器化/多实例部署）
	if dir := os.Getenv("STATIC_DIR"); dir != "" {
		return filepath.Join(dir, filename)
	}

	// 默认优先 cmd/web/static（本模块）
	candidates := []string{
		filepath.Join("cmd", "web", "static"),
		filepath.Join("cmd", "apiserver", "static"), // 兼容（方便复用）
		"static", // 兼容从模块目录启动
	}
	for _, dir := range candidates {
		p := filepath.Join(dir, filename)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return filepath.Join("static", filename)
}

// DashboardModule wires hub lifecycle start.
var DashboardModule = fx.Module("dashboard",
	fx.Provide(NewResourceHub),
	fx.Provide(NewDashboardRoutes),
	fx.Invoke(func(lc fx.Lifecycle, hub *ResourceHub) {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				hub.Start(ctx)
				return nil
			},
		})
	}),
)
