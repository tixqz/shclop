package store

import (
	"context"
	"testing"
)

func TestMemoryCreateUserAndIdentity_HappyPath(t *testing.T) {
	s := NewMemory()
	ctx := context.Background()

	args := CreateUserAndIdentityArgs{
		Username:     "alice@example.com",
		Role:         "user",
		ProviderName: "oidc",
		Subject:      "sub-001",
		Email:        "alice@example.com",
		DisplayName:  "Alice",
	}
	user, err := s.CreateUserAndIdentity(ctx, args)
	if err != nil {
		t.Fatalf("CreateUserAndIdentity: %v", err)
	}
	if user.ID == "" {
		t.Fatal("expected non-empty user ID")
	}
	if user.Username != args.Username {
		t.Fatalf("expected username %q, got %q", args.Username, user.Username)
	}

	got, err := s.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got.ID != user.ID {
		t.Fatalf("user ID mismatch")
	}

	identities, err := s.ListUserIdentities(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListUserIdentities: %v", err)
	}
	if len(identities) != 1 {
		t.Fatalf("expected 1 identity, got %d", len(identities))
	}
	if identities[0].ProviderName != args.ProviderName || identities[0].Subject != args.Subject {
		t.Fatalf("identity mismatch: got %+v", identities[0])
	}
}

func TestMemoryCreateUserAndIdentity_UsernameConflict(t *testing.T) {
	s := NewMemory()
	ctx := context.Background()

	args := CreateUserAndIdentityArgs{
		Username: "bob@example.com", Role: "user",
		ProviderName: "oidc", Subject: "sub-bob",
		Email: "bob@example.com",
	}
	_, err := s.CreateUserAndIdentity(ctx, args)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	args2 := CreateUserAndIdentityArgs{
		Username: "bob@example.com", Role: "user",
		ProviderName: "oidc", Subject: "sub-bob-2",
		Email: "bob@example.com",
	}
	_, err = s.CreateUserAndIdentity(ctx, args2)
	if err != ErrConflict {
		t.Fatalf("expected ErrConflict for duplicate username, got %v", err)
	}
}

func TestMemoryCreateUserAndIdentity_IdentityConflict(t *testing.T) {
	s := NewMemory()
	ctx := context.Background()

	args := CreateUserAndIdentityArgs{
		Username: "carol@example.com", Role: "user",
		ProviderName: "oidc", Subject: "sub-carol",
		Email: "carol@example.com",
	}
	_, err := s.CreateUserAndIdentity(ctx, args)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	args2 := CreateUserAndIdentityArgs{
		Username: "carol2@example.com", Role: "user",
		ProviderName: "oidc", Subject: "sub-carol",
		Email: "carol2@example.com",
	}
	_, err = s.CreateUserAndIdentity(ctx, args2)
	if err != ErrConflict {
		t.Fatalf("expected ErrConflict for duplicate identity, got %v", err)
	}
}

func TestMemoryGetUserIDByIdentity_NotFound(t *testing.T) {
	s := NewMemory()
	ctx := context.Background()

	uid, found, err := s.GetUserIDByIdentity(ctx, "oidc", "nonexistent-sub")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected found=false")
	}
	if uid != "" {
		t.Fatalf("expected empty userID, got %q", uid)
	}
}

func TestMemoryTouchIdentityLogin_Updates(t *testing.T) {
	s := NewMemory()
	ctx := context.Background()

	args := CreateUserAndIdentityArgs{
		Username: "dave@example.com", Role: "user",
		ProviderName: "oidc", Subject: "sub-dave",
		Email: "dave@example.com",
	}
	user, err := s.CreateUserAndIdentity(ctx, args)
	if err != nil {
		t.Fatalf("CreateUserAndIdentity: %v", err)
	}

	identities, err := s.ListUserIdentities(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListUserIdentities before touch: %v", err)
	}
	if identities[0].LastLoginAt != nil {
		t.Fatal("expected LastLoginAt to be nil before touch")
	}

	if err := s.TouchIdentityLogin(ctx, "oidc", "sub-dave"); err != nil {
		t.Fatalf("TouchIdentityLogin: %v", err)
	}

	identities, err = s.ListUserIdentities(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListUserIdentities after touch: %v", err)
	}
	if identities[0].LastLoginAt == nil {
		t.Fatal("expected LastLoginAt to be non-nil after touch")
	}
}

