#!/bin/bash

# 演示脚本：单机模式演示 cluster create 流程，使用 nginx 镜像
# 用法: ./demo.sh

set -e

CLUSTER_DIR=".k3"
NGINX_DEPLOYMENT="nginx-deployment.yaml"
API_SERVER="http://localhost:8080"
CONFIG_PATH="${CLUSTER_DIR}/node-1/.config.yaml"

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

echo_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

echo_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

echo_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

# 清理函数
cleanup() {
    echo_info "清理中..."
    if [ -n "$K3_PID" ]; then
        echo_info "停止 k3 进程 (PID: $K3_PID)"
        kill $K3_PID 2>/dev/null || true
        wait $K3_PID 2>/dev/null || true
    fi
}

trap cleanup EXIT INT TERM

# 步骤 1: 创建集群配置
echo_step "步骤 1: 创建集群配置（单机模式，1个节点）"
if [ -d "$CLUSTER_DIR" ]; then
    echo_warn "集群目录已存在，先清理..."
    go run ./cmd/k3 cluster clear --dir "$CLUSTER_DIR" --force || true
    sleep 1
fi

go run ./cmd/k3 cluster create --dir "$CLUSTER_DIR" --nodes 1 --web-port 8080 --storage memory
if [ ! -f "$CONFIG_PATH" ]; then
    echo_error "配置文件未生成: $CONFIG_PATH"
    exit 1
fi
echo_info "✓ 集群配置已创建"

# 修改配置为 one 模式
if ! grep -q "role: one" "$CONFIG_PATH"; then
    # 在 web: 之前添加 role: one
    sed -i.bak '/^web:/i\
role: one
' "$CONFIG_PATH"
    rm -f "${CONFIG_PATH}.bak"
    echo_info "✓ 已设置 role: one"
fi

# 步骤 2: 启动 k3（one 模式）
echo_step "步骤 2: 启动 k3（one 模式）"
echo_info "配置文件: $CONFIG_PATH"

# 启动 k3 在后台
echo_info "启动 k3 服务（Consul 将自动启动）..."
CONFIG_PATH="$CONFIG_PATH" AUTO_START_CONSUL=true go run ./cmd/k3 run > /tmp/k3.log 2>&1 &
K3_PID=$!
echo_info "k3 进程 PID: $K3_PID"

# 等待服务启动
echo_info "等待服务启动（最多 90 秒）..."
for i in {1..90}; do
    # 检查 API Server 是否响应
    if curl -s -f "$API_SERVER/api/v1/pods" > /dev/null 2>&1; then
        echo_info "✓ API Server 已启动"
        # 再等待 2 秒确保服务完全就绪
        sleep 2
        break
    fi
    if [ $i -eq 90 ]; then
        echo_error "API Server 启动超时"
        echo_error "日志内容:"
        tail -50 /tmp/k3.log
        exit 1
    fi
    if [ $((i % 10)) -eq 0 ]; then
        echo_info "等待中... ($i/90)"
    fi
    sleep 1
done

# 步骤 3: 创建 nginx deployment YAML
echo_step "步骤 3: 创建 nginx deployment"
if [ ! -f "$NGINX_DEPLOYMENT" ]; then
    cat > "$NGINX_DEPLOYMENT" <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  namespace: default
  labels:
    app: nginx
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:latest
        ports:
        - containerPort: 80
          name: http
          protocol: TCP
EOF
fi
echo_info "✓ nginx deployment YAML: $NGINX_DEPLOYMENT"

# 步骤 4: 提交 deployment
echo_step "步骤 4: 提交 nginx deployment"
# apply 命令只需要发送 HTTP 请求，不需要读取配置，但为了保持一致，我们仍然设置 CONFIG_PATH
export CONFIG_PATH="$CONFIG_PATH"

# 重试提交 deployment（最多 3 次）
for retry in {1..3}; do
    if go run ./cmd/k3 apply -f "$NGINX_DEPLOYMENT" --server "$API_SERVER" 2>&1; then
        echo_info "✓ deployment 已提交"
        break
    else
        if [ $retry -eq 3 ]; then
            echo_error "提交 deployment 失败（已重试 3 次）"
            echo_error "检查服务器日志:"
            tail -30 /tmp/k3.log
            exit 1
        fi
        echo_warn "提交失败，等待 3 秒后重试 ($retry/3)..."
        sleep 3
    fi
done

