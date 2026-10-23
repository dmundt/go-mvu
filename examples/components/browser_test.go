package main

import (
	"testing"

	"github.com/dmundt/go-mvu/examples/internal/browsertest"
)

func TestBrowserServer(t *testing.T) {
	browsertest.Serve(t, newHandler())
}
