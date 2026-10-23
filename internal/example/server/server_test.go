package server_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/dmundt/go-mvu/internal/example/server"
	"github.com/dmundt/go-mvu/mvu"
	"github.com/dmundt/go-mvu/web"
)

type message struct{}

func (message) MessageName() string { return "test" }

type recorder struct {
	mu     sync.Mutex
	events []string
}

func (r *recorder) add(event string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *recorder) list() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.events)
}

func (r *recorder) before(t *testing.T, first, second string) {
	t.Helper()
	events := r.list()
	i, j := slices.Index(events, first), slices.Index(events, second)
	if i < 0 || j < 0 || i >= j {
		t.Fatalf("want %q before %q, events %v", first, second, events)
	}
}

type closerFunc func() error

func (fn closerFunc) Close() error { return fn() }

func listen(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return listener
}

func newRunner(t *testing.T) *web.AsyncCommandRunner[message] {
	t.Helper()
	runner, err := web.NewAsyncCommandRunner[message](1, 1, time.Minute, func(err error) {
		t.Errorf("command error: %v", err)
	})
	if err != nil {
		t.Fatal(err)
	}
	return runner
}

func get(url string) (int, error) {
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
	response, err := client.Get(url)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	return response.StatusCode, nil
}

func wait(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout waiting for %s", what)
	}
}

// fixture wires an HTTP handler that blocks until released, then submits a
// command to a real bounded runner. The command blocks until released.
type fixture struct {
	events         *recorder
	runner         *web.AsyncCommandRunner[message]
	entered        chan struct{}
	releaseRequest chan struct{}
	effectStarted  chan struct{}
	releaseEffect  chan struct{}
	effectCtx      chan context.Context
	shutdown       chan struct{}
	drainStarted   chan struct{}
	closed         chan struct{}
	server         *http.Server
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{
		events:         &recorder{},
		runner:         newRunner(t),
		entered:        make(chan struct{}),
		releaseRequest: make(chan struct{}),
		effectStarted:  make(chan struct{}),
		releaseEffect:  make(chan struct{}),
		effectCtx:      make(chan context.Context, 1),
		shutdown:       make(chan struct{}),
		drainStarted:   make(chan struct{}),
		closed:         make(chan struct{}),
	}
	command := mvu.CommandFunc[message](func(ctx context.Context, _ mvu.Dispatch[message]) error {
		f.effectCtx <- ctx
		f.events.add("effect started")
		close(f.effectStarted)
		<-f.releaseEffect
		f.events.add("effect done")
		return nil
	})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(f.entered)
		<-f.releaseRequest
		err := f.runner.Run(r.Context(), []mvu.Command[message]{command}, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		f.events.add("request done")
		w.WriteHeader(http.StatusAccepted)
	})
	f.server = server.New("", handler)
	f.server.RegisterOnShutdown(func() {
		f.events.add("admission stopped")
		close(f.shutdown)
	})
	return f
}

func (f *fixture) config(timeout time.Duration, listener net.Listener) server.Config {
	return server.Config{
		Server:          f.server,
		Listener:        listener,
		ShutdownTimeout: timeout,
		Drain: []func(context.Context) error{func(ctx context.Context) error {
			f.events.add("drain started")
			close(f.drainStarted)
			return f.runner.Close(ctx)
		}},
		Dependencies: []io.Closer{closerFunc(func() error {
			f.events.add("dependency closed")
			close(f.closed)
			return nil
		})},
	}
}

func TestRunGracefulOrdering(t *testing.T) {
	f := newFixture(t)
	listener := listen(t)
	url := "http://" + listener.Addr().String() + "/"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- server.Run(ctx, f.config(5*time.Second, listener)) }()

	status := make(chan int, 1)
	go func() {
		code, err := get(url)
		if err != nil {
			t.Errorf("in-flight request: %v", err)
		}
		status <- code
	}()
	wait(t, f.entered, "request entered")
	cancel()
	wait(t, f.shutdown, "admission stopped")
	if conn, err := net.Dial("tcp", listener.Addr().String()); err == nil {
		conn.Close()
		t.Fatal("listener still accepts connections after shutdown started")
	}
	close(f.releaseRequest)
	if code := <-status; code != http.StatusAccepted {
		t.Fatalf("in-flight request status %d", code)
	}
	wait(t, f.effectStarted, "effect started")
	wait(t, f.drainStarted, "drain started")
	select {
	case <-f.closed:
		t.Fatal("dependency closed while command running")
	case err := <-result:
		t.Fatalf("Run returned before command drained: %v", err)
	default:
	}
	close(f.releaseEffect)
	if err := <-result; err != nil {
		t.Fatalf("Run: %v", err)
	}
	f.events.before(t, "admission stopped", "request done")
	f.events.before(t, "request done", "drain started")
	f.events.before(t, "drain started", "effect done")
	f.events.before(t, "effect done", "dependency closed")
}

