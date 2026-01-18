# go-template

	幸福的人生总是类似 悲剧的家庭却各有不同

## 介绍

![img](docs/png-clipart-gundam-head-illustration-zgmf-x10a-freedom-gundam-drawing-zgmf-x20a-strike-freedom-line-art-seeds-miscellaneous-symmetry.png)

好的设计，就像高达，你从头部设计的细节就知道成品质量会怎么样

普通版本的自由2.0:
![img](docs/967ab04f4a947db97b22c2cd6ffb24b7.jpg)

mgex 版本的自由:
![img](docs/sec04obj02.png)

## 核心概念

### Module

配置，数据库，日志，service，中间件，第三方服务，应用接口，一切皆为模块（module）。

在模块里面里面隐藏了接口的概念。所有的业务实现，本质上都是实现接口。

```go
type People interface {
	Bullshit() string
}
```

如果是以前那些 java 没学好的同事，估计会这样写：

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

他们给我的感觉就好像“我已经把屎拉好了，他们还在封装纸尿裤”。
这类行为的本质都是凡人在放屁。其实对我来讲就只有一个:

```go
func (p People) Bullshit() string {
	return p.Thought
}
```

如果你封装能力真的好，甚至不用定义 interface ，也能让所有实现符合完全一致的范式。

### Function

本来我觉得文档定义1个概念（module）就够了。但我对于函数的理解，跟一些人的认知有偏差，所以我就稍微讲解一下。

函数（Function）是解决问题的一种方法论。
函数只有3类公民：输入（input），计算（compute），输出（output）；
换一种分类思维，函数也分局部函数（Local Function）和云函数（Cloud Function）。

打车就是云端函数，看时间就是本地函数。
本地时间跟服务器有时候不在一个时区，所以询问时间的时间函数，没有必要在云端运行。

#### Local Function

有些函数带有局限性，没必要云端执行，就放在本地。
比如 Android api，ios api , Window api，这一类函数基本依赖硬件，所以建议在本地运行；

#### Cloud Function

云端函数脱离了本地环境，需要比较大的计算资源，所以叫云端函数。
