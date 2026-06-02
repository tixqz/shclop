package k8s

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client holds dynamic and typed k8s clients plus the default runtime namespace.
type Client struct {
	Dynamic    dynamic.Interface
	Kubernetes kubernetes.Interface
	Namespace  string
}

// NewClient builds a Client from a kubeconfig path or in-cluster config if path is empty.
func NewClient(kubeconfig string, namespace string) (*Client, error) {
	var cfg *rest.Config
	var err error
	if kubeconfig == "" {
		cfg, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("in-cluster config: %w", err)
		}
	} else {
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("kubeconfig %q: %w", kubeconfig, err)
		}
	}

	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("dynamic client: %w", err)
	}

	kube, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("kubernetes client: %w", err)
	}

	return &Client{
		Dynamic:    dyn,
		Kubernetes: kube,
		Namespace:  namespace,
	}, nil
}

// IntegrationPlugins returns a cluster-scoped dynamic resource interface.
func (c *Client) IntegrationPlugins() dynamic.ResourceInterface {
	return c.Dynamic.Resource(IntegrationPluginGVR)
}

// MCPServers returns a namespace-scoped dynamic resource interface for the given namespace.
func (c *Client) MCPServers(namespace string) dynamic.ResourceInterface {
	return c.Dynamic.Resource(MCPServerGVR).Namespace(namespace)
}

// UnstructuredToIntegrationPlugin converts an unstructured object to a typed IntegrationPlugin.
func UnstructuredToIntegrationPlugin(u *unstructured.Unstructured) (*IntegrationPlugin, error) {
	var out IntegrationPlugin
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &out); err != nil {
		return nil, fmt.Errorf("convert to IntegrationPlugin: %w", err)
	}
	return &out, nil
}

// UnstructuredToMCPServer converts an unstructured object to a typed MCPServer.
func UnstructuredToMCPServer(u *unstructured.Unstructured) (*MCPServer, error) {
	var out MCPServer
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &out); err != nil {
		return nil, fmt.Errorf("convert to MCPServer: %w", err)
	}
	return &out, nil
}
