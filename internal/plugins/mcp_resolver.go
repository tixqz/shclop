package plugins

import (
	"context"
	"fmt"
	"log/slog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"

	"github.com/mipopov/shclop/internal/k8s"
)

// ResolvedMCPServer is a runtime-ready MCP server entry that the agent receives
// via SHCLOP_MCP_CONFIG.
type ResolvedMCPServer struct {
	Name    string            `json:"name"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// MCPResolver translates MCPRef entries from plugin manifests into
// ResolvedMCPServer entries using the informer cache for in-cluster lookups.
type MCPResolver struct {
	Lister    cache.GenericLister
	Namespace string
	Logger    *slog.Logger
}

// NewMCPResolver creates an MCPResolver backed by the provided informer.
func NewMCPResolver(informer cache.SharedIndexInformer, namespace string, logger *slog.Logger) *MCPResolver {
	return &MCPResolver{
		Lister:    cache.NewGenericLister(informer.GetIndexer(), k8s.MCPServerGVR.GroupResource()),
		Namespace: namespace,
		Logger:    logger,
	}
}

// Resolve translates refs into concrete ResolvedMCPServer entries.
// Template values in URLs, auth headers, and env values are rendered against vars.
func (r *MCPResolver) Resolve(ctx context.Context, refs []MCPRef, vars TemplateContext) ([]ResolvedMCPServer, error) {
	out := make([]ResolvedMCPServer, 0, len(refs))

	for i, ref := range refs {
		entry, err := r.resolveOne(ctx, ref, vars)
		if err != nil {
			return nil, fmt.Errorf("ref[%d] %q: %w", i, ref.Name, err)
		}
		out = append(out, entry)
	}

	return out, nil
}

func (r *MCPResolver) resolveOne(_ context.Context, ref MCPRef, vars TemplateContext) (ResolvedMCPServer, error) {
	if ref.External != nil {
		return r.resolveExternal(ref, vars)
	}
	if ref.Ref != "" {
		return r.resolveCluster(ref, vars)
	}
	return ResolvedMCPServer{}, fmt.Errorf("ref has neither external nor ref field set")
}

func (r *MCPResolver) resolveExternal(ref MCPRef, vars TemplateContext) (ResolvedMCPServer, error) {
	url, err := renderTemplate(ref.Name+".url", ref.External.URL, vars)
	if err != nil {
		return ResolvedMCPServer{}, fmt.Errorf("render url: %w", err)
	}

	entry := ResolvedMCPServer{Name: ref.Name, URL: url}

	if ref.External.AuthHeader != "" {
		auth, err := renderTemplate(ref.Name+".auth_header", ref.External.AuthHeader, vars)
		if err != nil {
			return ResolvedMCPServer{}, fmt.Errorf("render auth_header: %w", err)
		}
		entry.Headers = map[string]string{"Authorization": auth}
	}

	env, err := r.renderEnvFrom(ref.EnvFrom, vars)
	if err != nil {
		return ResolvedMCPServer{}, err
	}
	if len(env) > 0 {
		entry.Env = env
	}

	return entry, nil
}

func (r *MCPResolver) resolveCluster(ref MCPRef, vars TemplateContext) (ResolvedMCPServer, error) {
	obj, err := r.Lister.ByNamespace(r.Namespace).Get(ref.Ref)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ResolvedMCPServer{}, fmt.Errorf("mcpserver_not_found: %s", ref.Ref)
		}
		return ResolvedMCPServer{}, fmt.Errorf("lister get %q: %w", ref.Ref, err)
	}

	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return ResolvedMCPServer{}, fmt.Errorf("unexpected type %T for MCPServer %q", obj, ref.Ref)
	}

	cr, err := k8s.UnstructuredToMCPServer(u)
	if err != nil {
		return ResolvedMCPServer{}, fmt.Errorf("convert MCPServer %q: %w", ref.Ref, err)
	}

	serviceURL := cr.Status.ServiceURL
	if serviceURL == "" {
		serviceURL = fmt.Sprintf("http://mcp-%s.%s.svc.cluster.local:%d",
			cr.Name, r.Namespace, cr.Spec.Port)
	}

	entry := ResolvedMCPServer{Name: ref.Name, URL: serviceURL}

	env, err := r.renderEnvFrom(ref.EnvFrom, vars)
	if err != nil {
		return ResolvedMCPServer{}, err
	}
	if len(env) > 0 {
		entry.Env = env
	}

	return entry, nil
}

func (r *MCPResolver) renderEnvFrom(envFrom map[string]string, vars TemplateContext) (map[string]string, error) {
	if len(envFrom) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(envFrom))
	for k, v := range envFrom {
		rendered, err := renderTemplate("env:"+k, v, vars)
		if err != nil {
			return nil, fmt.Errorf("render env_from[%s]: %w", k, err)
		}
		out[k] = rendered
	}
	return out, nil
}
