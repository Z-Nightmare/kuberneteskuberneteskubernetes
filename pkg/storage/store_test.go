package storage

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestMemoryStore_CreateGet(t *testing.T) {
	store := NewMemoryStore()

	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
	}

	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}

	// Create
	err := store.Create(gvk, pod)
	if err != nil {
		t.Fatalf("Failed to create pod: %v", err)
	}

	// Get
	retrieved, err := store.Get(gvk, "default", "test-pod")
	if err != nil {
		t.Fatalf("Failed to get pod: %v", err)
	}

	retrievedPod, ok := retrieved.(*corev1.Pod)
	if !ok {
		t.Fatalf("Retrieved object is not a Pod")
	}

	if retrievedPod.Name != "test-pod" {
		t.Errorf("Expected name 'test-pod', got '%s'", retrievedPod.Name)
	}
}

func TestMemoryStore_Update(t *testing.T) {
	store := NewMemoryStore()

	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
	}

	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}

	// Create
	err := store.Create(gvk, pod)
	if err != nil {
		t.Fatalf("Failed to create pod: %v", err)
	}

	// Update
	pod.Labels = map[string]string{"app": "nginx"}
	err = store.Update(gvk, pod)
	if err != nil {
		t.Fatalf("Failed to update pod: %v", err)
	}

	// Verify
	retrieved, err := store.Get(gvk, "default", "test-pod")
	if err != nil {
		t.Fatalf("Failed to get pod: %v", err)
	}

	retrievedPod := retrieved.(*corev1.Pod)
	if retrievedPod.Labels["app"] != "nginx" {
		t.Errorf("Expected label 'app=nginx', got '%v'", retrievedPod.Labels)
	}
}

func TestMemoryStore_Delete(t *testing.T) {
	store := NewMemoryStore()

	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
	}

	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}

	// Create
	err := store.Create(gvk, pod)
	if err != nil {
		t.Fatalf("Failed to create pod: %v", err)
	}

	// Delete
	err = store.Delete(gvk, "default", "test-pod")
	if err != nil {
		t.Fatalf("Failed to delete pod: %v", err)
	}

	// Verify deletion
	_, err = store.Get(gvk, "default", "test-pod")
	if err == nil {
		t.Error("Expected error when getting deleted pod, got nil")
	}
}

func TestMemoryStore_List(t *testing.T) {
	store := NewMemoryStore()

	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}

	// Create multiple pods
	for i := 0; i < 3; i++ {
		pod := &corev1.Pod{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Pod",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("test-pod-%d", i),
				Namespace: "default",
			},
		}
		err := store.Create(gvk, pod)
		if err != nil {
			t.Fatalf("Failed to create pod %d: %v", i, err)
		}
	}

	// List
	pods, err := store.List(gvk, "default")
	if err != nil {
		t.Fatalf("Failed to list pods: %v", err)
	}

	if len(pods) != 3 {
		t.Errorf("Expected 3 pods, got %d", len(pods))
	}
}

func TestMemoryStore_Watch(t *testing.T) {
	store := NewMemoryStore()

	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}

	// Start watching
	eventCh, err := store.Watch(gvk, "default", "")
	if err != nil {
		t.Fatalf("Failed to start watch: %v", err)
	}

	// Create a pod (should trigger ADDED event)
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
	}

	err = store.Create(gvk, pod)
	if err != nil {
		t.Fatalf("Failed to create pod: %v", err)
	}

	// Wait for event
	event := <-eventCh
	if event.Type != EventAdded {
		t.Errorf("Expected ADDED event, got %s", event.Type)
	}

	// Update pod (should trigger MODIFIED event)
	pod.Labels = map[string]string{"app": "nginx"}
	err = store.Update(gvk, pod)
	if err != nil {
		t.Fatalf("Failed to update pod: %v", err)
	}

	event = <-eventCh
	if event.Type != EventModified {
		t.Errorf("Expected MODIFIED event, got %s", event.Type)
	}

	// Delete pod (should trigger DELETED event)
	err = store.Delete(gvk, "default", "test-pod")
	if err != nil {
		t.Fatalf("Failed to delete pod: %v", err)
	}

	event = <-eventCh
	if event.Type != EventDeleted {
		t.Errorf("Expected DELETED event, got %s", event.Type)
	}
}
