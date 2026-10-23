package middleware

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/dmundt/go-mvu/httpx"
)

type GzipOptions struct {
	// Eligible must explicitly opt safe routes into compression. Do not enable
	// compression for responses mixing secrets and attacker-controlled content.
	Eligible func(*http.Request) bool
	Level    int
	// OnError observes flush/close failures after response commitment.
	// nil logs through slog. Write errors are also returned to the handler.
	OnError func(*http.Request, error)
}

// Gzip streams responses without whole-response buffering. It preserves
// ResponseController flushing and skips upgrades, HEAD, bodyless statuses,
// encoded/ranged responses, no-transform, and noncompressible content types.
func Gzip(options GzipOptions) (Extension, error) {
	if options.Eligible == nil || options.Level < gzip.HuffmanOnly || options.Level > gzip.BestCompression {
		return nil, fmt.Errorf("gzip: explicit eligibility and valid compression level required")
	}
	onError := options.OnError
	if onError == nil {
		onError = func(r *http.Request, err error) {
			slog.ErrorContext(r.Context(), "gzip response failed", "error", err)
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !options.Eligible(r) || r.Method == http.MethodHead || r.Header.Get("Upgrade") != "" || headerToken(r.Header.Get("Connection"), "upgrade") {
				next.ServeHTTP(w, r)
				return
			}
			httpx.Vary(w.Header(), "Accept-Encoding")
			writer := &gzipResponse{ResponseWriter: w, level: options.Level,
				enabled: acceptsGzip(strings.Join(r.Header.Values("Accept-Encoding"), ","))}
			defer func() {
				if writer.compressor != nil {
					if err := writer.compressor.Close(); err != nil {
						onError(r, err)
					}
				}
			}()
			next.ServeHTTP(writer, r)
		})
	}, nil
}

func headerToken(value, token string) bool {
	for _, part := range strings.Split(value, ",") {
		if strings.EqualFold(strings.TrimSpace(part), token) {
			return true
		}
	}
	return false
}

func acceptsGzip(value string) bool {
	explicit, wildcard := -1.0, -1.0
	for _, coding := range strings.Split(value, ",") {
		parts := strings.Split(coding, ";")
		name, quality := strings.ToLower(strings.TrimSpace(parts[0])), 1.0
		for _, parameter := range parts[1:] {
			key, val, ok := strings.Cut(strings.TrimSpace(parameter), "=")
			if !ok || !strings.EqualFold(key, "q") {
				quality = 0
				break
			}
			q, err := strconv.ParseFloat(val, 64)
			if err != nil || q < 0 || q > 1 || q != q {
				quality = 0
				break
			}
			quality = q
		}
		switch name {
		case "gzip":
			// Conflicting duplicate directives fail closed.
			if explicit < 0 || quality < explicit {
				explicit = quality
			}
		case "*":
			if wildcard < 0 || quality < wildcard {
				wildcard = quality
			}
		}
	}
	if explicit >= 0 {
		return explicit > 0
	}
	return wildcard > 0
}

type gzipResponse struct {
	http.ResponseWriter
	level      int
	enabled    bool
	started    bool
	compressor *gzip.Writer
}

func (w *gzipResponse) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *gzipResponse) WriteHeader(status int) {
	if w.started {
		return
	}
	if status >= 100 && status < 200 && status != http.StatusSwitchingProtocols {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	w.started = true
	header := w.Header()
	httpx.Vary(header, "Accept-Encoding")
	contentType, _, _ := mime.ParseMediaType(header.Get("Content-Type"))
	compressible := strings.HasPrefix(contentType, "text/") ||
		contentType == "application/json" || contentType == "application/javascript" ||
		contentType == "application/xml" || contentType == "image/svg+xml"
	if w.enabled && compressible && contentType != "text/event-stream" && status >= 200 && status != 204 && status != 205 && status != 304 &&
		header.Get("Content-Encoding") == "" && header.Get("Content-Range") == "" &&
		header.Get("Upgrade") == "" && !headerToken(header.Get("Cache-Control"), "no-transform") {
		w.compressor, _ = gzip.NewWriterLevel(w.ResponseWriter, w.level)
		header.Set("Content-Encoding", "gzip")
		header.Del("Content-Length")
		// A strong validator for the identity representation does not validate
		// the compressed wire representation.
		header.Del("ETag")
		header.Del("Accept-Ranges")
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *gzipResponse) Write(data []byte) (int, error) {
	if !w.started {
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", http.DetectContentType(data))
		}
		w.WriteHeader(http.StatusOK)
	}
	if w.compressor != nil {
		return w.compressor.Write(data)
	}
	return w.ResponseWriter.Write(data)
}

func (w *gzipResponse) FlushError() error {
	if !w.started {
		w.WriteHeader(http.StatusOK)
	}
	if w.compressor != nil {
		if err := w.compressor.Flush(); err != nil {
			return err
		}
	}
	return http.NewResponseController(w.ResponseWriter).Flush()
}

func (w *gzipResponse) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if w.started {
		return nil, nil, fmt.Errorf("gzip: cannot hijack a started response")
	}
	return http.NewResponseController(w.ResponseWriter).Hijack()
}
