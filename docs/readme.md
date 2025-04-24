

## 依赖注入的问题


### 问题1 

![alt text](<error.png>)

启动出错可以打断点看看 app.Run 里面的错误属性

```log
error(go.uber.org/dig.errInvalidInput) {Message: "must provide constructor function, got {0x140000ba740} (type logprovider.Logger)", Cause: error nil}
```

dig 要求你提供的是“构造函数”（constructor function），但你传的是一个具体的实例（如结构体或接口对象）。这是不合法的。

```go
// ❌ 错误：你传了一个实例
fx.Provide(logprovider.GetLogger())

// ✅ 正确：传递构造函数本身（不要加括号！）
fx.Provide(logprovider.GetLogger)
```

### 问题2

```log
error(go.uber.org/dig.errMissingTypes) [{Key: (*"go.uber.org/dig.key")(0x14000172cd0), suggestions: []go.uber.org/dig.key len: 0, cap: 0, nil}]
```

```go

type HealthRoutes struct {
	logger logprovider.Logger
	gin    webprovider.MyGinEngine
	s      service.HealthService
	// m middleware.JWTMiddleware
}

func NewHealthRoutes(logger logprovider.Logger, gin webprovider.MyGinEngine) HealthRoutes {
	return HealthRoutes{
		logger: logger,
		gin:    gin,
	}
}
```

结果是这里缺了注入参数。

### 问题3

```log
"Fx 启动失败: missing dependencies for function \"main\".StartGinServer (/Users/adam/code/go-template/cmd/web/main.go:97): missing type: api.Routes"
```

```go
var Modules = fx.Options(
	fx.Provide(NewHealthRoutes),
	fx.Provide(NewRoutes)) //写少了这一个
```


#### 问题4

```log
"main.main
	/Users/zeusro/code/go-template/cmd/web/main.go:38
	runtime.main
	/usr/local/go/src/runtime/proc.go:283"
```
