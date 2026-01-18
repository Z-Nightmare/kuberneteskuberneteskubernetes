package apiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"zeusro.com/hermes/pkg/parser"
	"zeusro.com/hermes/pkg/storage"
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

// parseGVKFromContext 从 Gin context 解析 GroupVersionKind
func parseGVKFromContext(c *gin.Context) (schema.GroupVersionKind, error) {
	path := c.Request.URL.Path
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
func (s *APIServer) HandleGet(c *gin.Context) {
	gvk, err := parseGVKFromContext(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "resource name is required"})
		return
	}

	obj, err := s.store.Get(gvk, namespace, name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, obj)
}

// HandleList 处理 GET 请求（列出资源）
func (s *APIServer) HandleList(c *gin.Context) {
	gvk, err := parseGVKFromContext(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	namespace := c.Param("namespace")

	objects, err := s.store.List(gvk, namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
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

	c.JSON(http.StatusOK, list)
}

// HandleCreate 处理 POST 请求（创建资源）
func (s *APIServer) HandleCreate(c *gin.Context) {
	gvk, err := parseGVKFromContext(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 将 JSON 转换为 YAML 格式的字节
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 使用 parser 解析对象
	obj, _, err := s.parser.ParseYAML(bodyBytes)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 创建资源
	if err := s.store.Create(gvk, obj); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, obj)
}

// HandleUpdate 处理 PUT 请求（更新资源）
func (s *APIServer) HandleUpdate(c *gin.Context) {
	gvk, err := parseGVKFromContext(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	obj, _, err := s.parser.ParseYAML(bodyBytes)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 更新资源
	if err := s.store.Update(gvk, obj); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, obj)
}

// HandlePatch 处理 PATCH 请求（部分更新资源）
func (s *APIServer) HandlePatch(c *gin.Context) {
	gvk, err := parseGVKFromContext(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "resource name is required"})
		return
	}

	// 获取现有资源
	obj, err := s.store.Get(gvk, namespace, name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// 解析 patch 数据
	var patchData map[string]interface{}
	if err := c.ShouldBindJSON(&patchData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 简单的 merge patch 实现
	objBytes, err := json.Marshal(obj)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var objMap map[string]interface{}
	if err := json.Unmarshal(objBytes, &objMap); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 合并 patch
	mergePatch(objMap, patchData)

	// 转换回对象
	mergedBytes, err := json.Marshal(objMap)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	patchedObj, _, err := s.parser.ParseYAML(mergedBytes)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 更新资源
	if err := s.store.Update(gvk, patchedObj); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, patchedObj)
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
func (s *APIServer) HandleDelete(c *gin.Context) {
	gvk, err := parseGVKFromContext(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "resource name is required"})
		return
	}

	// 获取资源（用于返回）
	obj, err := s.store.Get(gvk, namespace, name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// 删除资源
	if err := s.store.Delete(gvk, namespace, name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 返回删除的对象
	c.JSON(http.StatusOK, obj)
}

// HandleWatch 处理 WATCH 请求（监听资源变更）
func (s *APIServer) HandleWatch(c *gin.Context) {
	gvk, err := parseGVKFromContext(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	namespace := c.Param("namespace")
	resourceVersion := c.Query("resourceVersion")

	// 设置 Server-Sent Events 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // 禁用 nginx 缓冲

	// 创建 watch channel
	eventCh, err := s.store.Watch(gvk, namespace, resourceVersion)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
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
	ctx := c.Request.Context()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// 发送初始事件（BOOKMARK）
	sendSSE(c.Writer, watch.Event{
		Type:   watch.Bookmark,
		Object: &metav1.Status{},
	})

	// 流式发送事件
	c.Stream(func(w io.Writer) bool {
		select {
		case event, ok := <-eventCh:
			if !ok {
				return false
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

			if !sendSSE(w, watchEvent) {
				return false
			}
			return true

		case <-ctx.Done():
			return false
		}
	})
}

// sendSSE 发送 Server-Sent Event
func sendSSE(w io.Writer, event watch.Event) bool {
	data, err := json.Marshal(event)
	if err != nil {
		return false
	}

	// SSE 格式: data: {json}\n\n
	_, err = fmt.Fprintf(w, "data: %s\n\n", string(data))
	if err != nil {
		return false
	}

	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	return true
}
