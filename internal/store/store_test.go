package store

import (
	"context"
	"testing"
	"time"

	"github.com/mipopov/shclop/internal/domain"
)

func TestMemoryStore_IntegrationConnection(t *testing.T) {
	s := NewMemory()
	ctx := context.Background()

	// Initially no connection
	_, err := s.GetIntegrationConnection(ctx, "user-1", "github")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for missing connection, got %v", err)
	}

	// Create a connection
	conn := domain.IntegrationConnection{
		ProviderID:        "github",
		UserID:            "user-1",
		ExternalAccountID: "12345",
		ExternalLogin:     "testuser",
		AccountType:       "User",
		Status:            "connected",
		Secret:            "encrypted_token_here",
		Revision:          1,
	}
	saved, err := s.UpsertIntegrationConnection(ctx, conn)
	if err != nil {
		t.Fatalf("UpsertIntegrationConnection: %v", err)
	}
	if saved.Revision != 1 {
		t.Fatalf("expected revision 1, got %d", saved.Revision)
	}
	if saved.ExternalLogin != "testuser" {
		t.Fatalf("expected external_login testuser, got %q", saved.ExternalLogin)
	}

	// Fetch
	fetched, err := s.GetIntegrationConnection(ctx, "user-1", "github")
	if err != nil {
		t.Fatalf("GetIntegrationConnection: %v", err)
	}
	if fetched.Secret != "encrypted_token_here" {
		t.Fatalf("secret not preserved: got %q", fetched.Secret)
	}

	// Upsert with updated data — revision should auto-increment
	conn.Revision = 0 // should be ignored/auto-set
	conn.ExternalLogin = "testuser2"
	conn.Secret = "new_encrypted_token"
	updated, err := s.UpsertIntegrationConnection(ctx, conn)
	if err != nil {
		t.Fatalf("UpsertIntegrationConnection update: %v", err)
	}
	if updated.Revision != 2 {
		t.Fatalf("expected revision 2 after update, got %d", updated.Revision)
	}
	if updated.ExternalLogin != "testuser2" {
		t.Fatalf("expected external_login testuser2, got %q", updated.ExternalLogin)
	}

	// Delete
	if err := s.DeleteIntegrationConnection(ctx, "user-1", "github"); err != nil {
		t.Fatalf("DeleteIntegrationConnection: %v", err)
	}
	_, err = s.GetIntegrationConnection(ctx, "user-1", "github")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestMemoryStore_AgentIntegration(t *testing.T) {
	s := NewMemory()
	ctx := context.Background()

	// Upsert agent integration (toggle on)
	enabled, err := s.UpsertAgentIntegration(ctx, "agent-1", "github", true, 0, "active")
	if err != nil {
		t.Fatalf("UpsertAgentIntegration: %v", err)
	}
	if !enabled.Enabled {
		t.Fatal("expected enabled = true")
	}
	if enabled.Revision != 1 {
		t.Fatalf("expected revision 1, got %d", enabled.Revision)
	}

	// Get it
	got, err := s.GetAgentIntegration(ctx, "agent-1", "github")
	if err != nil {
		t.Fatalf("GetAgentIntegration: %v", err)
	}
	if !got.Enabled {
		t.Fatal("expected enabled = true")
	}

	// Toggle off
	disabled, err := s.UpsertAgentIntegration(ctx, "agent-1", "github", false, 0, "disabled")
	if err != nil {
		t.Fatalf("UpsertAgentIntegration toggle off: %v", err)
	}
	if disabled.Enabled {
		t.Fatal("expected enabled = false")
	}
	if disabled.Revision != 2 {
		t.Fatalf("expected revision 2, got %d", disabled.Revision)
	}
	if disabled.Status != "disabled" {
		t.Fatalf("expected status disabled, got %q", disabled.Status)
	}

	// List agent integrations
	list, err := s.ListAgentIntegrations(ctx, "agent-1")
	if err != nil {
		t.Fatalf("ListAgentIntegrations: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 integration, got %d", len(list))
	}
	if list[0].ProviderID != "github" {
		t.Fatalf("expected github, got %q", list[0].ProviderID)
	}

	// Get non-existent
	_, err = s.GetAgentIntegration(ctx, "agent-2", "github")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMemoryStore_PluginManifestCRUD(t *testing.T) {
	s := NewMemory()
	ctx := context.Background()

	// Not found initially
	_, err := s.GetPluginManifest(ctx, "github")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for missing manifest, got %v", err)
	}

	// Insert
	pm := domain.PluginManifest{
		ID:      "github",
		YAML:    "spec: {id: github}",
		Enabled: true,
	}
	saved, err := s.UpsertPluginManifest(ctx, pm)
	if err != nil {
		t.Fatalf("UpsertPluginManifest insert: %v", err)
	}
	if saved.Revision != 1 {
		t.Fatalf("expected revision 1, got %d", saved.Revision)
	}
	if saved.CreatedAt.IsZero() || saved.UpdatedAt.IsZero() {
		t.Fatal("expected timestamps to be set on insert")
	}
	createdAt := saved.CreatedAt

	// Get
	got, err := s.GetPluginManifest(ctx, "github")
	if err != nil {
		t.Fatalf("GetPluginManifest: %v", err)
	}
	if got.YAML != "spec: {id: github}" {
		t.Fatalf("unexpected YAML: %q", got.YAML)
	}

	// Update — revision should increment, CreatedAt unchanged
	time.Sleep(time.Millisecond)
	pm.YAML = "spec: {id: github, v: 2}"
	updated, err := s.UpsertPluginManifest(ctx, pm)
	if err != nil {
		t.Fatalf("UpsertPluginManifest update: %v", err)
	}
	if updated.Revision != 2 {
		t.Fatalf("expected revision 2 after update, got %d", updated.Revision)
	}
	if !updated.CreatedAt.Equal(createdAt) {
		t.Fatal("expected CreatedAt to remain unchanged on upsert")
	}
	if updated.YAML != "spec: {id: github, v: 2}" {
		t.Fatalf("unexpected YAML after update: %q", updated.YAML)
	}

	// List all
	list, err := s.ListPluginManifests(ctx, false)
	if err != nil {
		t.Fatalf("ListPluginManifests: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(list))
	}

	// List enabled only — should still return 1
	enabledList, err := s.ListPluginManifests(ctx, true)
	if err != nil {
		t.Fatalf("ListPluginManifests enabled: %v", err)
	}
	if len(enabledList) != 1 {
		t.Fatalf("expected 1 enabled manifest, got %d", len(enabledList))
	}

	// Disable and list enabled — should return 0
	pm.Enabled = false
	_, err = s.UpsertPluginManifest(ctx, pm)
	if err != nil {
		t.Fatalf("UpsertPluginManifest disable: %v", err)
	}
	disabledList, err := s.ListPluginManifests(ctx, true)
	if err != nil {
		t.Fatalf("ListPluginManifests after disable: %v", err)
	}
	if len(disabledList) != 0 {
		t.Fatalf("expected 0 enabled manifests after disable, got %d", len(disabledList))
	}

	// Delete
	if err := s.DeletePluginManifest(ctx, "github"); err != nil {
		t.Fatalf("DeletePluginManifest: %v", err)
	}
	_, err = s.GetPluginManifest(ctx, "github")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}

	// Delete non-existent
	if err := s.DeletePluginManifest(ctx, "nonexistent"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for missing manifest delete, got %v", err)
	}
}

