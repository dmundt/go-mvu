package theme

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strings"
)

var ErrWeakPreferenceSecret = errors.New("theme preference signing secret must be at least 32 bytes")

// ModeField and ReturnField are the form fields accepted by Handler.
const (
	ModeField   = "mode"
	ReturnField = "return"
)

type PreferenceOptions struct {
	CookieName string
	Path       string
	Secure     bool
	SameSite   http.SameSite
	MaxAge     int
}

// Preference stores the mode in an HMAC-signed, HttpOnly cookie. Cookie
// values are "system", "light", or "dark" plus a signature, so clients
// cannot forge arbitrary values.
type Preference struct {
	secret  []byte
	options PreferenceOptions
}

type preferenceContextKey struct{}

func NewPreference(secret []byte, options PreferenceOptions) (*Preference, error) {
	if len(secret) < 32 {
		return nil, ErrWeakPreferenceSecret
	}
	if options.CookieName == "" {
		options.CookieName = "gomvu_theme"
	}
	if options.Path == "" {
		options.Path = "/"
	}
	if options.SameSite == 0 {
		options.SameSite = http.SameSiteLaxMode
	}
	if err := (&http.Cookie{Name: options.CookieName, Value: "system", Path: options.Path}).Valid(); err != nil {
		return nil, err
	}
	return &Preference{secret: append([]byte(nil), secret...), options: options}, nil
}

// WithMode stores mode in ctx for ModeFromContext.
func WithMode(ctx context.Context, mode Mode) context.Context {
	return context.WithValue(ctx, preferenceContextKey{}, mode)
}

// ModeFromContext returns the mode stored by Wrap or WithMode, else System.
func ModeFromContext(ctx context.Context) Mode {
	mode, ok := ctx.Value(preferenceContextKey{}).(Mode)
	if !ok || mode > Dark {
		return System
	}
	return mode
}

// Resolve reads the signed cookie. Missing or tampered cookies yield System.
func (preference *Preference) Resolve(r *http.Request) (Mode, error) {
	cookie, err := r.Cookie(preference.options.CookieName)
	if err != nil {
		return System, nil
	}
	return preference.read(cookie.Value), nil
}

// Persist writes mode to the signed cookie.
func (preference *Preference) Persist(w http.ResponseWriter, _ *http.Request, mode Mode) error {
	if mode > Dark {
		return ErrInvalidMode
	}
	preference.Set(w, mode)
	return nil
}

// Wrap resolves the mode once per request and stores it in the context.
func (preference *Preference) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mode, _ := preference.Resolve(r)
		next.ServeHTTP(w, r.WithContext(WithMode(r.Context(), mode)))
	})
}

func (preference *Preference) Set(w http.ResponseWriter, mode Mode) {
	if mode > Dark {
		mode = System
	}
	http.SetCookie(w, &http.Cookie{
		Name: preference.options.CookieName, Value: preference.sign(mode),
		Path: preference.options.Path, MaxAge: preference.options.MaxAge,
		HttpOnly: true, Secure: preference.options.Secure,
		SameSite: preference.options.SameSite,
	})
}

// Handler accepts POST mode=system|light|dark and an optional local return
// URL. Invalid modes get 400. HTMX requests get 204 with HX-Refresh so the
// whole themed shell re-renders; plain form posts get 303 to the validated
// return URL, else fallback. Wrap with CSRF protection.
func (preference *Preference) Handler(fallback string) http.Handler {
	fallback = SafeReturnPath(fallback)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		mode, err := ParseMode(r.PostFormValue(ModeField))
		if err != nil {
			http.Error(w, "invalid theme mode", http.StatusBadRequest)
			return
		}
		preference.Set(w, mode)
		respond(w, r, returnPath(r, fallback))
	})
}

// ToggleHandler switches between light and dark and redirects to
// redirectPath. Prefer Handler for the three-choice switcher.
func (preference *Preference) ToggleHandler(redirectPath string) http.Handler {
	redirectPath = SafeReturnPath(redirectPath)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		preference.Set(w, ModeFromContext(r.Context()).Toggle())
		respond(w, r, redirectPath)
	})
}

func respond(w http.ResponseWriter, r *http.Request, location string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, location, http.StatusSeeOther)
}

func returnPath(r *http.Request, fallback string) string {
	value := r.PostFormValue(ReturnField)
	if value == "" {
		return fallback
	}
	if safe := SafeReturnPath(value); safe == value {
		return safe
	}
	return fallback
}

// SafeReturnPath returns value when it is a local absolute path such as
// "/users?page=2", else "/". It blocks open redirects like "//evil",
// "/\evil", and absolute URLs.
func SafeReturnPath(value string) string {
	if value == "" || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") ||
		strings.ContainsAny(value, "\\\x00\r\n\t") {
		return "/"
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/") {
		return "/"
	}
	return value
}

func (preference *Preference) sign(mode Mode) string {
	value := mode.String()
	mac := hmac.New(sha256.New, preference.secret)
	_, _ = mac.Write([]byte("go-mvu/theme/v1\x00"))
	_, _ = mac.Write([]byte(value))
	return value + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (preference *Preference) read(value string) Mode {
	modeValue, encodedSignature, ok := strings.Cut(value, ".")
	if !ok {
		return System
	}
	mode, err := ParseMode(modeValue)
	if err != nil {
		return System
	}
	signature, err := base64.RawURLEncoding.DecodeString(encodedSignature)
	if err != nil {
		return System
	}
	expected, _ := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(preference.sign(mode), modeValue+"."))
	if !hmac.Equal(signature, expected) {
		return System
	}
	return mode
}
