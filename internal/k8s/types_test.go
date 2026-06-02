package k8s

import (
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func sampleIntegrationPlugin() *IntegrationPlugin {
	return &IntegrationPlugin{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "shclop.io/v1alpha1",
			Kind:       "IntegrationPlugin",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "github",
		},
		Spec: IntegrationPluginSpec{
			ID:              "github",
			DisplayName:     "GitHub",
			PluginKind:      "http_template",
			ScopesSupported: []string{"user"},
			Auth: AuthSpec{
				Type: "pat_token",
				Fields: []FormField{
					{Name: "token", Label: "Personal Access Token", Secret: true},
				},
			},
			Validate: ValidateSpec{
				HTTP: &HTTPValidate{
					Method:        "GET",
					URL:           "https://api.github.com/user",
					Headers:       map[string]string{"Authorization": "Bearer {{.Token}}"},
					SuccessStatus: []int{200},
					Timeout:       "5s",
				},
			},
			Contributions: Contributions{
				Env: map[string]string{"GITHUB_TOKEN": "{{.Token}}"},
			},
		},
	}
}

func sampleMCPServer() *MCPServer {
	return &MCPServer{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "shclop.io/v1alpha1",
			Kind:       "MCPServer",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-mcp",
			Namespace: "default",
		},
		Spec: MCPServerSpec{
			Image:    "ghcr.io/example/mcp:latest",
			Port:     8080,
			Replicas: 1,
			NetworkPolicy: MCPNetworkPolicySpec{
				EgressCIDRs: []string{"0.0.0.0/0"},
			},
		},
		Status: MCPServerStatus{
			Phase:      "Running",
			Replicas:   1,
			ServiceURL: "http://mcp-my-mcp.default.svc.cluster.local:8080",
			Conditions: []metav1.Condition{
				{
					Type:               "Ready",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.NewTime(time.Now().Truncate(time.Second)),
					Reason:             "DeploymentReady",
				},
			},
		},
	}
}

func TestIntegrationPluginDeepCopy_DifferentPointer(t *testing.T) {
	orig := sampleIntegrationPlugin()
	cp := orig.DeepCopy()

	if orig == cp {
		t.Fatal("DeepCopy returned same pointer")
	}
	if !reflect.DeepEqual(orig, cp) {
		t.Fatal("DeepCopy result not deeply equal to original")
	}
}

func TestIntegrationPluginDeepCopy_Isolation(t *testing.T) {
	orig := sampleIntegrationPlugin()
	cp := orig.DeepCopy()

	cp.Spec.Auth.Fields[0].Name = "mutated"
	if orig.Spec.Auth.Fields[0].Name == "mutated" {
		t.Fatal("DeepCopy did not isolate Auth.Fields slice")
	}

	cp.Spec.Validate.HTTP.Headers["Authorization"] = "mutated"
	if orig.Spec.Validate.HTTP.Headers["Authorization"] == "mutated" {
		t.Fatal("DeepCopy did not isolate Validate.HTTP.Headers map")
	}

	cp.Spec.Contributions.Env["GITHUB_TOKEN"] = "mutated"
	if orig.Spec.Contributions.Env["GITHUB_TOKEN"] == "mutated" {
		t.Fatal("DeepCopy did not isolate Contributions.Env map")
	}

	cp.Spec.ScopesSupported[0] = "mutated"
	if orig.Spec.ScopesSupported[0] == "mutated" {
		t.Fatal("DeepCopy did not isolate ScopesSupported slice")
	}
}

func TestIntegrationPluginDeepCopyObject_Kind(t *testing.T) {
	orig := sampleIntegrationPlugin()
	obj := orig.DeepCopyObject()
	if obj == nil {
		t.Fatal("DeepCopyObject returned nil")
	}
	plugin, ok := obj.(*IntegrationPlugin)
	if !ok {
		t.Fatalf("DeepCopyObject returned %T, want *IntegrationPlugin", obj)
	}
	if plugin.Kind != orig.Kind {
		t.Fatalf("kind mismatch: got %q, want %q", plugin.Kind, orig.Kind)
	}
	if plugin == orig {
		t.Fatal("DeepCopyObject returned same pointer")
	}
}

func TestIntegrationPluginListDeepCopy(t *testing.T) {
	list := &IntegrationPluginList{
		Items: []IntegrationPlugin{*sampleIntegrationPlugin()},
	}
	cp := list.DeepCopy()
	if !reflect.DeepEqual(list, cp) {
		t.Fatal("list DeepCopy not deeply equal")
	}
	cp.Items[0].Spec.ID = "mutated"
	if list.Items[0].Spec.ID == "mutated" {
		t.Fatal("list DeepCopy did not isolate Items slice")
	}

	obj := list.DeepCopyObject()
	if _, ok := obj.(*IntegrationPluginList); !ok {
		t.Fatalf("list DeepCopyObject returned %T", obj)
	}
}

func TestMCPServerDeepCopy_DifferentPointer(t *testing.T) {
	orig := sampleMCPServer()
	cp := orig.DeepCopy()

	if orig == cp {
		t.Fatal("DeepCopy returned same pointer")
	}
	if !reflect.DeepEqual(orig, cp) {
		t.Fatalf("DeepCopy result not deeply equal\norig: %+v\ncopy: %+v", orig, cp)
	}
}