func TestMemoryStore_IntegrationConnection_UpsertRevisionAutoIncrement(t *testing.T) {
	s := NewMemory()
	ctx := context.Background()

	// First insert
	c1 := domain.IntegrationConnection{
		ProviderID: "github", UserID: "u1", ExternalAccountID: "1",
		Status: "connected", Secret: "s1",
	}
	saved1, err := s.UpsertIntegrationConnection(ctx, c1)
	if err != nil {
		t.Fatal(err)
	}
	if saved1.Revision != 1 {
		t.Fatalf("expected revision 1, got %d", saved1.Revision)
	}
	if saved1.CreatedAt.IsZero() || saved1.UpdatedAt.IsZero() {
		t.Fatal("expected timestamps to be set")
	}
	createdAt := saved1.CreatedAt

	// Wait a tiny bit to ensure time changes
	time.Sleep(time.Millisecond)

	// Second upsert
	c2 := domain.IntegrationConnection{
		ProviderID: "github", UserID: "u1", ExternalAccountID: "1",
		Status: "connected", Secret: "s2",
	}
	saved2, err := s.UpsertIntegrationConnection(ctx, c2)
	if err != nil {
		t.Fatal(err)
	}
	if saved2.Revision != 2 {
		t.Fatalf("expected revision 2, got %d", saved2.Revision)
	}
	if !saved2.CreatedAt.Equal(createdAt) {
		t.Fatal("expected CreatedAt to remain unchanged on upsert")
	}
	if saved2.UpdatedAt.Before(saved1.UpdatedAt) {
		t.Fatal("expected UpdatedAt to advance on upsert")
	}
}
