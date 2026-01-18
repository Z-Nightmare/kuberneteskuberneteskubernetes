# Changelog

## 2025-01-XX - Kubernetes YAML Parser 集成

### 新增功能

1. **Kubernetes YAML 解析器** (`pkg/parser`)
   - 支持解析所有 Kubernetes 原生资源类型
   - 支持单文档和多文档 YAML manifest 解析
   - 自动识别 GroupVersionKind
   - YAML 序列化和反序列化支持

2. **升级到 Go 1.25**
   - 更新 `go.mod` 中的 Go 版本
   - 所有依赖已更新并兼容

3. **Kubernetes 依赖集成**
   - `k8s.io/api v0.35.0` - Kubernetes API 类型定义
   - `k8s.io/apimachinery v0.35.0` - 核心 machinery（runtime、serializer）
   - `k8s.io/client-go v0.35.0` - 客户端库（包含 scheme）

### 目录结构

借鉴 Kubernetes 项目的目录编排方式：

```
pkg/
  └── parser/          # YAML 解析器模块
      ├── yaml.go      # 核心解析逻辑
      ├── yaml_test.go # 测试用例
      └── README.md    # 使用文档
```

### 测试覆盖

- ✅ Pod 解析测试
- ✅ Deployment 解析测试
- ✅ 多文档 Manifest 解析测试
- ✅ YAML 序列化测试
- ✅ 原生资源类型测试（ConfigMap、Secret、Namespace、Service、StatefulSet、DaemonSet）

### 技术实现

- 使用 `k8s.io/client-go/kubernetes/scheme` 的标准 scheme
- 使用 `UniversalDeserializer` 自动识别和解析资源类型
- 支持所有 Kubernetes 原生 API 资源类型
- 完全兼容 Kubernetes 官方 API 规范
