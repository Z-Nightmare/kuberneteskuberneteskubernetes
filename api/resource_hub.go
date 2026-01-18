package api

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/logprovider"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/pkg/storage"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ResourceSnapshot struct {
	Type        string    `json:"type"` // "snapshot"
	GeneratedAt time.Time `json:"generatedAt"`
	Nodes       []NodeDTO `json:"nodes"`
	Pods        []PodDTO  `json:"pods"`
	Counts      CountsDTO `json:"counts"`
	Error       *ErrorDTO `json:"error,omitempty"`
	Info        *InfoDTO  `json:"info,omitempty"`
}

type CountsDTO struct {
	Nodes int `json:"nodes"`
	Pods  int `json:"pods"`
}

type ErrorDTO struct {
	Message string `json:"message"`
}

type InfoDTO struct {
	Message string `json:"message"`
}

type NodeDTO struct {
	Name   string            `json:"name"`
	Ready  bool              `json:"ready"`
	Phase  string            `json:"phase"`
	RV     string            `json:"resourceVersion"`
	UID    string            `json:"uid"`
	Labels map[string]string `json:"labels,omitempty"`
}

type PodDTO struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	NodeName  string `json:"nodeName,omitempty"`
	Phase     string `json:"phase"`
	Ready     bool   `json:"ready"`
	Restarts  int32  `json:"restarts"`
	RV        string `json:"resourceVersion"`
	UID       string `json:"uid"`
}

// ResourceHub watches Store and broadcasts snapshots to subscribers.
type ResourceHub struct {
	store  storage.Store
	logger logprovider.Logger

	mu   sync.RWMutex
	subs map[string]chan []byte

	startOnce sync.Once
}

func NewResourceHub(store storage.Store, logger logprovider.Logger) *ResourceHub {
	return &ResourceHub{
		store:  store,
		logger: logger,
		subs:   make(map[string]chan []byte),
	}
}

func (h *ResourceHub) Start(ctx context.Context) {
	h.startOnce.Do(func() {
		nodeGVK := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Node"}
		podGVK := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}

		nodeCh, err := h.store.Watch(nodeGVK, "", "")
		if err != nil {
			h.logger.Warnf("ResourceHub: watch nodes failed: %v", err)
		}
		podCh, err := h.store.Watch(podGVK, "", "")
		if err != nil {
			h.logger.Warnf("ResourceHub: watch pods failed: %v", err)
		}

		trigger := make(chan struct{}, 1)
		triggerSend := func() {
			select {
			case trigger <- struct{}{}:
			default:
			}
		}

		// Any store event triggers a debounced snapshot broadcast.
		if nodeCh != nil {
			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case _, ok := <-nodeCh:
						if !ok {
							return
						}
						triggerSend()
					}
				}
			}()
		}
		if podCh != nil {
			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case _, ok := <-podCh:
						if !ok {
							return
						}
						triggerSend()
					}
				}
			}()
		}

		// Broadcast loop with debounce.
		go func() {
			var (
				timer  *time.Timer
				timerC <-chan time.Time
			)
			for {
				select {
				case <-ctx.Done():
					return
				case <-trigger:
					if timer == nil {
						timer = time.NewTimer(200 * time.Millisecond)
						timerC = timer.C
					} else {
						if !timer.Stop() {
							select {
							case <-timer.C:
							default:
							}
						}
						timer.Reset(200 * time.Millisecond)
						timerC = timer.C
					}
				case <-timerC:
					timer.Stop()
					timer = nil
					timerC = nil
					h.broadcastSnapshot()
				}
			}
		}()

		// Initial push.
		h.broadcastSnapshot()
	})
}

func (h *ResourceHub) Subscribe(id string) (ch <-chan []byte, unsubscribe func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	c := make(chan []byte, 20)
	h.subs[id] = c

	return c, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if existing, ok := h.subs[id]; ok {
			delete(h.subs, id)
			close(existing)
		}
	}
}

func (h *ResourceHub) broadcastSnapshot() {
	payload, err := h.SnapshotJSON()
	if err != nil {
		h.logger.Warnf("ResourceHub: snapshot failed: %v", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for id, ch := range h.subs {
		select {
		case ch <- payload:
		default:
			// Slow client; drop message.
			_ = id
		}
	}
}

func (h *ResourceHub) SnapshotJSON() ([]byte, error) {
	snap := ResourceSnapshot{
		Type:        "snapshot",
		GeneratedAt: time.Now(),
	}

	nodeGVK := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Node"}
	podGVK := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}

	nodes, err := h.store.List(nodeGVK, "")
	if err != nil {
		snap.Error = &ErrorDTO{Message: "list nodes failed: " + err.Error()}
	}
	pods, err2 := h.store.List(podGVK, "")
	if err2 != nil {
		if snap.Error == nil {
			snap.Error = &ErrorDTO{Message: "list pods failed: " + err2.Error()}
		} else {
			snap.Error.Message = snap.Error.Message + "; list pods failed: " + err2.Error()
		}
	}

	for _, obj := range nodes {
		if n, ok := obj.(*corev1.Node); ok {
			snap.Nodes = append(snap.Nodes, nodeToDTO(n))
		}
	}
	for _, obj := range pods {
		if p, ok := obj.(*corev1.Pod); ok {
			snap.Pods = append(snap.Pods, podToDTO(p))
		}
	}
	snap.Counts = CountsDTO{Nodes: len(snap.Nodes), Pods: len(snap.Pods)}

	b, err := json.Marshal(snap)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func nodeToDTO(n *corev1.Node) NodeDTO {
	ready := false
	for _, c := range n.Status.Conditions {
		if c.Type == corev1.NodeReady {
			ready = c.Status == corev1.ConditionTrue
			break
		}
	}
	return NodeDTO{
		Name:   n.Name,
		Ready:  ready,
		Phase:  string(n.Status.Phase),
		RV:     n.ResourceVersion,
		UID:    string(n.UID),
		Labels: n.Labels,
	}
}

func podToDTO(p *corev1.Pod) PodDTO {
	ready := false
	for _, c := range p.Status.Conditions {
		if c.Type == corev1.PodReady {
			ready = c.Status == corev1.ConditionTrue
			break
		}
	}
	var restarts int32
	for _, cs := range p.Status.ContainerStatuses {
		restarts += cs.RestartCount
	}
	return PodDTO{
		Namespace: p.Namespace,
		Name:      p.Name,
		NodeName:  p.Spec.NodeName,
		Phase:     string(p.Status.Phase),
		Ready:     ready,
		Restarts:  restarts,
		RV:        p.ResourceVersion,
		UID:       string(p.UID),
	}
}
