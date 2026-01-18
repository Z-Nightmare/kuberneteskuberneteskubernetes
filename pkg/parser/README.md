# Kubernetes YAML Parser

这是一个基于 Kubernetes 官方 YAML 解析引擎的解析器，支持解析所有 Kubernetes 原生资源对象。

## 功能特性

- ✅ 支持解析所有 Kubernetes 原生资源类型（Pod、Deployment、Service、ConfigMap、Secret 等）
- ✅ 支持解析多文档 YAML manifest（使用 `---` 分隔符）
- ✅ 自动识别资源类型和 GroupVersionKind
- ✅ 支持 YAML 序列化和反序列化
- ✅ 使用 Kubernetes 官方的 scheme 和 codec

## 使用方法

### 解析单个 YAML 文档

```go
package main

import (
    "fmt"
    "zeusro.com/hermes/pkg/parser"
)

func main() {
    yamlData := `
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  namespace: default
spec:
  containers:
  - name: nginx
    image: nginx:1.21
`

    p := parser.NewParser()
    obj, gvk, err := p.ParseYAML([]byte(yamlData))
    if err != nil {
        panic(err)
    }

    fmt.Printf("Kind: %s\n", gvk.Kind)
    fmt.Printf("Group: %s\n", gvk.Group)
    fmt.Printf("Version: %s\n", gvk.Version)
    // obj 是 runtime.Object 类型，可以进行类型断言
}
```

### 解析多文档 YAML Manifest

```go
package main

import (
    "fmt"
    "zeusro.com/hermes/pkg/parser"
)

func main() {
    manifestYAML := `
apiVersion: v1
kind: Pod
metadata:
  name: pod1
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deployment1
`

    p := parser.NewParser()
    objects, gvks, err := p.ParseYAMLManifest([]byte(manifestYAML))
    if err != nil {
        panic(err)
    }

    fmt.Printf("Parsed %d objects\n", len(objects))
    for i, obj := range objects {
        fmt.Printf("Object %d: %s\n", i+1, gvks[i].Kind)
    }
}
```

### 从文件解析

```go
p := parser.NewParser()
objects, gvks, err := p.ParseYAMLFile("/path/to/manifest.yaml")
if err != nil {
    panic(err)
}
```

### 序列化为 YAML

```go
import (
    appsv1 "k8s.io/api/apps/v1"
    "zeusro.com/hermes/pkg/parser"
)

deployment := &appsv1.Deployment{
    // ... 设置 deployment 字段
}

yamlData, err := parser.ToYAML(deployment)
if err != nil {
    panic(err)
}

fmt.Println(string(yamlData))
```

## 支持的原生资源类型

该解析器支持所有 Kubernetes 原生资源类型，包括但不限于：

- **Core Resources**: Pod, Service, ConfigMap, Secret, Namespace, PersistentVolume, PersistentVolumeClaim 等
- **Apps Resources**: Deployment, StatefulSet, DaemonSet, ReplicaSet 等
- **Networking Resources**: Ingress, NetworkPolicy 等
- **Storage Resources**: StorageClass, VolumeAttachment 等
- **RBAC Resources**: Role, RoleBinding, ClusterRole, ClusterRoleBinding 等
- **其他所有标准 Kubernetes API 资源**

## 技术实现

- 使用 `k8s.io/client-go/kubernetes/scheme` 的标准 scheme
- 使用 `k8s.io/apimachinery/pkg/runtime` 的 UniversalDeserializer
- 支持自动类型识别和转换
- 完全兼容 Kubernetes 官方 API 规范

## 依赖

- `k8s.io/api`: Kubernetes API 类型定义
- `k8s.io/apimachinery`: Kubernetes 核心 machinery（runtime、serializer 等）
- `k8s.io/client-go`: Kubernetes 客户端库（包含 scheme）
