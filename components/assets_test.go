package components

import (
	"crypto/sha512"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerServesHTMXMatchingIntegrity(t *testing.T) {
	w := httptest.NewRecorder()
	AssetHandler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, HTMXPath, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Fatalf("content type = %q", ct)
	}
	body, _ := io.ReadAll(w.Body)
	sum := sha512.Sum384(body)
	if got := "sha384-" + base64.StdEncoding.EncodeToString(sum[:]); got != HTMXIntegrity {
		t.Fatalf("integrity = %s, want %s", got, HTMXIntegrity)
	}
	if !strings.Contains(w.Header().Get("Cache-Control"), "immutable") {
		t.Fatal("missing immutable caching")
	}
}

func TestHandlerHidesDirectoryListing(t *testing.T) {
	w := httptest.NewRecorder()
	AssetHandler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, AssetPrefix, nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}
