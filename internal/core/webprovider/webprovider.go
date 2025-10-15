package webprovider

import (
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"runtime/debug"
	"strings"
	"time"

	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"zeusro.com/hermes/internal/core/config"
	"zeusro.com/hermes/internal/core/logprovider"
	"zeusro.com/hermes/internal/middleware"
)

type MyGinEngine struct {
	Gin *gin.Engine
	Api *gin.RouterGroup
}

func NewGinEngine(config config.Config) MyGinEngine {
	if config.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	engine := gin.New()
	//todo 按需按配置加载中间件
	zapLogger := logprovider.GetZapLogger()
	engine.Use(middleware.TraceIDMiddleware(zapLogger))
	engine.Use(ginzap.Ginzap(zapLogger, time.RFC3339, true))
	// engine.Use(ginzap.RecoveryWithZap(zapLogger, true))
	engine.Use(RecoveryMiddleware(zapLogger, true))
	return MyGinEngine{
		Gin: engine,
		Api: engine.Group("/api"),
	}
}

func RecoveryMiddleware(logger *zap.Logger, stack bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Check for a broken connection, as it is not really a
				// condition that warrants a panic stack trace.
				var brokenPipe bool
				if ne, ok := err.(*net.OpError); ok {
					if se, ok := ne.Err.(*os.SyscallError); ok {
						if strings.Contains(strings.ToLower(se.Error()), "broken pipe") || strings.Contains(strings.ToLower(se.Error()), "connection reset by peer") {
							brokenPipe = true
						}
					}
				}

				httpRequest, _ := httputil.DumpRequest(c.Request, false)
				if brokenPipe {
					logger.Error(c.Request.URL.Path,
						zap.Any("error", err),
						zap.String("request", string(httpRequest)),
					)
					// If the connection is dead, we can't write a status to it.
					c.Error(err.(error)) // nolint: errcheck
					c.Abort()
					return
				}

				if stack {
					logger.Error("[Recovery from panic]",
						zap.Time("time", time.Now()),
						zap.Any("error", err),
						zap.String("request", string(httpRequest)),
						//zap.String("stack"),
					)
					logger.Sugar().Error("[panic stack]: %s", string(debug.Stack()))
				} else {
					logger.Error("[Recovery from panic]",
						zap.Time("time", time.Now()),
						zap.Any("error", err),
						zap.String("request", string(httpRequest)),
					)
				}
				c.JSON(http.StatusInternalServerError, gin.H{
					"code":    http.StatusInternalServerError,
					"message": "Internal Server Error",
					"payload": err,
				})
				// 终止请求处理
				c.Abort()
			}
		}()
		c.Next()
	}
}
