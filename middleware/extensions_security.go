package middleware

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dmundt/go-mvu/httpx"
)

type CORSOptions struct {
	Origins      []string
	Methods      []string
	Headers      []string
	Expose       []string
	Credentials  bool
	MaxAge       time.Duration
	ErrorHandler httpx.ErrorHandler
}

func httpToken(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if !(char >= '0' && char <= '9' || char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' ||
			strings.ContainsRune("!#$%&'*+-.^_`|~", char)) {
			return false
		}
	}
	return true
}

func headerToken(value, token string) bool {
	for _, part := range strings.Split(value, ",") {
		if strings.EqualFold(strings.TrimSpace(part), token) {
			return true
		}
	}
	return false
}

func validOrigin(origin string) bool {
	parsed, err := url.Parse(origin)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") &&
		parsed.Host != "" && parsed.User == nil && parsed.Path == "" && parsed.RawQuery == "" &&
		parsed.Fragment == "" && parsed.RawFragment == "" && !parsed.ForceQuery && parsed.String() == origin &&
		!strings.ContainsAny(origin, "\r\n, *")
}

// CORS permits only exact explicit origins, methods, and header names. Wildcards
// and the opaque "null" origin are intentionally unsupported.
func CORS(options CORSOptions) (Extension, error) {
	if len(options.Origins) == 0 || len(options.Methods) == 0 || options.MaxAge < 0 {
		return nil, fmt.Errorf("cors: explicit origins, methods and nonnegative max age required")
	}
	origins, methods, headers := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, origin := range options.Origins {
		if !validOrigin(origin) {
			return nil, fmt.Errorf("cors: invalid origin %q", origin)
		}
		origins[origin] = true
	}
	for _, method := range options.Methods {
		if !httpToken(method) || method == "*" {
			return nil, fmt.Errorf("cors: invalid method")
		}
		methods[method] = true
	}
	for _, header := range append(append([]string(nil), options.Headers...), options.Expose...) {
		if !httpToken(header) || header == "*" {
			return nil, fmt.Errorf("cors: invalid header")
		}
	}
	for _, header := range options.Headers {
		headers[strings.ToLower(header)] = true
	}
	expose := strings.Join(options.Expose, ", ")
	fail := extensionError(options.ErrorHandler)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			httpx.Vary(w.Header(), "Origin")
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}
			preflight := r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""
			if preflight {
				httpx.Vary(w.Header(), "Access-Control-Request-Method", "Access-Control-Request-Headers")
			}
			method := r.Method
			if preflight {
				method = r.Header.Get("Access-Control-Request-Method")
			}
			if len(r.Header.Values("Origin")) != 1 || !origins[origin] || !methods[method] {
				rejectExtension(fail, w, r, http.StatusForbidden)
				return
			}
			var requested []string
			if preflight {
				if len(r.Header.Values("Access-Control-Request-Method")) != 1 {
					rejectExtension(fail, w, r, http.StatusForbidden)
					return
				}
				raw := strings.Join(r.Header.Values("Access-Control-Request-Headers"), ",")
				if raw != "" {
					for _, header := range strings.Split(raw, ",") {
						header = strings.TrimSpace(header)
						if !httpToken(header) || !headers[strings.ToLower(header)] {
							rejectExtension(fail, w, r, http.StatusForbidden)
							return
						}
						requested = append(requested, http.CanonicalHeaderKey(header))
					}
				}
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			if options.Credentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			if preflight {
				w.Header().Set("Access-Control-Allow-Methods", method)
				if len(requested) != 0 {
					w.Header().Set("Access-Control-Allow-Headers", strings.Join(requested, ", "))
				}
				if options.MaxAge > 0 {
					w.Header().Set("Access-Control-Max-Age", strconv.FormatInt(int64(options.MaxAge/time.Second), 10))
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
			if expose != "" {
				w.Header().Set("Access-Control-Expose-Headers", expose)
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}

type HSTSOptions struct {
	MaxAge            time.Duration
	IncludeSubDomains bool
	Preload           bool
}

// HSTS is emitted only for requests served over a direct TLS connection.
func HSTS(options HSTSOptions) (Extension, error) {
	if options.MaxAge < time.Second || options.Preload && (!options.IncludeSubDomains || options.MaxAge < 365*24*time.Hour) {
		return nil, fmt.Errorf("hsts: positive max age required; preload requires subdomains and at least one year")
	}
	value := "max-age=" + strconv.FormatInt(int64(options.MaxAge/time.Second), 10)
	if options.IncludeSubDomains {
		value += "; includeSubDomains"
	}
	if options.Preload {
		value += "; preload"
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.TLS != nil {
				w.Header().Set("Strict-Transport-Security", value)
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}
