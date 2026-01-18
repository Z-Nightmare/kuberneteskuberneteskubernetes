package parser

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestParseYAML_Pod(t *testing.T) {
	podYAML := `
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  namespace: default
spec:
  containers:
  - name: nginx
    image: nginx:1.21
    ports:
    - containerPort: 80
`

	parser := NewParser()
	obj, gvk, err := parser.ParseYAML([]byte(podYAML))
	if err != nil {
		t.Fatalf("Failed to parse Pod YAML: %v", err)
	}

	if gvk == nil {
		t.Fatal("GroupVersionKind is nil")
	}

	if gvk.Kind != "Pod" {
		t.Errorf("Expected Kind to be 'Pod', got '%s'", gvk.Kind)
	}

	if gvk.Version != "v1" {
		t.Errorf("Expected Version to be 'v1', got '%s'", gvk.Version)
	}

	if gvk.Group != "" {
		t.Errorf("Expected Group to be empty, got '%s'", gvk.Group)
	}

	// 类型断言
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		t.Fatalf("Expected *corev1.Pod, got %T", obj)
	}

	if pod.Name != "test-pod" {
		t.Errorf("Expected pod name to be 'test-pod', got '%s'", pod.Name)
	}

	if len(pod.Spec.Containers) != 1 {
		t.Fatalf("Expected 1 container, got %d", len(pod.Spec.Containers))
	}

	if pod.Spec.Containers[0].Name != "nginx" {
		t.Errorf("Expected container name to be 'nginx', got '%s'", pod.Spec.Containers[0].Name)
	}
}

