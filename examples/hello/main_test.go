package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/examples/internal/browsertest"
)

func TestHelloPage(t *testing.T) {
	handler := newHandler()
	for _, test := range []struct {
		method string
		path   string
		status int
	}{
		{http.MethodGet, "/", http.StatusOK},
		{http.MethodGet, "/missing", http.StatusNotFound},
		{http.MethodPost, "/", http.StatusMethodNotAllowed},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))
		if response.Code != test.status {
			t.Fatalf("%s %s: status = %d", test.method, test.path, response.Code)
		}
		if test.status == http.StatusOK {
			if response.Header().Get("Content-Type") != "text/html; charset=utf-8" ||
				!strings.Contains(response.Body.String(), "<h1>Hello World!</h1>") ||
				strings.Contains(response.Body.String(), "<script") {
				t.Fatalf("hello response = %s", response.Body.String())
			}
		}
	}
}

func TestBrowserServer(t *testing.T) {
	browsertest.Serve(t, newHandler())
}
