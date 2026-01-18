package apiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/pkg/parser"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/pkg/storage"
	"github.com/gofiber/fiber/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
)

// APIServer 是 Kubernetes API server 的实现
type APIServer struct {
	store  storage.Store
	parser *parser.Parser
}

// NewAPIServer 创建新的 API server
func NewAPIServer(store storage.Store) *APIServer {
	return &APIServer{
		store:  store,
		parser: parser.NewParser(),
	}
}

// ListOptions 表示列表查询选项
type ListOptions struct {
	LabelSelector string
	FieldSelector string
	Limit         int64
	Continue      string
}

// WatchOptions 表示 watch 查询选项
type WatchOptions struct {
	ResourceVersion     string
	AllowWatchBookmarks bool
	TimeoutSeconds      *int64
}

// parseGVKFromContext 从 Fiber context 解析 GroupVersionKind
func parseGVKFromContext(c *fiber.Ctx) (schema.GroupVersionKind, error) {
	path := c.Path()
	parts := strings.Split(strings.Trim(path, "/"), "/")

	if len(parts) < 3 {
		return schema.GroupVersionKind{}, fmt.Errorf("invalid path format")
	}

	var group, version, kind string

	if parts[0] == "api" {
		// Core API: /api/v1/pods
		group = ""
		version = parts[1]
		// 从路径中找到 kind（跳过 watch 和 namespaces）
		for i := 1; i < len(parts); i++ {
			if parts[i] != "v1" && parts[i] != "watch" && parts[i] != "namespaces" {
				kind = parts[i]
				break
			}
		}
	} else if parts[0] == "apis" {
		// Grouped API: /apis/apps/v1/deployments
		if len(parts) < 4 {
			return schema.GroupVersionKind{}, fmt.Errorf("invalid grouped API path")
		}
		group = parts[1]
		version = parts[2]
		// 从路径中找到 kind（跳过 watch 和 namespaces）
		for i := 3; i < len(parts); i++ {
			if parts[i] != "watch" && parts[i] != "namespaces" {
				kind = parts[i]
				break
			}
		}
	} else {
		return schema.GroupVersionKind{}, fmt.Errorf("unknown API path prefix: %s", parts[0])
	}

	// 将 kind 转换为正确的格式（首字母大写，其余小写）
	if len(kind) > 0 {
		kind = strings.ToUpper(kind[:1]) + strings.ToLower(kind[1:])
	}

	return schema.GroupVersionKind{
		Group:   group,
		Version: version,
		Kind:    kind,
	}, nil
}

// HandleGet 处理 GET 请求（获取单个资源）
func (s *APIServer) HandleGet(c *fiber.Ctx) error {
	gvk, err := parseGVKFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	namespace := c.Params("namespace")
	name := c.Params("name")

	if name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "resource name is required"})
	}

	obj, err := s.store.Get(gvk, namespace, name)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(obj)
}

// HandleList 处理 GET 请求（列出资源）
func (s *APIServer) HandleList(c *fiber.Ctx) error {
	gvk, err := parseGVKFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	namespace := c.Params("namespace")

	objects, err := s.store.List(gvk, namespace)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// 构建 List 响应
	list := &metav1.List{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "List",
		},
		Items: make([]runtime.RawExtension, 0, len(objects)),
	}

	for _, obj := range objects {
		data, err := json.Marshal(obj)
		if err != nil {
			continue
		}
		list.Items = append(list.Items, runtime.RawExtension{Raw: data})
	}

	return c.Status(fiber.StatusOK).JSON(list)
}

// HandleCreate 处理 POST 请求（创建资源）
func (s *APIServer) HandleCreate(c *fiber.Ctx) error {
	gvk, err := parseGVKFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var body map[string]interface{}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// 将 JSON 转换为 YAML 格式的字节
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// 使用 parser 解析对象
	obj, _, err := s.parser.ParseYAML(bodyBytes)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// 创建资源
	if err := s.store.Create(gvk, obj); err != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(obj)
}

// HandleUpdate 处理 PUT 请求（更新资源）
func (s *APIServer) HandleUpdate(c *fiber.Ctx) error {
	gvk, err := parseGVKFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var body map[string]interface{}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	obj, _, err := s.parser.ParseYAML(bodyBytes)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// 更新资源
	if err := s.store.Update(gvk, obj); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(obj)
}

