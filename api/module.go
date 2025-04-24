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

func NewRoutes(indexRoutes IndexRoutes) Routes {
	return Routes{
		indexRoutes,
	}
}

var Modules = fx.Options(
	fx.Provide(NewIndexRoutes),
	fx.Provide(NewRoutes))
