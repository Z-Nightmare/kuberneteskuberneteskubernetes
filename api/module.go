package api

import "go.uber.org/fx"

type Route interface {
	SetUp()
}

type Routes []Route

func (r Routes) SetUp() {
	for _, route := range r {
		route.SetUp()
	}
}

func NewRoutes(healthRoutes HealthRoutes) Routes {
	return Routes{
		healthRoutes,
	}
}

var Modules = fx.Options(
	fx.Provide(NewHealthRoutes))
