# 设计架构

## api层

api server。

## 控制器

- scheduler
- kubelet
- proxy
- container runtime

控制器包含了原始kubernetes中控制单元的最小实现。

启动控制器之后，会将当前节点上报，并存储到 node 单元。

## 存储

存储可以从 etcd，mysql，内存中任由选择。

### mysql

一种资源对应mysql一个表。字段只能追加不能减。