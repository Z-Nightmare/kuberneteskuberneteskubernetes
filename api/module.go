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

func NewRoutes(indexRoutes IndexRoutes, dashboardRoutes DashboardRoutes) Routes {
	return Routes{
		indexRoutes,
		dashboardRoutes,
	}
}

var Modules = fx.Options(
	fx.Provide(NewIndexRoutes),
	DashboardModule,
	fx.Provide(NewRoutes),
)
