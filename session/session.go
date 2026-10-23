package session

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
)

type contextKey struct{}

var ErrWeakSecret = errors.New("session signing secret must be at least 32 bytes")

type Options struct {
	CookieName string
	Path       string
	Secure     bool
	SameSite   http.SameSite
	MaxAge     int
}

type Middleware struct {
	secret  []byte
	options Options
}

func New(secret []byte, options Options) (*Middleware, error) {
	if len(secret) < 32 {
		return nil, ErrWeakSecret
	}
	options.CookieName = defaultValue(options.CookieName, "gomvu_session")
	options.Path = defaultValue(options.Path, "/")
	if options.SameSite == 0 {
		options.SameSite = http.SameSiteLaxMode
	}
	return &Middleware{secret: append([]byte(nil), secret...), options: options}, nil
}

func (middleware *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, valid := middleware.readID(r)
		if !valid {
			var err error
			id, err = newID()
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			http.SetCookie(w, &http.Cookie{
				Name: middleware.options.CookieName, Value: middleware.sign(id),
				Path: middleware.options.Path, MaxAge: middleware.options.MaxAge,
				HttpOnly: true, Secure: middleware.options.Secure,
				SameSite: middleware.options.SameSite,
			})
		}
		ctx := context.WithValue(r.Context(), contextKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func ID(r *http.Request) (string, bool) {
	id, ok := r.Context().Value(contextKey{}).(string)
	return id, ok && id != ""
}

func (middleware *Middleware) readID(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(middleware.options.CookieName)
	if err != nil {
		return "", false
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 {
		return "", false
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", false
	}
	expected, _ := base64.RawURLEncoding.DecodeString(middleware.signature(parts[0]))
	if !hmac.Equal(signature, expected) {
		return "", false
	}
	id, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || len(id) != 32 {
		return "", false
	}
	return string(id), true
}

func (middleware *Middleware) sign(id string) string {
	encoded := base64.RawURLEncoding.EncodeToString([]byte(id))
	return encoded + "." + middleware.signature(encoded)
}

func (middleware *Middleware) signature(value string) string {
	mac := hmac.New(sha256.New, middleware.secret)
	_, _ = mac.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func newID() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return string(value), nil
}

func defaultValue(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
