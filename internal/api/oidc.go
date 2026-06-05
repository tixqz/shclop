package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mipopov/shclop/internal/auth"
	"github.com/mipopov/shclop/internal/domain"
	"github.com/mipopov/shclop/internal/store"
	"golang.org/x/oauth2"
)

const oidcStateCookieName = "shclop_oidc_state"

func (s *Server) handleListAuthProviders(w http.ResponseWriter, r *http.Request) {
	providers := s.idpRegistry.List()
	summaries := make([]domain.IdPProviderSummary, 0, len(providers))
	for _, p := range providers {
		if !p.Enabled {
			continue
		}
		summaries = append(summaries, domain.IdPProviderSummary{
			Name:        p.Config.Name,
			DisplayName: p.Config.DisplayName,
			Status:      string(p.Status),
			Error:       p.LastErr,
			Enabled:     p.Enabled,
		})
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"providers": summaries})
}

func (s *Server) handleOIDCRoute(w http.ResponseWriter, r *http.Request) {
	suffix := strings.TrimPrefix(r.URL.Path, "/api/auth/oidc/")
	parts := strings.SplitN(suffix, "/", 2)
	if len(parts) != 2 || parts[1] == "" {
		http.NotFound(w, r)
		return
	}
	action := parts[0]
	provider := parts[1]
	switch action {
	case "login":
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		s.handleOIDCLogin(w, r, provider)
	case "callback":
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		s.handleOIDCCallback(w, r, provider)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleOIDCLogin(w http.ResponseWriter, r *http.Request, providerName string) {
	entry, ok := s.idpRegistry.Get(providerName)
	if !ok || !entry.Enabled || entry.Status != "ready" {
		http.Error(w, "provider not available", http.StatusNotFound)
		return
	}

	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	state := base64.RawURLEncoding.EncodeToString(stateBytes)

	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	nonce := base64.RawURLEncoding.EncodeToString(nonceBytes)

	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	codeVerifier := base64.RawURLEncoding.EncodeToString(verifierBytes)
	sum := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(sum[:])

	returnTo := sanitizeReturnTo(r.URL.Query().Get("return_to"))

	stateCookie := auth.OIDCStateCookie{
		State:        state,
		Nonce:        nonce,
		CodeVerifier: codeVerifier,
		Provider:     providerName,
		ReturnTo:     returnTo,
		ExpiresAt:    time.Now().Add(10 * time.Minute),
	}
	encoded, err := s.cookieCodec.Encode(stateCookie)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookieName,
		Value:    encoded,
		Path:     "/api/auth/oidc/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})

	authURL := entry.Provider.OAuth2.AuthCodeURL(
		state,
		oauth2.SetAuthURLParam("nonce", nonce),
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request, providerName string) {
	cookie, err := r.Cookie(oidcStateCookieName)
	if err != nil {
		http.Error(w, "missing state cookie", http.StatusBadRequest)
		return
	}

	stateCookie, err := s.cookieCodec.Decode(cookie.Value)
	if errors.Is(err, auth.ErrExpired) {
		http.Error(w, "state cookie expired", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, "invalid state cookie", http.StatusBadRequest)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookieName,
		Value:    "",
		Path:     "/api/auth/oidc/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	if stateCookie.Provider != providerName {
		http.Error(w, "provider mismatch", http.StatusBadRequest)
		return
	}

	if r.URL.Query().Get("state") != stateCookie.State {
		http.Error(w, "state mismatch", http.StatusBadRequest)
		return
	}

	errParam := r.URL.Query().Get("error")
	if errParam != "" {
		http.Error(w, "oidc error: "+errParam, http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	entry, ok := s.idpRegistry.Get(providerName)
	if !ok || !entry.Enabled || entry.Status != "ready" {
		http.Error(w, "provider not available", http.StatusBadRequest)
		return
	}

	token, err := entry.Provider.OAuth2.Exchange(
		r.Context(),
		code,
		oauth2.SetAuthURLParam("code_verifier", stateCookie.CodeVerifier),
	)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("oidc token exchange failed", "provider", providerName, "err", err)
		}
		http.Error(w, "token exchange failed", http.StatusBadRequest)
		return
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		http.Error(w, "missing id_token", http.StatusBadRequest)
		return
	}

	idToken, err := entry.Provider.Verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		http.Error(w, "id token verification failed", http.StatusBadRequest)
		return
	}

	if idToken.Nonce != stateCookie.Nonce {
		http.Error(w, "nonce mismatch", http.StatusBadRequest)
		return
	}

	identity, err := entry.Provider.IdentityFromIDToken(idToken)
	if err != nil {
		http.Error(w, "identity extraction failed", http.StatusBadRequest)
		return
	}

	user, err := s.linkOrCreateUser(r.Context(), providerName, identity.Subject, identity.Email, identity.Name)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("oidc link/create user failed", "provider", providerName, "sub", identity.Subject, "err", err)
		}
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	sessionToken, err := s.auth.IssueToken(user)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "shclop_session",
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	s.recordActivity("auth.sso_login", user.ID, "", "sso login succeeded", map[string]any{
		"provider": providerName,
		"subject":  identity.Subject,
	})

	returnTo := stateCookie.ReturnTo
	if returnTo == "" {
		returnTo = "/"
	}
	http.Redirect(w, r, returnTo, http.StatusFound)
}

func (s *Server) linkOrCreateUser(ctx context.Context, providerName, subject, email, displayName string) (domain.User, error) {
	userID, found, err := s.store.GetUserIDByIdentity(ctx, providerName, subject)
	if err != nil {
		return domain.User{}, err
	}
	if found {
		if err := s.store.TouchIdentityLogin(ctx, providerName, subject); err != nil && s.logger != nil {
			s.logger.Warn("touch identity login failed", "err", err)
		}
		user, err := s.store.GetUser(ctx, userID)
		if err != nil {
			return domain.User{}, err
		}
		if user.Disabled {
			return domain.User{}, errors.New("account disabled")
		}
		return user, nil
	}

	existingByEmail, err := s.store.GetUserByUsername(ctx, email)
	if err == nil && existingByEmail.ID != "" {
		return domain.User{}, errors.New("email already registered with a local account")
	}

	newUser, err := s.store.CreateUserAndIdentity(ctx, store.CreateUserAndIdentityArgs{
		Username:     email,
		Role:         "user",
		ProviderName: providerName,
		Subject:      subject,
		Email:        email,
		DisplayName:  displayName,
	})
	if err != nil {
		return domain.User{}, err
	}
	return newUser, nil
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := ""
	if cookie, err := r.Cookie("shclop_session"); err == nil {
		token = strings.TrimSpace(cookie.Value)
	}
	if token == "" {
		const prefix = "Bearer "
		if authorization := r.Header.Get("Authorization"); strings.HasPrefix(authorization, prefix) {
			token = strings.TrimSpace(strings.TrimPrefix(authorization, prefix))
		}
	}

	if token != "" {
		s.auth.Revoke(token)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "shclop_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	s.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleGetAuthSettings(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if user.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	providers := s.idpRegistry.List()
	summaries := make([]domain.IdPProviderSummary, 0, len(providers))
	for _, p := range providers {
		summaries = append(summaries, domain.IdPProviderSummary{
			Name:        p.Config.Name,
			DisplayName: p.Config.DisplayName,
			Status:      string(p.Status),
			Error:       p.LastErr,
			Enabled:     p.Enabled,
			Diagnostics: &domain.IdPProviderDiagnostics{
				Issuer:      p.Config.Issuer,
				ClientID:    p.Config.ClientID,
				RedirectURI: p.Config.RedirectURI,
				Scopes:      append([]string(nil), p.Config.Scopes...),
				EmailClaim:  p.Config.EmailClaim,
				NameClaim:   p.Config.NameClaim,
				GroupsClaim: p.Config.GroupsClaim,
				SecretSet:   p.Config.ClientSecret != "",
			},
		})
	}

	settings := domain.AuthSettings{
		Mode:      s.idpRegistry.Mode(),
		Providers: summaries,
	}
	s.writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handlePatchAuthSettings(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if user.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		Mode     *string          `json:"mode"`
		Providers []struct {
			Name    string `json:"name"`
			Enabled *bool  `json:"enabled"`
		} `json:"providers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if req.Mode != nil {
		mode := domain.AuthMode(*req.Mode)
		switch mode {
		case domain.AuthModeLocal, domain.AuthModeSSO, domain.AuthModeBoth:
		default:
			http.Error(w, "bad request: invalid mode", http.StatusBadRequest)
			return
		}
		if err := s.store.SetAppSetting(r.Context(), "auth.mode", string(mode)); err != nil {
			s.writeStoreError(w, err)
			return
		}
	}

	for _, p := range req.Providers {
		if p.Enabled == nil {
			continue
		}
		val := "false"
		if *p.Enabled {
			val = "true"
		}
		key := "auth.idp." + p.Name + ".enabled"
		if err := s.store.SetAppSetting(r.Context(), key, val); err != nil {
			s.writeStoreError(w, err)
			return
		}
	}

	if err := s.idpRegistry.Reload(r.Context()); err != nil && s.logger != nil {
		s.logger.Warn("auth settings reload failed", "err", err)
	}

	s.handleGetAuthSettings(w, r)
}

func (s *Server) handleAdminListIdentities(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	caller, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if caller.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	identities, err := s.store.ListUserIdentities(r.Context(), userID)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"identities": toIdentityJSON(identities)})
}

func (s *Server) handleAdminLinkIdentity(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	caller, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if caller.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		ProviderName string `json:"provider_name"`
		Subject      string `json:"subject"`
		Email        string `json:"email"`
		DisplayName  string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.ProviderName == "" || req.Subject == "" {
		http.Error(w, "bad request: provider_name and subject required", http.StatusBadRequest)
		return
	}

	if err := s.store.LinkIdentityToUser(r.Context(), userID, req.ProviderName, req.Subject, req.Email, req.DisplayName); err != nil {
		if errors.Is(err, store.ErrConflict) {
			http.Error(w, "identity already linked", http.StatusConflict)
			return
		}
		if errors.Is(err, store.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		s.writeStoreError(w, err)
		return
	}

	identities, err := s.store.ListUserIdentities(r.Context(), userID)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	s.writeJSON(w, http.StatusCreated, map[string]any{"identities": toIdentityJSON(identities)})
}

func (s *Server) handleAdminUnlinkIdentity(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	providerName := r.PathValue("provider")
	subject := r.PathValue("subject")
	caller, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if caller.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := s.store.UnlinkIdentity(r.Context(), userID, providerName, subject); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		s.writeStoreError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type userIdentityJSON struct {
	ProviderName string     `json:"provider_name"`
	Subject      string     `json:"subject"`
	UserID       string     `json:"user_id"`
	Email        string     `json:"email"`
	DisplayName  string     `json:"display_name"`
	LinkedAt     time.Time  `json:"linked_at"`
	LastLoginAt  *time.Time `json:"last_login_at"`
}

func toIdentityJSON(ids []domain.UserIdentity) []userIdentityJSON {
	out := make([]userIdentityJSON, len(ids))
	for i, id := range ids {
		out[i] = userIdentityJSON{
			ProviderName: id.ProviderName,
			Subject:      id.Subject,
			UserID:       id.UserID,
			Email:        id.Email,
			DisplayName:  id.DisplayName,
			LinkedAt:     id.LinkedAt,
			LastLoginAt:  id.LastLoginAt,
		}
	}
	return out
}

func sanitizeReturnTo(returnTo string) string {
	if returnTo == "" {
		return ""
	}
	parsed, err := url.Parse(returnTo)
	if err != nil {
		return ""
	}
	if parsed.Host != "" {
		return ""
	}
	if !strings.HasPrefix(parsed.Path, "/") {
		return ""
	}
	return parsed.Path
}
