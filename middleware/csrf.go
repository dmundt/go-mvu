package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/httpx"
	"github.com/dmundt/go-mvu/session"
)

const (
	CSRFFieldName  = "_csrf"
	CSRFHeaderName = "X-CSRF-Token"
	maxFormBytes   = 1 << 20
)

var (
	ErrWeakCSRFSecret = errors.New("CSRF signing secret must be at least 32 bytes")
	ErrCSRFTokenUnset = errors.New("CSRF token unavailable in request context")
)

type csrfContextKey struct{}

type CSRF struct {
	secret  []byte
	onError httpx.ErrorHandler
}

func NewCSRF(secret []byte) (*CSRF, error) {
	if len(secret) < 32 {
		return nil, ErrWeakCSRFSecret
	}
	return &CSRF{secret: append([]byte(nil), secret...), onError: httpx.RespondError}, nil
}

// WithErrorHandler returns an independent configuration using a shared
// application rejection policy. A nil handler restores the default policy.
func (protection *CSRF) WithErrorHandler(handler httpx.ErrorHandler) *CSRF {
	copy := *protection
	if handler == nil {
		handler = httpx.RespondError
	}
	copy.onError = handler
	return &copy
}

func CSRFToken(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(csrfContextKey{}).(string)
	return token, ok && token != ""
}

func (protection *CSRF) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionID, hasSession := session.ID(r)
		if !hasSession && unsafeMethod(r.Method) {
			protection.onError(w, r, &httpx.Error{Status: http.StatusForbidden})
			return
		}

		token := ""
		if hasSession {
			token = protection.token(sessionID)
		}
		if unsafeMethod(r.Method) {
			provided := r.Header.Get(CSRFHeaderName)
			if provided == "" {
				r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
				if err := r.ParseForm(); err != nil {
					var tooLarge *http.MaxBytesError
					if errors.As(err, &tooLarge) {
						protection.onError(w, r, &httpx.Error{Status: http.StatusRequestEntityTooLarge, Cause: err})
						return
					}
					protection.onError(w, r, &httpx.Error{Status: http.StatusForbidden, Cause: err})
					return
				}
				provided = r.PostForm.Get(CSRFFieldName)
			}
			if !hmac.Equal([]byte(token), []byte(provided)) {
				protection.onError(w, r, &httpx.Error{
					Status: http.StatusForbidden,
					Public: "Request verification failed. Reload the page before trying again.",
				})
				return
			}
		}

		ctx := context.WithValue(r.Context(), csrfContextKey{}, token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (protection *CSRF) token(sessionID string) string {
	mac := hmac.New(sha256.New, protection.secret)
	_, _ = mac.Write([]byte("go-mvu/csrf/v1\x00"))
	_, _ = mac.Write([]byte(sessionID))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func unsafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return false
	default:
		return true
	}
}

type CSRFInput struct{}

func (CSRFInput) RenderHTML(ctx context.Context, out *html.Context) error {
	token, ok := CSRFToken(ctx)
	if !ok {
		return ErrCSRFTokenUnset
	}
	out.Static(`<input type="hidden" name="`)
	out.Static(CSRFFieldName)
	out.Static(`" value="`)
	out.Attribute(token)
	out.Static(`">`)
	return out.Err()
}

type CSRFHTMX struct {
	Body html.Component
}

func (component CSRFHTMX) RenderHTML(ctx context.Context, out *html.Context) error {
	token, ok := CSRFToken(ctx)
	if !ok {
		return ErrCSRFTokenUnset
	}
	headers, err := json.Marshal(map[string]string{CSRFHeaderName: token})
	if err != nil {
		return err
	}
	out.Static(`<div hx-headers="`)
	out.Attribute(string(headers))
	out.Static(`">`)
	out.Component(ctx, component.Body)
	out.Static(`</div>`)
	return out.Err()
}
