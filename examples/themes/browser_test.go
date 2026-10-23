package main

import (
	"testing"

	"github.com/dmundt/go-mvu/internal/browsertest"
)

func TestBrowserServer(t *testing.T) {
	browsertest.Serve(t, newHandler())
}
