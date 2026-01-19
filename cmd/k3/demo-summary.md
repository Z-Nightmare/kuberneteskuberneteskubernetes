# Cluster Create 演示脚本说明

## 功能概述

演示脚本 `demo.sh` 实现了以下功能：

1. **创建集群配置**：使用 `cluster create` 命令创建单机模式（1个节点）的集群配置
2. **启动 k3 服务**：以 one 模式启动 k3，自动启动 Consul 容器
3. **提交 nginx deployment**：使用 `apply` 命令提交 nginx deployment
4. **验证 Pod 状态**：通过 API 查询 Pod 状态
5. **验证 Docker 容器**：检查 Docker 容器是否运行
6. **清除集群**：使用 `cluster clear` 清除集群配置和容器

## 使用方法

```bash
cd /Users/zeusro/code/kuberneteskuberneteskubernetes
./cmd/k3/demo.sh
```

## 已知问题

在验证过程中发现的问题：

1. **存储类型问题**：服务器端在处理 Deployment 时可能错误地使用了 MySQL 存储，即使配置是 memory
   - 错误信息：`failed to check table existence: dial tcp [::1]:3306: connect: connection refused`
   - 需要进一步调查存储类型判断逻辑

2. **服务启动时间**：Consul 容器启动需要时间，脚本已增加等待时间

## 已实现的功能

1. ✅ Consul 自动启动功能（discovery 模块）
2. ✅ 集群配置创建
3. ✅ 服务启动和等待逻辑
4. ✅ Deployment 提交逻辑
5. ✅ Pod 状态验证逻辑
6. ✅ Docker 容器验证逻辑
7. ✅ 集群清理逻辑

## 待修复的问题

1. ⚠️ 存储类型判断问题（需要进一步调查）
2. ⚠️ 服务启动超时处理（已优化，但可能需要进一步调整）
