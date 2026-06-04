package plugins

import (
	"context"
	"fmt"
	"log/slog"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/mipopov/shclop/internal/k8s"
)

// CRDRegistry watches IntegrationPlugin CRs via a SharedIndexInformer and
// implements the Registry interface.  Source label is "crd".
type CRDRegistry struct {
	baseRegistry

	informer cache.SharedIndexInformer
	logger   *slog.Logger
	nameToID map[string]string // CR metadata.name → manifest id
}

// NewCRDRegistry creates a CRDRegistry backed by the provided informer.
func NewCRDRegistry(informer cache.SharedIndexInformer, logger *slog.Logger) *CRDRegistry {
	return &CRDRegistry{
		baseRegistry: baseRegistry{resolved: make(map[string]Resolved)},
		informer:     informer,
		logger:       logger,
		nameToID:     make(map[string]string),
	}
}

// Run registers event handlers on the informer, waits for the cache to sync,
// and then blocks until ctx is cancelled.  It is idempotent (sync.Once).
// The function returns nil immediately; blocking work runs in a background goroutine.
func (r *CRDRegistry) Run(ctx context.Context) error {
	r.once.Do(func() {
		r.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj any) {
				r.upsert(obj)
			},
			UpdateFunc: func(_, newObj any) {
				r.upsert(newObj)
			},
			DeleteFunc: func(obj any) {
				r.delete(obj)
			},
		})

		go func() {
			if !cache.WaitForCacheSync(ctx.Done(), r.informer.HasSynced) {
				return
			}
			r.logger.Info("crd_registry: cache synced")
			<-ctx.Done()
		}()
	})
	return nil
}

// syntheticEnvelope is marshalled via sigs.k8s.io/yaml to produce the YAML blob
// that ParseManifest expects.  JSON tags match the YAML tags on ManifestSpec/Manifest
// because sigs.k8s.io/yaml converts JSON tags to YAML keys.
type syntheticEnvelope struct {
	APIVersion string                    `json:"apiVersion"`
	Kind       string                    `json:"kind"`
	Metadata   syntheticMeta             `json:"metadata"`
	Spec       k8s.IntegrationPluginSpec `json:"spec"`
}

type syntheticMeta struct {
	Name string `json:"name"`
}

// upsert converts an unstructured IntegrationPlugin into a Manifest, stores it,
// and broadcasts to subscribers.
func (r *CRDRegistry) upsert(obj any) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		r.logger.Warn("crd_registry: upsert received non-Unstructured object", "type", fmt.Sprintf("%T", obj))
		return
	}

	ip, err := k8s.UnstructuredToIntegrationPlugin(u)
	if err != nil {
		r.logger.Warn("crd_registry: failed to convert to IntegrationPlugin",
			"name", u.GetName(), "err", err)
		return
	}

	env := syntheticEnvelope{
		APIVersion: "shclop.io/v1alpha1",
		Kind:       "IntegrationPlugin",
		Metadata:   syntheticMeta{Name: ip.Name},
		Spec:       ip.Spec,
	}

	yamlBytes, err := sigsyaml.Marshal(env)
	if err != nil {
		r.logger.Warn("crd_registry: failed to marshal spec to yaml",
			"name", ip.Name, "err", err)
		return
	}

	m, err := ParseManifest(yamlBytes)
	if err != nil {
		r.logger.Warn("crd_registry: manifest validation failed",
			"name", ip.Name, "err", err)
		return
	}

	resolved := Resolved{Manifest: *m, Source: "crd"}

	r.mu.Lock()
	r.resolved[m.Spec.ID] = resolved
	r.nameToID[ip.Name] = m.Spec.ID
	r.mu.Unlock()

	r.broadcast()
}

// delete removes the entry associated with the deleted CR and broadcasts.
func (r *CRDRegistry) delete(obj any) {
	// Unwrap tombstone if the informer couldn't deliver the full object.
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}

	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		r.logger.Warn("crd_registry: delete received non-Unstructured object", "type", fmt.Sprintf("%T", obj))
		return
	}

	name := u.GetName()

	r.mu.Lock()
	id, found := r.nameToID[name]
	if found {
		delete(r.resolved, id)
		delete(r.nameToID, name)
	}
	r.mu.Unlock()

	if found {
		r.broadcast()
	}
}