func TestRunTimeoutKeepsDependenciesOpen(t *testing.T) {
	f := newFixture(t)
	listener := listen(t)
	url := "http://" + listener.Addr().String() + "/"
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- server.Run(ctx, f.config(50*time.Millisecond, listener)) }()
	go func() { _, _ = get(url) }()
	wait(t, f.entered, "request entered")
	close(f.releaseRequest)
	wait(t, f.effectStarted, "effect started")
	cancel()

	err := <-result
	if !errors.Is(err, server.ErrDependenciesOpen) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run error = %v, want ErrDependenciesOpen and DeadlineExceeded", err)
	}
	select {
	case <-f.closed:
		t.Fatal("dependency closed while command still running")
	default:
	}
	effectCtx := <-f.effectCtx
	if effectCtx.Err() != nil {
		t.Fatalf("shutdown timeout cancelled accepted command: %v", effectCtx.Err())
	}
	if err := f.runner.Run(context.Background(), []mvu.Command[message]{
		mvu.CommandFunc[message](func(context.Context, mvu.Dispatch[message]) error { return nil }),
	}, nil); !errors.Is(err, web.ErrCommandClosed) {
		t.Fatalf("runner accepts work after shutdown: %v", err)
	}
	close(f.releaseEffect)
	if err := f.runner.Close(context.Background()); err != nil {
		t.Fatalf("final drain: %v", err)
	}
	if got := f.events.list(); !slices.Contains(got, "effect done") {
		t.Fatalf("accepted command did not complete: %v", got)
	}
}

func TestRunHTTPTimeoutForceClosesAndKeepsDependenciesOpen(t *testing.T) {
	f := newFixture(t)
	listener := listen(t)
	url := "http://" + listener.Addr().String() + "/"
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- server.Run(ctx, f.config(50*time.Millisecond, listener)) }()
	clientErr := make(chan error, 1)
	go func() {
		_, err := get(url)
		clientErr <- err
	}()
	wait(t, f.entered, "request entered")
	cancel()

	err := <-result
	if !errors.Is(err, server.ErrDependenciesOpen) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run error = %v, want ErrDependenciesOpen and DeadlineExceeded", err)
	}
	if err := <-clientErr; err == nil {
		t.Fatal("stuck request connection not force-closed")
	}
	select {
	case <-f.closed:
		t.Fatal("dependency closed while handler still running")
	default:
	}
	close(f.releaseRequest)
	if err := f.runner.Close(context.Background()); err != nil {
		t.Fatalf("final drain: %v", err)
	}
}

func TestRunServeErrorCleansUp(t *testing.T) {
	f := newFixture(t)
	listener := listen(t)
	listener.Close()
	err := server.Run(context.Background(), f.config(time.Second, listener))
	if err == nil || errors.Is(err, server.ErrDependenciesOpen) {
		t.Fatalf("Run error = %v, want serve error with cleanup", err)
	}
	wait(t, f.drainStarted, "drain")
	wait(t, f.closed, "dependency closed")
}

func TestRunListenErrorCleansUp(t *testing.T) {
	f := newFixture(t)
	occupied := listen(t)
	defer occupied.Close()
	f.server.Addr = occupied.Addr().String()
	err := server.Run(context.Background(), f.config(time.Second, nil))
	if err == nil {
		t.Fatal("Run succeeded on occupied address")
	}
	wait(t, f.closed, "dependency closed")
}

func TestRunInvalidConfig(t *testing.T) {
	for _, config := range []server.Config{
		{ShutdownTimeout: time.Second},
		{Server: server.New("", http.NotFoundHandler())},
	} {
		err := server.Run(context.Background(), config)
		if !errors.Is(err, server.ErrInvalidConfig) {
			t.Fatalf("Run(%+v) = %v, want ErrInvalidConfig", config, err)
		}
	}
}

func TestNewSetsTimeouts(t *testing.T) {
	s := server.New("127.0.0.1:0", http.NotFoundHandler())
	if s.ReadHeaderTimeout != server.ReadHeaderTimeout || s.IdleTimeout != server.IdleTimeout {
		t.Fatalf("timeouts = %v/%v", s.ReadHeaderTimeout, s.IdleTimeout)
	}
}

func TestRepeatedRunsDoNotLeakGoroutines(t *testing.T) {
	baseline := runtime.NumGoroutine()
	for range 20 {
		listener := listen(t)
		runner := newRunner(t)
		ctx, cancel := context.WithCancel(context.Background())
		served := make(chan struct{})
		s := server.New("", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		result := make(chan error, 1)
		go func() {
			result <- server.Run(ctx, server.Config{
				Server:          s,
				Listener:        listener,
				ShutdownTimeout: time.Second,
				Drain:           []func(context.Context) error{runner.Close},
			})
		}()
		go func() {
			if _, err := get("http://" + listener.Addr().String() + "/"); err != nil {
				t.Errorf("request: %v", err)
			}
			close(served)
		}()
		<-served
		cancel()
		if err := <-result; err != nil {
			t.Fatalf("Run: %v", err)
		}
	}
	deadline := time.Now().Add(5 * time.Second)
	for runtime.NumGoroutine() > baseline+2 {
		if time.Now().After(deadline) {
			t.Fatalf("goroutines %d, baseline %d", runtime.NumGoroutine(), baseline)
		}
		runtime.Gosched()
		time.Sleep(10 * time.Millisecond)
	}
}
