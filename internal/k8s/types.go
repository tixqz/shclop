package k8s

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	GroupName    = "shclop.io"
	GroupVersion = "v1alpha1"
)

var (
	SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: GroupVersion}
	IntegrationPluginGVR = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  GroupVersion,
		Resource: "integrationplugins",
	}
	MCPServerGVR = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  GroupVersion,
		Resource: "mcpservers",
	}
)

// IntegrationPlugin is a cluster-scoped CRD that declares an integration plugin.
type IntegrationPlugin struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              IntegrationPluginSpec   `json:"spec"`
	Status            IntegrationPluginStatus `json:"status,omitempty"`
}

func (in *IntegrationPlugin) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(IntegrationPlugin)
	in.DeepCopyInto(out)
	return out
}

func (in *IntegrationPlugin) DeepCopy() *IntegrationPlugin {
	if in == nil {
		return nil
	}
	out := new(IntegrationPlugin)
	in.DeepCopyInto(out)
	return out
}

func (in *IntegrationPlugin) DeepCopyInto(out *IntegrationPlugin) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// IntegrationPluginSpec mirrors plugins.ManifestSpec with json tags matching the YAML tags.
// Duplicated here to avoid a circular import: plugins will eventually import k8s for the CRD registry.
type IntegrationPluginSpec struct {
	ID              string        `json:"id"`
	DisplayName     string        `json:"display_name,omitempty"`
	Description     string        `json:"description,omitempty"`
	PluginKind      string        `json:"kind,omitempty"`
	ScopesSupported []string      `json:"scopes_supported,omitempty"`
	Auth            AuthSpec      `json:"auth,omitempty"`
	Validate        ValidateSpec  `json:"validate,omitempty"`
	Sidecar         *SidecarSpec  `json:"sidecar,omitempty"`
	Contributions   Contributions `json:"contributions,omitempty"`
}

func (in *IntegrationPluginSpec) DeepCopyInto(out *IntegrationPluginSpec) {
	*out = *in
	if in.ScopesSupported != nil {
		out.ScopesSupported = make([]string, len(in.ScopesSupported))
		copy(out.ScopesSupported, in.ScopesSupported)
	}
	in.Auth.DeepCopyInto(&out.Auth)
	in.Validate.DeepCopyInto(&out.Validate)
	if in.Sidecar != nil {
		out.Sidecar = new(SidecarSpec)
		*out.Sidecar = *in.Sidecar
	}
	in.Contributions.DeepCopyInto(&out.Contributions)
}

type AuthSpec struct {
	Type   string      `json:"type,omitempty"`
	Fields []FormField `json:"fields,omitempty"`
}

func (in *AuthSpec) DeepCopyInto(out *AuthSpec) {
	*out = *in
	if in.Fields != nil {
		out.Fields = make([]FormField, len(in.Fields))
		copy(out.Fields, in.Fields)
	}
}

type FormField struct {
	Name        string `json:"name,omitempty"`
	Label       string `json:"label,omitempty"`
	Secret      bool   `json:"secret,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	HelpURL     string `json:"help_url,omitempty"`
}

type ValidateSpec struct {
	HTTP *HTTPValidate `json:"http,omitempty"`
}

func (in *ValidateSpec) DeepCopyInto(out *ValidateSpec) {
	*out = *in
	if in.HTTP != nil {
		out.HTTP = new(HTTPValidate)
		in.HTTP.DeepCopyInto(out.HTTP)
	}
}

type HTTPValidate struct {
	Method        string            `json:"method,omitempty"`
	URL           string            `json:"url,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	SuccessStatus []int             `json:"success_status,omitempty"`
	Extract       ExtractSpec       `json:"extract,omitempty"`
	Timeout       string            `json:"timeout,omitempty"`
}

func (in *HTTPValidate) DeepCopyInto(out *HTTPValidate) {
	*out = *in
	if in.Headers != nil {
		out.Headers = make(map[string]string, len(in.Headers))
		for k, v := range in.Headers {
			out.Headers[k] = v
		}
	}
	if in.SuccessStatus != nil {
		out.SuccessStatus = make([]int, len(in.SuccessStatus))
		copy(out.SuccessStatus, in.SuccessStatus)
	}
	out.Extract = in.Extract
}

type ExtractSpec struct {
	ExternalAccountID string `json:"external_account_id,omitempty"`
	ExternalLogin     string `json:"external_login,omitempty"`
	AccountType       string `json:"account_type,omitempty"`
}

type SidecarSpec struct {
	URL     string `json:"url,omitempty"`
	Timeout string `json:"timeout,omitempty"`
}

type Contributions struct {
	Env        map[string]string `json:"env,omitempty"`
	MCPServers []MCPRef          `json:"mcp_servers,omitempty"`
}

