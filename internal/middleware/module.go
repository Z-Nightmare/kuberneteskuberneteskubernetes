package middleware

import "go.uber.org/fx"

type Middlewares []IMiddleware

type IMiddleware interface {
	SetUp()
}

func NewMiddlewares() Middlewares {
	return Middlewares{}
}

func (m Middlewares) SetUp() {
	for _, middleware := range m {
		middleware.SetUp()
	}
}

var Module = fx.Options(
	// fx.Provide(NewCorsMiddleware),
	// fx.Provide(NewJWTMiddleware),
	fx.Provide(NewMiddlewares),
)