# 步骤 5: 等待 deployment 创建 pod
echo_step "步骤 5: 等待 deployment 创建 pod（最多 90 秒）..."
POD_NAME=""
for i in {1..90}; do
    sleep 2
    # 查询 pods
    PODS_RESPONSE=$(curl -s "$API_SERVER/api/v1/namespaces/default/pods" || echo "")
    if [ -n "$PODS_RESPONSE" ]; then
        # 尝试提取 pod 名称（使用 jq 如果可用，否则用 grep）
        if command -v jq &> /dev/null; then
            POD_NAME=$(echo "$PODS_RESPONSE" | jq -r '.items[]? | select(.metadata.name | contains("nginx")) | .metadata.name' | head -1 || echo "")
        else
            # 使用 grep 和 sed 提取
            POD_NAME=$(echo "$PODS_RESPONSE" | grep -o '"name":"[^"]*nginx[^"]*"' | head -1 | sed 's/"name":"\(.*\)"/\1/' || echo "")
        fi
        if [ -n "$POD_NAME" ]; then
            echo_info "✓ 找到 Pod: $POD_NAME"
            break
        fi
    fi
    if [ $i -eq 90 ]; then
        echo_error "等待 Pod 创建超时"
        echo_error "当前 Pods:"
        if command -v jq &> /dev/null; then
            echo "$PODS_RESPONSE" | jq '.'
        else
            echo "$PODS_RESPONSE"
        fi
        exit 1
    fi
done

if [ -z "$POD_NAME" ]; then
    echo_error "未找到 Pod 名称"
    exit 1
fi

# 步骤 6: 验证 pod 状态
echo_step "步骤 6: 验证 Pod 状态"
echo_info "查询 Pod 详情: $POD_NAME"
POD_JSON=$(curl -s "$API_SERVER/api/v1/namespaces/default/pods/$POD_NAME")
if [ -z "$POD_JSON" ]; then
    echo_error "无法获取 Pod 信息"
    exit 1
fi

# 检查 Pod 状态
if command -v jq &> /dev/null; then
    POD_PHASE=$(echo "$POD_JSON" | jq -r '.status.phase // "Unknown"')
    POD_READY=$(echo "$POD_JSON" | jq -r '.status.conditions[]? | select(.type=="Ready") | .status // "False"' | head -1)
else
    POD_PHASE=$(echo "$POD_JSON" | grep -o '"phase":"[^"]*"' | cut -d'"' -f4 | head -1 || echo "Unknown")
    POD_READY="Unknown"
fi

echo_info "Pod 阶段: $POD_PHASE"
if [ "$POD_READY" != "Unknown" ]; then
    echo_info "Pod Ready: $POD_READY"
fi

if [ "$POD_PHASE" = "Running" ]; then
    echo_info "✓ Pod 状态: Running"
else
    echo_warn "Pod 状态: $POD_PHASE (可能还在启动中)"
    # 显示更多信息
    if command -v jq &> /dev/null; then
        echo_info "Pod 详细信息:"
        echo "$POD_JSON" | jq '{name: .metadata.name, phase: .status.phase, conditions: .status.conditions}'
    fi
fi

# 步骤 7: 验证 Docker 容器是否运行
echo_step "步骤 7: 验证 Docker 容器是否运行"
CONTAINER_NAME="k8s_default_${POD_NAME}_nginx"
echo_info "查找容器: $CONTAINER_NAME"

# 检查容器是否存在并运行
if docker ps --format "{{.Names}}" 2>/dev/null | grep -q "^${CONTAINER_NAME}$"; then
    echo_info "✓ 容器正在运行: $CONTAINER_NAME"
    echo_info "容器详情:"
    docker ps --filter "name=${CONTAINER_NAME}" --format "table {{.Names}}\t{{.Status}}\t{{.Image}}\t{{.Ports}}"
else
    echo_warn "容器未运行或不存在: $CONTAINER_NAME"
    echo_info "检查所有 k8s_ 开头的容器:"
    docker ps -a --filter "name=k8s_" --format "table {{.Names}}\t{{.Status}}\t{{.Image}}" 2>/dev/null || echo_warn "未找到相关容器或 Docker 不可用"
    
    # 检查是否有其他 nginx 容器
    echo_info "检查是否有其他 nginx 相关容器:"
    docker ps -a --filter "ancestor=nginx" --format "table {{.Names}}\t{{.Status}}\t{{.Image}}" 2>/dev/null || true
fi

# 步骤 8: 显示 Pod 详细信息
echo_step "步骤 8: Pod 详细信息"
if command -v jq &> /dev/null; then
    echo "$POD_JSON" | jq '.'
else
    echo "$POD_JSON"
fi

# 步骤 9: 清除集群
echo_step ""
echo_step "步骤 9: 清除集群"
read -p "是否清除集群？(yes/no): " answer
if [ "$answer" = "yes" ]; then
    cleanup
    sleep 2
    go run ./cmd/k3 cluster clear --dir "$CLUSTER_DIR" --force
    rm -f "$NGINX_DEPLOYMENT"
    echo_info "✓ 集群已清除"
else
    echo_info "保留集群配置，可手动运行: go run ./cmd/k3 cluster clear --dir $CLUSTER_DIR"
    echo_info "k3 进程仍在运行 (PID: $K3_PID)"
    echo_info "停止命令: kill $K3_PID"
fi

echo_info ""
echo_info "演示完成！"
