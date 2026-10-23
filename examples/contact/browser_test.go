package main

import (
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/examples/internal/browsertest"
)

func TestBrowserServer(t *testing.T) {
	handler, err := newHandler([]byte(strings.Repeat("browser-test", 4)))
	if err != nil {
		t.Fatal(err)
	}
	browsertest.Serve(t, handler)
}