func TestMemoryLinkIdentityToUser_NotFound(t *testing.T) {
	s := NewMemory()
	ctx := context.Background()

	err := s.LinkIdentityToUser(ctx, "nonexistent-user", "oidc", "sub-x", "", "")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMemoryUnlinkIdentity_NotFound(t *testing.T) {
	s := NewMemory()
	ctx := context.Background()

	err := s.UnlinkIdentity(ctx, "nonexistent-user", "oidc", "nonexistent-sub")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMemoryAppSettings_DefaultAuthMode(t *testing.T) {
	s := NewMemory()
	ctx := context.Background()

	val, err := s.GetAppSetting(ctx, "auth.mode")
	if err != nil {
		t.Fatalf("GetAppSetting: %v", err)
	}
	if val != "local" {
		t.Fatalf("expected default auth.mode=local, got %q", val)
	}
}

func TestMemoryAppSettings_SetGet(t *testing.T) {
	s := NewMemory()
	ctx := context.Background()

	if err := s.SetAppSetting(ctx, "auth.mode", "sso"); err != nil {
		t.Fatalf("SetAppSetting: %v", err)
	}
	val, err := s.GetAppSetting(ctx, "auth.mode")
	if err != nil {
		t.Fatalf("GetAppSetting: %v", err)
	}
	if val != "sso" {
		t.Fatalf("expected sso, got %q", val)
	}
}

func TestMemoryAppSettings_ListPrefix(t *testing.T) {
	s := NewMemory()
	ctx := context.Background()

	for k, v := range map[string]string{
		"auth.idp.x.enabled": "true",
		"auth.idp.y.enabled": "true",
		"unrelated":          "value",
	} {
		if err := s.SetAppSetting(ctx, k, v); err != nil {
			t.Fatalf("SetAppSetting %q: %v", k, err)
		}
	}

	result, err := s.ListAppSettings(ctx, "auth.idp.")
	if err != nil {
		t.Fatalf("ListAppSettings: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 keys with prefix auth.idp., got %d: %v", len(result), result)
	}
	if _, ok := result["auth.idp.x.enabled"]; !ok {
		t.Fatal("expected auth.idp.x.enabled in result")
	}
	if _, ok := result["auth.idp.y.enabled"]; !ok {
		t.Fatal("expected auth.idp.y.enabled in result")
	}
	if _, ok := result["unrelated"]; ok {
		t.Fatal("did not expect unrelated key in prefix-filtered result")
	}
}

func TestMemoryListUserIdentities_SortedAndIsolated(t *testing.T) {
	s := NewMemory()
	ctx := context.Background()

	userA, err := s.CreateUserAndIdentity(ctx, CreateUserAndIdentityArgs{
		Username: "userA@example.com", Role: "user",
		ProviderName: "oidc", Subject: "sub-a1",
		Email: "userA@example.com",
	})
	if err != nil {
		t.Fatalf("create userA first identity: %v", err)
	}

	if err := s.LinkIdentityToUser(ctx, userA.ID, "google", "sub-a2", "userA@example.com", "User A"); err != nil {
		t.Fatalf("LinkIdentityToUser userA: %v", err)
	}

	userB, err := s.CreateUserAndIdentity(ctx, CreateUserAndIdentityArgs{
		Username: "userB@example.com", Role: "user",
		ProviderName: "oidc", Subject: "sub-b1",
		Email: "userB@example.com",
	})
	if err != nil {
		t.Fatalf("create userB first identity: %v", err)
	}
	if err := s.LinkIdentityToUser(ctx, userB.ID, "github", "sub-b2", "userB@example.com", "User B"); err != nil {
		t.Fatalf("LinkIdentityToUser userB: %v", err)
	}

	identsA, err := s.ListUserIdentities(ctx, userA.ID)
	if err != nil {
		t.Fatalf("ListUserIdentities userA: %v", err)
	}
	if len(identsA) != 2 {
		t.Fatalf("expected 2 identities for userA, got %d", len(identsA))
	}
	// Should be sorted: google < oidc
	if identsA[0].ProviderName != "google" || identsA[1].ProviderName != "oidc" {
		t.Fatalf("expected sorted [google, oidc], got [%s, %s]", identsA[0].ProviderName, identsA[1].ProviderName)
	}

	identsB, err := s.ListUserIdentities(ctx, userB.ID)
	if err != nil {
		t.Fatalf("ListUserIdentities userB: %v", err)
	}
	if len(identsB) != 2 {
		t.Fatalf("expected 2 identities for userB, got %d", len(identsB))
	}
	for _, id := range identsB {
		if id.UserID != userB.ID {
			t.Fatalf("userB identity has wrong userID: %q", id.UserID)
		}
	}
}