// HandlePatch 处理 PATCH 请求（部分更新资源）
func (s *APIServer) HandlePatch(c *fiber.Ctx) error {
	gvk, err := parseGVKFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	namespace := c.Params("namespace")
	name := c.Params("name")

	if name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "resource name is required"})
	}

	// 获取现有资源
	obj, err := s.store.Get(gvk, namespace, name)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}

	// 解析 patch 数据
	var patchData map[string]interface{}
	if err := c.BodyParser(&patchData); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// 简单的 merge patch 实现
	objBytes, err := json.Marshal(obj)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var objMap map[string]interface{}
	if err := json.Unmarshal(objBytes, &objMap); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// 合并 patch
	mergePatch(objMap, patchData)

	// 转换回对象
	mergedBytes, err := json.Marshal(objMap)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	patchedObj, _, err := s.parser.ParseYAML(mergedBytes)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// 更新资源
	if err := s.store.Update(gvk, patchedObj); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(patchedObj)
}

// mergePatch 合并 patch 数据
func mergePatch(dst, src map[string]interface{}) {
	for k, v := range src {
		if v == nil {
			delete(dst, k)
		} else if srcMap, ok := v.(map[string]interface{}); ok {
			if dstMap, ok := dst[k].(map[string]interface{}); ok {
				mergePatch(dstMap, srcMap)
			} else {
				dst[k] = v
			}
		} else {
			dst[k] = v
		}
	}
}

// HandleDelete 处理 DELETE 请求（删除资源）
func (s *APIServer) HandleDelete(c *fiber.Ctx) error {
	gvk, err := parseGVKFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	namespace := c.Params("namespace")
	name := c.Params("name")

	if name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "resource name is required"})
	}

	// 获取资源（用于返回）
	obj, err := s.store.Get(gvk, namespace, name)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}

	// 删除资源
	if err := s.store.Delete(gvk, namespace, name); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// 返回删除的对象
	return c.Status(fiber.StatusOK).JSON(obj)
}

// HandleWatch 处理 WATCH 请求（监听资源变更）
func (s *APIServer) HandleWatch(c *fiber.Ctx) error {
	gvk, err := parseGVKFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	namespace := c.Params("namespace")
	resourceVersion := c.Query("resourceVersion")

	// 设置 Server-Sent Events 响应头
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no") // 禁用 nginx 缓冲

	// 创建 watch channel
	eventCh, err := s.store.Watch(gvk, namespace, resourceVersion)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// 设置超时（可选）
	timeoutSeconds := c.Query("timeoutSeconds")
	var timeout time.Duration = 30 * time.Minute // 默认 30 分钟
	if timeoutSeconds != "" {
		if sec, err := strconv.ParseInt(timeoutSeconds, 10, 64); err == nil {
			timeout = time.Duration(sec) * time.Second
		}
	}

	// 创建 context with timeout
	reqCtx := c.UserContext()
	if timeout > 0 {
		var cancel context.CancelFunc
		reqCtx, cancel = context.WithTimeout(reqCtx, timeout)
		defer cancel()
	}

	// 发送初始事件（BOOKMARK）
	initialEvent := watch.Event{
		Type:   watch.Bookmark,
		Object: &metav1.Status{},
	}
	if err := sendSSEFiber(c, initialEvent); err != nil {
		return err
	}

	// 流式发送事件
	for {
		select {
		case event, ok := <-eventCh:
			if !ok {
				return nil
			}

			// 转换事件类型
			var watchType watch.EventType
			switch event.Type {
			case storage.EventAdded:
				watchType = watch.Added
			case storage.EventModified:
				watchType = watch.Modified
			case storage.EventDeleted:
				watchType = watch.Deleted
			case storage.EventBookmark:
				watchType = watch.Bookmark
			default:
				watchType = watch.Added
			}

			// 发送事件
			watchEvent := watch.Event{
				Type:   watchType,
				Object: event.Object,
			}

			if err := sendSSEFiber(c, watchEvent); err != nil {
				return err
			}

		case <-reqCtx.Done():
			return nil
		}
	}
}

// sendSSEFiber 发送 Server-Sent Event (Fiber版本)
func sendSSEFiber(c *fiber.Ctx, event watch.Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// SSE 格式: data: {json}\n\n
	_, err = fmt.Fprintf(c, "data: %s\n\n", string(data))
	if err != nil {
		return err
	}

	// Fiber 会自动处理刷新
	return nil
}
