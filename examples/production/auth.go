package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/dmundt/go-mvu/adapters/caskstore"
	"github.com/dmundt/go-mvu/httpx"
	"github.com/dmundt/go-mvu/session"
	"github.com/dmundt/go-mvu/store"
	"golang.org/x/oauth2"
)

type identity struct {
	Account string    `json:"account"`
	Expires time.Time `json:"expires"`
}

type principal struct {
	identity
	SessionID string
}

type principalKey struct{}

type loginAttempt struct {
	SessionID string
	Nonce     string
	Verifier  string
	Expires   time.Time
}

type authentication struct {
	sessions *session.Middleware
	records  *caskstore.Store[identity]
	oauth    oauth2.Config
	verifier *oidc.IDTokenVerifier
	client   *http.Client
	issuer   string
	mu       sync.Mutex
	attempts map[string]loginAttempt
}

func randomToken() (string, error) {
	var data [32]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data[:]), nil
}

func (auth *authentication) find(r *http.Request) (principal, error) {
	id, ok := session.ID(r)
	if !ok {
		return principal{}, &httpx.Error{Status: 401, Location: "/login"}
	}
	value, err := auth.records.Load(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		return principal{}, &httpx.Error{Status: 401, Location: "/login"}
	}
	if err != nil {
		return principal{}, err
	}
	if value.Model.Account == "" || !time.Now().Before(value.Model.Expires) {
		return principal{}, &httpx.Error{Status: 401, Location: "/login"}
	}
	return principal{identity: value.Model, SessionID: id}, nil
}

func (auth *authentication) protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := auth.find(r)
		if err != nil {
			httpx.RespondError(w, r, err)
			return
		}
		ctx := context.WithValue(r.Context(), principalKey{}, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func accountKey(r *http.Request) (string, error) {
	user, ok := r.Context().Value(principalKey{}).(principal)
	if !ok || user.Account == "" {
		return "", &httpx.Error{Status: 401, Location: "/login"}
	}
	return user.Account, nil
}

func (auth *authentication) authorize(
	ctx context.Context, key string, _ Model, _ Message,
) error {
	user, ok := ctx.Value(principalKey{}).(principal)
	if !ok || user.Account != key {
		return &httpx.Error{Status: 403}
	}
	// Recheck durable authorization for every CAS attempt/follow-up.
	current, err := auth.records.Load(ctx, user.SessionID)
	if errors.Is(err, store.ErrNotFound) {
		return &httpx.Error{Status: 401, Location: "/login"}
	}
	if err != nil {
		return err
	}
	if current.Model.Account != key || !time.Now().Before(current.Model.Expires) {
		return &httpx.Error{Status: 403}
	}
	return nil
}

func (auth *authentication) begin(w http.ResponseWriter, r *http.Request) {
	if _, err := auth.find(r); err == nil {
		httpx.RespondError(w, r, &httpx.Error{Status: 409, Public: "Log out first."})
		return
	} else {
		var rejection *httpx.Error
		if !errors.As(err, &rejection) || rejection.Status != 401 {
			httpx.RespondError(w, r, err)
			return
		}
	}
	id, ok := session.ID(r)
	if !ok {
		httpx.RespondError(w, r, session.ErrNoSession)
		return
	}
	state, err := randomToken()
	if err != nil {
		httpx.RespondError(w, r, err)
		return
	}
	nonce, err := randomToken()
	if err != nil {
		httpx.RespondError(w, r, err)
		return
	}
	verifier, err := randomToken()
	if err != nil {
		httpx.RespondError(w, r, err)
		return
	}
	now := time.Now()
	auth.mu.Lock()
	for key, attempt := range auth.attempts {
		if !now.Before(attempt.Expires) {
			delete(auth.attempts, key)
		}
	}
	if len(auth.attempts) >= 256 {
		auth.mu.Unlock()
		httpx.RespondError(w, r, &httpx.Error{Status: 503})
		return
	}
	auth.attempts[state] = loginAttempt{
		SessionID: id,
		Nonce:     nonce,
		Verifier:  verifier,
		Expires:   now.Add(5 * time.Minute),
	}
	auth.mu.Unlock()
	http.Redirect(w, r, auth.oauth.AuthCodeURL(state,
		oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier)), http.StatusSeeOther)
}

func (auth *authentication) callback(w http.ResponseWriter, r *http.Request) {
	id, ok := session.ID(r)
	state := r.URL.Query().Get("state")
	auth.mu.Lock()
	attempt, exists := auth.attempts[state]
	if exists && ok && attempt.SessionID == id {
		delete(auth.attempts, state)
	}
	auth.mu.Unlock()
	if !exists || !ok || attempt.SessionID != id || !time.Now().Before(attempt.Expires) ||
		r.URL.Query().Get("code") == "" || r.URL.Query().Get("error") != "" {
		httpx.RespondError(w, r, &httpx.Error{Status: 400, Public: "Invalid login callback."})
		return
	}
	ctx := oidc.ClientContext(r.Context(), auth.client)
	token, err := auth.oauth.Exchange(ctx, r.URL.Query().Get("code"),
		oauth2.VerifierOption(attempt.Verifier))
	if err != nil {
		httpx.RespondError(w, r, &httpx.Error{Status: 502, Cause: fmt.Errorf("OIDC exchange: %w", err)})
		return
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok {
		httpx.RespondError(w, r, &httpx.Error{Status: 502, Public: "Provider omitted ID token."})
		return
	}
	verified, err := auth.verifier.Verify(ctx, raw)
	if err != nil {
		httpx.RespondError(w, r, &httpx.Error{Status: 401, Cause: fmt.Errorf("OIDC verify: %w", err)})
		return
	}
	if verified.Subject == "" ||
		subtle.ConstantTimeCompare([]byte(verified.Nonce), []byte(attempt.Nonce)) != 1 {
		httpx.RespondError(w, r, &httpx.Error{Status: 401, Public: "Invalid login identity."})
		return
	}
	// Issuer + subject, never email, identify an account. No tokens are retained.
	sum := sha256.Sum256([]byte(auth.issuer + "\x00" + verified.Subject))
	user := identity{
		Account: hex.EncodeToString(sum[:]),
		Expires: minTime(verified.Expiry, time.Now().Add(8*time.Hour)),
	}
	_, err = auth.sessions.Rotate(w, r, func(_, next string) error {
		_, err := auth.records.Create(r.Context(), next, user)
		return err
	})
	if err != nil {
		httpx.RespondError(w, r, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func (auth *authentication) logout(w http.ResponseWriter, r *http.Request) {
	id, ok := session.ID(r)
	if !ok {
		httpx.RespondError(w, r, session.ErrNoSession)
		return
	}
	// Durable denial precedes cookie invalidation. Even if invalidation fails,
	// a copied cookie cannot recover this authentication after restart.
	if err := auth.records.Delete(r.Context(), id); err != nil {
		httpx.RespondError(w, r, err)
		return
	}
	if err := auth.sessions.Invalidate(w, r); err != nil {
		httpx.RespondError(w, r, err)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
