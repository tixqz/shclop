package k8s

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

// Factory holds two DynamicSharedInformerFactories: one cluster-wide (for the
// cluster-scoped IntegrationPlugin) and one namespace-filtered (for MCPServer).
type Factory struct {
	cluster    dynamicinformer.DynamicSharedInformerFactory
	namespaced dynamicinformer.DynamicSharedInformerFactory
}

// NewFactory creates a Factory. namespace scopes the MCPServer informer;
// IntegrationPlugin uses metav1.NamespaceAll (cluster-wide).
func NewFactory(client *Client, namespace string, resync time.Duration) *Factory {
	cluster := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		client.Dynamic, resync, metav1.NamespaceAll, nil,
	)
	ns := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		client.Dynamic, resync, namespace, nil,
	)
	return &Factory{
		cluster:    cluster,
		namespaced: ns,
	}
}

// IntegrationPluginInformer returns the SharedIndexInformer for IntegrationPlugin (cluster-scoped).
func (f *Factory) IntegrationPluginInformer() cache.SharedIndexInformer {
	return f.cluster.ForResource(IntegrationPluginGVR).Informer()
}

// MCPServerInformer returns the SharedIndexInformer for MCPServer (namespace-scoped).
func (f *Factory) MCPServerInformer() cache.SharedIndexInformer {
	return f.namespaced.ForResource(MCPServerGVR).Informer()
}

// Start launches goroutines for both factories. stopCh stops all informers.
func (f *Factory) Start(stopCh <-chan struct{}) {
	f.cluster.Start(stopCh)
	f.namespaced.Start(stopCh)
}

// WaitForCacheSync blocks until both factories' caches are synced or stopCh is closed.
// Returns true if all caches synced successfully.
func (f *Factory) WaitForCacheSync(stopCh <-chan struct{}) bool {
	clusterSynced := f.cluster.WaitForCacheSync(stopCh)
	namespacedSynced := f.namespaced.WaitForCacheSync(stopCh)
	for _, ok := range clusterSynced {
		if !ok {
			return false
		}
	}
	for _, ok := range namespacedSynced {
		if !ok {
			return false
		}
	}
	return true
}
