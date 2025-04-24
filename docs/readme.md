

## 

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