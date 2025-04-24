## File Tree

```file
.
├── LICENSE
├── Makefile
├── README.md
├── config
│   └── config-example.yaml
├── deploy
│   ├── docker
│   │   └── Dockerfile
│   └── kubernetes
│       └── app.yaml
├── docker-compose.yaml
└── main.go
```

## 核心概念

### Module

配置，数据库，日志，service，中间件，第三方服务，应用接口，一切皆为模块（module）。

在模块里面里面隐藏了接口的概念。所有的业务实现，本质上都是实现接口。

```go
type People interface {
	Bullshit() string
}

```

如果是我以前那个java同事，他估计会这样写：

```go
func (p Teacher) Bullshit() string {
	return p.Thought
}

func (p Colleague) Bullshit() string {
	return p.Thought
}

func (p Boss) Bullshit() string {
	return p.Thought
}
```

老师放屁，同事放屁，老板放屁等 Module 实现 ，属于过度设计。
因为这类行为的本质都是凡人在放屁。其实对我来讲就只有一个:

```go
func (p People) Bullshit() string {
	return p.Thought
}
```

### Function

本来我觉得文档定义1个概念（module）就够了。但我对于函数的理解，跟一些人的认知有偏差，所以我就稍微讲解一下。都有代码例子，可以自己看看。

字面意思，函数（Function）就是解决问题的一种方法论。
函数只有3类公民：输入（input），计算（compute），输出（output）；
换一种分类思维，函数也分局部函数（Local Function）和云函数（Cloud Function）。

举个例子，打车就是云端函数，看时间就是本地函数。
本地时间跟服务器有时候不在一个时区，所以询问时间的时间函数，没有必要在云端运行。

#### Local Function

有些函数带有局限性，没必要云端执行，就放在本地。
比如 Android api，ios api , Window api，这一类函数基本依赖硬件，所以建议在本地运行；


#### Cloud Function

云端函数脱离了本地环境，需要比较大的计算资源，所以叫云端函数。

## 杂谈

接口与实现的关系，有一种神话含义，只要你实现（implement）了与神明的“约定（interface）”，你就可以获得TA给你的礼物（output），当然，0也是一种结果。

我以前有个中山大学的同学，问过我哲学的终极三大命题：我从哪里来？我是谁？我要去哪里？

这个问题我现在的回答是这样的：我不知道我是谁，所以我要踏上寻找自我的旅程，当我找到“我是谁”的答案之后，届时我就会知道要去哪里。而我来时的路便是第一个问题的答案。

```
Input --> Compute --> Output
```