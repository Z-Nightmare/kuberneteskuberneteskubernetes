package controller

import (
	"go.uber.org/fx"
)

// Module 提供控制器模块
var Module = fx.Module("controller",
	fx.Provide(
		NewControllerManager,
	),
)