func (in *Contributions) DeepCopyInto(out *Contributions) {
	*out = *in
	if in.Env != nil {
		out.Env = make(map[string]string, len(in.Env))
		for k, v := range in.Env {
			out.Env[k] = v
		}
	}
	if in.MCPServers != nil {
		out.MCPServers = make([]MCPRef, len(in.MCPServers))
		for i := range in.MCPServers {
			in.MCPServers[i].DeepCopyInto(&out.MCPServers[i])
		}
	}
}

type MCPRef struct {
	Name     string            `json:"name,omitempty"`
	Ref      string            `json:"ref,omitempty"`
	External *ExternalMCP      `json:"external,omitempty"`
	EnvFrom  map[string]string `json:"env_from,omitempty"`
}

func (in *MCPRef) DeepCopyInto(out *MCPRef) {
	*out = *in
	if in.External != nil {
		out.External = new(ExternalMCP)
		*out.External = *in.External
	}
	if in.EnvFrom != nil {
		out.EnvFrom = make(map[string]string, len(in.EnvFrom))
		for k, v := range in.EnvFrom {
			out.EnvFrom[k] = v
		}
	}
}

type ExternalMCP struct {
	URL        string `json:"url,omitempty"`
	AuthHeader string `json:"auth_header,omitempty"`
}

type IntegrationPluginStatus struct {
	Phase        string      `json:"phase,omitempty"`
	LastSyncedAt metav1.Time `json:"lastSyncedAt,omitempty"`
	Message      string      `json:"message,omitempty"`
}

type IntegrationPluginList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IntegrationPlugin `json:"items"`
}

func (in *IntegrationPluginList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(IntegrationPluginList)
	in.DeepCopyInto(out)
	return out
}

func (in *IntegrationPluginList) DeepCopy() *IntegrationPluginList {
	if in == nil {
		return nil
	}
	out := new(IntegrationPluginList)
	in.DeepCopyInto(out)
	return out
}

func (in *IntegrationPluginList) DeepCopyInto(out *IntegrationPluginList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]IntegrationPlugin, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

// MCPServer is a namespace-scoped CRD that defines an in-cluster MCP server deployment.
type MCPServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              MCPServerSpec   `json:"spec"`
	Status            MCPServerStatus `json:"status,omitempty"`
}

func (in *MCPServer) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(MCPServer)
	in.DeepCopyInto(out)
	return out
}

func (in *MCPServer) DeepCopy() *MCPServer {
	if in == nil {
		return nil
	}
	out := new(MCPServer)
	in.DeepCopyInto(out)
	return out
}

func (in *MCPServer) DeepCopyInto(out *MCPServer) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

type MCPServerSpec struct {
	Image              string                      `json:"image"`
	Port               int32                       `json:"port"`
	Replicas           int32                       `json:"replicas,omitempty"`
	Env                []corev1.EnvVar             `json:"env,omitempty"`
	Resources          corev1.ResourceRequirements `json:"resources,omitempty"`
	ServiceAccountName string                      `json:"serviceAccountName,omitempty"`
	NetworkPolicy      MCPNetworkPolicySpec        `json:"networkPolicy,omitempty"`
}

func (in *MCPServerSpec) DeepCopyInto(out *MCPServerSpec) {
	*out = *in
	if in.Env != nil {
		out.Env = make([]corev1.EnvVar, len(in.Env))
		for i := range in.Env {
			in.Env[i].DeepCopyInto(&out.Env[i])
		}
	}
	in.Resources.DeepCopyInto(&out.Resources)
	in.NetworkPolicy.DeepCopyInto(&out.NetworkPolicy)
}

type MCPNetworkPolicySpec struct {
	EgressCIDRs []string `json:"egressCIDRs,omitempty"`
}

func (in *MCPNetworkPolicySpec) DeepCopyInto(out *MCPNetworkPolicySpec) {
	*out = *in
	if in.EgressCIDRs != nil {
		out.EgressCIDRs = make([]string, len(in.EgressCIDRs))
		copy(out.EgressCIDRs, in.EgressCIDRs)
	}
}

type MCPServerStatus struct {
	Phase      string             `json:"phase,omitempty"`
	Replicas   int32              `json:"replicas,omitempty"`
	ServiceURL string             `json:"serviceURL,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

func (in *MCPServerStatus) DeepCopyInto(out *MCPServerStatus) {
	*out = *in
	if in.Conditions != nil {
		out.Conditions = make([]metav1.Condition, len(in.Conditions))
		for i := range in.Conditions {
			in.Conditions[i].DeepCopyInto(&out.Conditions[i])
		}
	}
}

type MCPServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MCPServer `json:"items"`
}

func (in *MCPServerList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(MCPServerList)
	in.DeepCopyInto(out)
	return out
}

func (in *MCPServerList) DeepCopy() *MCPServerList {
	if in == nil {
		return nil
	}
	out := new(MCPServerList)
	in.DeepCopyInto(out)
	return out
}

func (in *MCPServerList) DeepCopyInto(out *MCPServerList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]MCPServer, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}
