package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCounterBuildsOutsideRepository(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	project := t.TempDir()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(project, file), content, 0600); err != nil {
			t.Fatal(err)
		}
	}
	module := "module example.com/mycounter\n\ngo 1.24.0\n\n" +
		"require github.com/dmundt/go-mvu v0.0.0\n\n" +
		"replace github.com/dmundt/go-mvu => " + strconv.Quote(filepath.ToSlash(root)) + "\n"
	if err := os.WriteFile(filepath.Join(project, "go.mod"), []byte(module), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	for _, args := range [][]string{
		{"mod", "tidy"},
		{"build", "-o", filepath.Join(project, "counter.exe"), "."},
	} {
		command := exec.CommandContext(ctx, "go", args...)
		command.Dir = project
		command.Env = append(os.Environ(), "GOWORK=off")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("standalone go %v: %v\n%s", args, err, output)
		}
	}
}

func TestServeShutsDownOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := serve(ctx, &http.Server{Addr: "127.0.0.1:0"}); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestServeReportsListenFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := serve(context.Background(), &http.Server{Addr: listener.Addr().String()}); err == nil {
		t.Fatal("expected occupied-address failure")
	}
}

func TestServeAcceptsRequestsAndStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	address := make(chan string, 1)
	server := &http.Server{
		Addr: "127.0.0.1:0",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
		BaseContext: func(listener net.Listener) context.Context {
			address <- listener.Addr().String()
			return context.Background()
		},
	}
	defer server.Close()
	done := make(chan error, 1)
	go func() { done <- serve(ctx, server) }()
	select {
	case target := <-address:
		client := &http.Client{Timeout: 5 * time.Second}
		response, err := client.Get("http://" + target)
		if err != nil {
			t.Fatal(err)
		}
		_, readErr := io.Copy(io.Discard, response.Body)
		closeErr := response.Body.Close()
		if readErr != nil || closeErr != nil || response.StatusCode != http.StatusNoContent {
			t.Fatalf("status=%d read=%v close=%v", response.StatusCode, readErr, closeErr)
		}
	case err := <-done:
		t.Fatalf("server stopped before listening: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not listen")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop")
	}
}
