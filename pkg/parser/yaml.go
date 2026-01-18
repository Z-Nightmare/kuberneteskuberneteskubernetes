package parser

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
)

// Parser 是 Kubernetes YAML 解析器
type Parser struct {
	decoder runtime.Decoder
}

// NewParser 创建一个新的 YAML 解析器
// 使用 Kubernetes 的标准 scheme，支持所有原生资源类型
func NewParser() *Parser {
	// 使用 UniversalDeserializer，它包含了所有在 scheme 中注册的类型
	decoder := scheme.Codecs.UniversalDeserializer()
	return &Parser{
		decoder: decoder,
	}
}

// ParseYAML 解析单个 YAML 文档
// 返回解析后的 runtime.Object 和 GroupVersionKind
func (p *Parser) ParseYAML(data []byte) (runtime.Object, *schema.GroupVersionKind, error) {
	// 使用 UniversalDeserializer 来解码 YAML
	// 它会自动识别资源类型并反序列化为对应的 Go 对象
	obj, gvk, err := p.decoder.Decode(data, nil, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode YAML: %w", err)
	}

	return obj, gvk, nil
}

// ParseYAMLManifest 解析包含多个 YAML 文档的 manifest 文件
// Kubernetes manifest 文件通常使用 `---` 分隔多个资源
func (p *Parser) ParseYAMLManifest(data []byte) ([]runtime.Object, []*schema.GroupVersionKind, error) {
	var objects []runtime.Object
	var gvks []*schema.GroupVersionKind

	// 手动分割 YAML 文档（按 `---` 分隔符）
	docs := splitYAMLDocuments(data)

	for _, doc := range docs {
		doc = bytes.TrimSpace(doc)

		// 跳过空文档
		if len(doc) == 0 {
			continue
		}

		// 跳过纯注释文档
		lines := strings.Split(string(doc), "\n")
		hasContent := false
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				hasContent = true
				break
			}
		}
		if !hasContent {
			continue
		}

		obj, gvk, err := p.ParseYAML(doc)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse document: %w\nDocument content:\n%s", err, string(doc))
		}

		objects = append(objects, obj)
		gvks = append(gvks, gvk)
	}

	return objects, gvks, nil
}

// splitYAMLDocuments 分割 YAML 文档（按 `---` 分隔符）
func splitYAMLDocuments(data []byte) [][]byte {
	var docs [][]byte
	parts := bytes.Split(data, []byte("\n---\n"))

	for _, part := range parts {
		// 处理开头的 `---`
		part = bytes.TrimPrefix(part, []byte("---\n"))
		part = bytes.TrimPrefix(part, []byte("---"))
		part = bytes.TrimSpace(part)

		if len(part) > 0 {
			docs = append(docs, part)
		}
	}

	return docs
}

// ParseYAMLFromReader 从 io.Reader 读取并解析 YAML
func (p *Parser) ParseYAMLFromReader(reader io.Reader) ([]runtime.Object, []*schema.GroupVersionKind, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read from reader: %w", err)
	}
	return p.ParseYAMLManifest(data)
}

// ParseYAMLFile 从文件路径读取并解析 YAML
func (p *Parser) ParseYAMLFile(filePath string) ([]runtime.Object, []*schema.GroupVersionKind, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}
	return p.ParseYAMLManifest(data)
}

// GetYAMLSerializer 获取 YAML 序列化器
func GetYAMLSerializer() runtime.Serializer {
	// 使用 scheme 创建 CodecFactory
	codecFactory := serializer.NewCodecFactory(scheme.Scheme)
	// 获取 YAML 序列化器
	// YAMLMediaType() 返回的是 SerializerInfo，需要获取 Serializer
	info, ok := runtime.SerializerInfoForMediaType(codecFactory.SupportedMediaTypes(), runtime.ContentTypeYAML)
	if !ok {
		// 如果找不到，使用默认的 YAML 序列化器
		return codecFactory.LegacyCodec(scheme.Scheme.PreferredVersionAllGroups()...)
	}
	return info.Serializer
}

// ToYAML 将 runtime.Object 序列化为 YAML
func ToYAML(obj runtime.Object) ([]byte, error) {
	serializer := GetYAMLSerializer()
	return runtime.Encode(serializer, obj)
}