func TestParseYAML_Deployment(t *testing.T) {
	deploymentYAML := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
  namespace: default
spec:
  replicas: 3
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
        image: nginx:1.21
        ports:
        - containerPort: 80
`

	parser := NewParser()
	obj, gvk, err := parser.ParseYAML([]byte(deploymentYAML))
	if err != nil {
		t.Fatalf("Failed to parse Deployment YAML: %v", err)
	}

	if gvk == nil {
		t.Fatal("GroupVersionKind is nil")
	}

	if gvk.Kind != "Deployment" {
		t.Errorf("Expected Kind to be 'Deployment', got '%s'", gvk.Kind)
	}

	if gvk.Version != "v1" {
		t.Errorf("Expected Version to be 'v1', got '%s'", gvk.Version)
	}

	if gvk.Group != "apps" {
		t.Errorf("Expected Group to be 'apps', got '%s'", gvk.Group)
	}

	// 类型断言
	deployment, ok := obj.(*appsv1.Deployment)
	if !ok {
		t.Fatalf("Expected *appsv1.Deployment, got %T", obj)
	}

	if deployment.Name != "test-deployment" {
		t.Errorf("Expected deployment name to be 'test-deployment', got '%s'", deployment.Name)
	}

	if *deployment.Spec.Replicas != 3 {
		t.Errorf("Expected replicas to be 3, got %d", *deployment.Spec.Replicas)
	}
}

func TestParseYAMLManifest_MultipleResources(t *testing.T) {
	manifestYAML := `
apiVersion: v1
kind: Pod
metadata:
  name: pod1
  namespace: default
spec:
  containers:
  - name: nginx
    image: nginx:1.21
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deployment1
  namespace: default
spec:
  replicas: 2
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
        image: nginx:1.21
---
apiVersion: v1
kind: Service
metadata:
  name: service1
  namespace: default
spec:
  selector:
    app: nginx
  ports:
  - port: 80
    targetPort: 80
`

	parser := NewParser()
	objects, gvks, err := parser.ParseYAMLManifest([]byte(manifestYAML))
	if err != nil {
		t.Fatalf("Failed to parse manifest: %v", err)
	}

	if len(objects) != 3 {
		t.Fatalf("Expected 3 objects, got %d", len(objects))
	}

	if len(gvks) != 3 {
		t.Fatalf("Expected 3 GVKs, got %d", len(gvks))
	}

	// 检查第一个对象（Pod）
	if gvks[0].Kind != "Pod" {
		t.Errorf("Expected first object to be Pod, got %s", gvks[0].Kind)
	}
	pod, ok := objects[0].(*corev1.Pod)
	if !ok {
		t.Fatalf("Expected first object to be *corev1.Pod, got %T", objects[0])
	}
	if pod.Name != "pod1" {
		t.Errorf("Expected pod name to be 'pod1', got '%s'", pod.Name)
	}

	// 检查第二个对象（Deployment）
	if gvks[1].Kind != "Deployment" {
		t.Errorf("Expected second object to be Deployment, got %s", gvks[1].Kind)
	}
	deployment, ok := objects[1].(*appsv1.Deployment)
	if !ok {
		t.Fatalf("Expected second object to be *appsv1.Deployment, got %T", objects[1])
	}
	if deployment.Name != "deployment1" {
		t.Errorf("Expected deployment name to be 'deployment1', got '%s'", deployment.Name)
	}

	// 检查第三个对象（Service）
	if gvks[2].Kind != "Service" {
		t.Errorf("Expected third object to be Service, got %s", gvks[2].Kind)
	}
	service, ok := objects[2].(*corev1.Service)
	if !ok {
		t.Fatalf("Expected third object to be *corev1.Service, got %T", objects[2])
	}
	if service.Name != "service1" {
		t.Errorf("Expected service name to be 'service1', got '%s'", service.Name)
	}
}

func TestToYAML(t *testing.T) {
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:1.21",
				},
			},
		},
	}

	yamlData, err := ToYAML(pod)
	if err != nil {
		t.Fatalf("Failed to serialize Pod to YAML: %v", err)
	}

	if len(yamlData) == 0 {
		t.Fatal("YAML data is empty")
	}

	// 验证可以重新解析
	parser := NewParser()
	obj, _, err := parser.ParseYAML(yamlData)
	if err != nil {
		t.Fatalf("Failed to parse serialized YAML: %v", err)
	}

	parsedPod, ok := obj.(*corev1.Pod)
	if !ok {
		t.Fatalf("Expected *corev1.Pod, got %T", obj)
	}

	if parsedPod.Name != "test-pod" {
		t.Errorf("Expected pod name to be 'test-pod', got '%s'", parsedPod.Name)
	}
}

// 测试各种原生 Kubernetes 资源类型
func TestParseYAML_NativeResources(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected string
	}{
		{
			name: "ConfigMap",
			yaml: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key1: value1
  key2: value2
`,
			expected: "ConfigMap",
		},
		{
			name: "Secret",
			yaml: `
apiVersion: v1
kind: Secret
metadata:
  name: test-secret
  namespace: default
type: Opaque
data:
  username: dXNlcm5hbWU=
  password: cGFzc3dvcmQ=
`,
			expected: "Secret",
		},
		{
			name: "Namespace",
			yaml: `
apiVersion: v1
kind: Namespace
metadata:
  name: test-namespace
`,
			expected: "Namespace",
		},
		{
			name: "Service",
			yaml: `
apiVersion: v1
kind: Service
metadata:
  name: test-service
  namespace: default
spec:
  selector:
    app: nginx
  ports:
  - port: 80
    targetPort: 8080
`,
			expected: "Service",
		},
		{
			name: "StatefulSet",
			yaml: `
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test-statefulset
  namespace: default
spec:
  serviceName: test-service
  replicas: 3
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
        image: nginx:1.21
`,
			expected: "StatefulSet",
		},
		{
			name: "DaemonSet",
			yaml: `
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: test-daemonset
  namespace: default
spec:
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
        image: nginx:1.21
`,
			expected: "DaemonSet",
		},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, gvk, err := parser.ParseYAML([]byte(tt.yaml))
			if err != nil {
				t.Fatalf("Failed to parse %s YAML: %v", tt.name, err)
			}

			if gvk == nil {
				t.Fatal("GroupVersionKind is nil")
			}

			if gvk.Kind != tt.expected {
				t.Errorf("Expected Kind to be '%s', got '%s'", tt.expected, gvk.Kind)
			}

			if obj == nil {
				t.Fatal("Object is nil")
			}

			// 验证对象实现了 runtime.Object 接口
			if _, ok := obj.(runtime.Object); !ok {
				t.Errorf("Object does not implement runtime.Object")
			}
		})
	}
}