func TestMCPServerDeepCopy_Isolation(t *testing.T) {
	orig := sampleMCPServer()
	cp := orig.DeepCopy()

	cp.Spec.NetworkPolicy.EgressCIDRs[0] = "mutated"
	if orig.Spec.NetworkPolicy.EgressCIDRs[0] == "mutated" {
		t.Fatal("DeepCopy did not isolate NetworkPolicy.EgressCIDRs")
	}

	cp.Status.Conditions[0].Reason = "mutated"
	if orig.Status.Conditions[0].Reason == "mutated" {
		t.Fatal("DeepCopy did not isolate Status.Conditions")
	}
}

func TestMCPServerDeepCopyObject_Kind(t *testing.T) {
	orig := sampleMCPServer()
	obj := orig.DeepCopyObject()
	if obj == nil {
		t.Fatal("DeepCopyObject returned nil")
	}
	server, ok := obj.(*MCPServer)
	if !ok {
		t.Fatalf("DeepCopyObject returned %T, want *MCPServer", obj)
	}
	if server.Kind != orig.Kind {
		t.Fatalf("kind mismatch: got %q, want %q", server.Kind, orig.Kind)
	}
}

func TestMCPServerListDeepCopy(t *testing.T) {
	list := &MCPServerList{
		Items: []MCPServer{*sampleMCPServer()},
	}
	cp := list.DeepCopy()
	if !reflect.DeepEqual(list, cp) {
		t.Fatal("list DeepCopy not deeply equal")
	}
	cp.Items[0].Spec.Image = "mutated"
	if list.Items[0].Spec.Image == "mutated" {
		t.Fatal("list DeepCopy did not isolate Items slice")
	}

	obj := list.DeepCopyObject()
	if _, ok := obj.(*MCPServerList); !ok {
		t.Fatalf("list DeepCopyObject returned %T", obj)
	}
}

func TestUnstructuredRoundTrip_IntegrationPlugin(t *testing.T) {
	orig := sampleIntegrationPlugin()

	raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(orig)
	if err != nil {
		t.Fatalf("ToUnstructured: %v", err)
	}

	u := &unstructured.Unstructured{Object: raw}
	got, err := UnstructuredToIntegrationPlugin(u)
	if err != nil {
		t.Fatalf("UnstructuredToIntegrationPlugin: %v", err)
	}

	// TypeMeta is not preserved through unstructured conversion by default;
	// compare the spec and name which are the meaningful payload.
	if got.Name != orig.Name {
		t.Errorf("name: got %q, want %q", got.Name, orig.Name)
	}
	if got.Spec.ID != orig.Spec.ID {
		t.Errorf("spec.id: got %q, want %q", got.Spec.ID, orig.Spec.ID)
	}
	if got.Spec.DisplayName != orig.Spec.DisplayName {
		t.Errorf("spec.display_name: got %q, want %q", got.Spec.DisplayName, orig.Spec.DisplayName)
	}
	if len(got.Spec.Auth.Fields) != len(orig.Spec.Auth.Fields) {
		t.Errorf("auth.fields len: got %d, want %d", len(got.Spec.Auth.Fields), len(orig.Spec.Auth.Fields))
	}
	if got.Spec.Validate.HTTP == nil {
		t.Fatal("validate.http is nil after round-trip")
	}
	if got.Spec.Validate.HTTP.URL != orig.Spec.Validate.HTTP.URL {
		t.Errorf("validate.http.url: got %q, want %q", got.Spec.Validate.HTTP.URL, orig.Spec.Validate.HTTP.URL)
	}
	if v, ok := got.Spec.Contributions.Env["GITHUB_TOKEN"]; !ok || v != orig.Spec.Contributions.Env["GITHUB_TOKEN"] {
		t.Errorf("contributions.env[GITHUB_TOKEN]: got %q", v)
	}
}

func TestUnstructuredRoundTrip_MCPServer(t *testing.T) {
	orig := sampleMCPServer()
	orig.Status.Conditions = nil // avoid time serialization edge cases

	raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(orig)
	if err != nil {
		t.Fatalf("ToUnstructured: %v", err)
	}

	u := &unstructured.Unstructured{Object: raw}
	got, err := UnstructuredToMCPServer(u)
	if err != nil {
		t.Fatalf("UnstructuredToMCPServer: %v", err)
	}

	if got.Name != orig.Name {
		t.Errorf("name: got %q, want %q", got.Name, orig.Name)
	}
	if got.Spec.Image != orig.Spec.Image {
		t.Errorf("spec.image: got %q, want %q", got.Spec.Image, orig.Spec.Image)
	}
	if got.Spec.Port != orig.Spec.Port {
		t.Errorf("spec.port: got %d, want %d", got.Spec.Port, orig.Spec.Port)
	}
	if len(got.Spec.NetworkPolicy.EgressCIDRs) != 1 || got.Spec.NetworkPolicy.EgressCIDRs[0] != "0.0.0.0/0" {
		t.Errorf("egressCIDRs: got %v", got.Spec.NetworkPolicy.EgressCIDRs)
	}
}
