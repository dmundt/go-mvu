package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/httpx"
	"github.com/dmundt/go-mvu/mvu"
	"github.com/dmundt/go-mvu/store"
)

type principalContextKey struct{}

func TestRuntimeUsesSharedErrorHandler(t *testing.T) {
	var shared httpx.ErrorHandler = func(http.ResponseWriter, *http.Request, error) {}
	config := RuntimeConfig[string, testModel, testMessage]{OnError: shared}
	if config.OnError == nil {
		t.Fatal("shared ErrorHandler assignment failed")
	}
}

func TestDefaultErrorResponderClassifiesClientAndBodyErrors(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want int
	}{
		{name: "typed client status", err: &httpx.Error{Status: http.StatusUnprocessableEntity}, want: http.StatusUnprocessableEntity},
		{name: "wrapped body limit", err: fmt.Errorf("decode: %w", &http.MaxBytesError{Limit: 8}), want: http.StatusRequestEntityTooLarge},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime := newTestRuntime(t, nil)
			route := addRoute(Result{Mode: RenderNothing})
			route.Decode = func(*http.Request) (testMessage, error) {
				return 0, test.err
			}
			response := serve(runtime.Handle(route), httptest.NewRequest(http.MethodPost, "/add", nil))
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d: %s", response.Code, test.want, response.Body)
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestRenderAutoAddsNoStoreAndMergesVary(t *testing.T) {
	runtime := newTestRuntime(t, nil)
	handler := runtime.Handle(addRoute(Result{Mode: RenderAuto, Fragment: "count"}))
	request := httptest.NewRequest(http.MethodPost, "/add", nil)
	request.Header.Set("HX-Request", "true")
	response := httptest.NewRecorder()
	response.Header().Add("Vary", "Accept-Encoding")
	handler.ServeHTTP(response, request)
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
	}
	for _, field := range []string{"Accept-Encoding", "HX-Request", "HX-Boosted", "HX-History-Restore-Request"} {
		found := false
		for _, line := range response.Header().Values("Vary") {
			found = found || line == field
		}
		if !found {
			t.Errorf("Vary omitted %q: %v", field, response.Header().Values("Vary"))
		}
	}
}

func TestExpectedVersionRejectsStaleIntentWithoutRetry(t *testing.T) {
	memory := store.NewMemory[string, testModel]()
	_, err := memory.Create(context.Background(), "client", testModel{Count: 9})
	if err != nil {
		t.Fatal(err)
	}
	runtime := newTestRuntime(t, func(c *RuntimeConfig[string, testModel, testMessage]) {
		c.Store = memory
	})
	route := addRoute(Result{Mode: RenderNothing})
	route.ExpectedVersion = func(*http.Request) (store.Version, error) {
		return 0, nil
	}
	response := serve(runtime.Handle(route), httptest.NewRequest(http.MethodPost, "/add", nil))
	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", response.Code)
	}
	snapshot, err := memory.Load(context.Background(), "client")
	if err != nil || snapshot.Model.Count != 9 || snapshot.Version != 1 {
		t.Fatalf("snapshot = %+v, err = %v", snapshot, err)
	}
}

func TestExpectedVersionAppliesOnlyToDirectTransition(t *testing.T) {
	probe := &lockProbeStore{Memory: store.NewMemory[string, testModel]()}
	runtime := newTestRuntime(t, func(c *RuntimeConfig[string, testModel, testMessage]) {
		c.Store = probe
		c.App = commandApp{probe: probe}
	})
	route := addRoute(Result{Mode: RenderFragment, Fragment: "count"})
	route.ExpectedVersion = func(*http.Request) (store.Version, error) {
		return 1, nil
	}
	response := serve(runtime.Handle(route), httptest.NewRequest(http.MethodPost, "/add", nil))
	if response.Code != http.StatusOK || response.Body.String() != "<output>6</output>" {
		t.Fatalf("got %d %q", response.Code, response.Body)
	}
}

func TestTransitionAuthorizationRunsForCASRetriesAndCommands(t *testing.T) {
	backing := &conflictStore{Memory: store.NewMemory[string, testModel](), conflicts: 1}
	var mu sync.Mutex
	var models []int
	var messages []testMessage
	runtime := newTestRuntime(t, func(c *RuntimeConfig[string, testModel, testMessage]) {
		c.Store = backing
		c.App = commandApp{probe: &lockProbeStore{Memory: backing.Memory}}
		c.AuthorizeTransition = func(_ context.Context, key string, model testModel, message testMessage) error {
			if key != "client" {
				t.Errorf("key = %q", key)
			}
			mu.Lock()
			models = append(models, model.Count)
			messages = append(messages, message)
			mu.Unlock()
			return nil
		}
	})
	response := serve(runtime.Handle(addRoute(Result{Mode: RenderNothing})),
		httptest.NewRequest(http.MethodPost, "/add", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d", response.Code)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(models) != 4 || models[0] != 0 || models[1] != 0 || models[2] != 100 || models[3] != 101 {
		t.Fatalf("authorized models = %v; messages = %v", models, messages)
	}
	if messages[0] != 1 || messages[1] != 1 || messages[2] != 1 || messages[3] != 5 {
		t.Fatalf("authorized messages = %v", messages)
	}
}

func TestTransitionAuthorizationRejectsWithoutMutation(t *testing.T) {
	runtime := newTestRuntime(t, func(c *RuntimeConfig[string, testModel, testMessage]) {
		c.AuthorizeTransition = func(context.Context, string, testModel, testMessage) error {
			return &httpx.Error{Status: http.StatusForbidden, Public: "Forbidden"}
		}
	})
	response := serve(runtime.Handle(addRoute(Result{Mode: RenderNothing})),
		httptest.NewRequest(http.MethodPost, "/add", nil))
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d", response.Code)
	}
	if _, err := runtime.store.Load(context.Background(), "client"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unauthorized request created model: %v", err)
	}
}

func TestCanceledRequestDoesNotCreateModel(t *testing.T) {
	memory := store.NewMemory[string, testModel]()
	runtime := newTestRuntime(t, func(c *RuntimeConfig[string, testModel, testMessage]) {
		c.Store = memory
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	response := serve(runtime.Handle(addRoute(Result{Mode: RenderNothing})),
		httptest.NewRequest(http.MethodPost, "/add", nil).WithContext(ctx))
	if response.Code != http.StatusRequestTimeout {
		t.Fatalf("status = %d", response.Code)
	}
	if _, err := memory.Load(context.Background(), "client"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("canceled request created model: %v", err)
	}
}

type failingEffectApp struct{ testApp }

func (failingEffectApp) Update(
	_ context.Context, model testModel, message testMessage,
) (testModel, []mvu.Command[testMessage], error) {
	model.Count += int(message)
	return model, []mvu.Command[testMessage]{
		mvu.CommandFunc[testMessage](func(context.Context, mvu.Dispatch[testMessage]) error {
			return errors.New("effect failed")
		}),
	}, nil
}

func TestOutcomeObserverClassifiesCommittedEffectFailure(t *testing.T) {
	var count atomic.Int32
	var outcome RequestOutcome
	runtime := newTestRuntime(t, func(c *RuntimeConfig[string, testModel, testMessage]) {
		c.App = failingEffectApp{}
		c.ObserveOutcome = func(_ *http.Request, got RequestOutcome) {
			count.Add(1)
			outcome = got
		}
	})
	response := serve(runtime.Handle(addRoute(Result{Mode: RenderNothing})),
		httptest.NewRequest(http.MethodPost, "/add", nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", response.Code)
	}
	var effect *EffectError
	if count.Load() != 1 || outcome.State != OutcomeEffectFailed || !outcome.Committed || !errors.As(outcome.Err, &effect) {
		t.Fatalf("count=%d outcome=%+v", count.Load(), outcome)
	}
	snapshot, err := runtime.store.Load(context.Background(), "client")
	if err != nil || snapshot.Model.Count != 1 {
		t.Fatalf("committed snapshot = %+v, err = %v", snapshot, err)
	}
}

type panicView struct{ testView }

func (panicView) Page(context.Context, testModel, RequestContext) (html.Page, error) {
	panic("view panic")
}

func TestOutcomeObserverRunsOnceOnPanic(t *testing.T) {
	var count atomic.Int32
	var outcome RequestOutcome
	runtime := newTestRuntime(t, func(c *RuntimeConfig[string, testModel, testMessage]) {
		c.View = panicView{}
		c.ObserveOutcome = func(_ *http.Request, got RequestOutcome) {
			count.Add(1)
			outcome = got
		}
	})
	handler := runtime.Handle(addRoute(Result{Mode: RenderPage}))
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("expected view panic")
			}
		}()
		serve(handler, httptest.NewRequest(http.MethodPost, "/add", nil))
	}()
	if count.Load() != 1 || outcome.State != OutcomeCommitted || !outcome.Committed {
		t.Fatalf("count=%d outcome=%+v", count.Load(), outcome)
	}
}

func TestOutcomeObserverMarksStreamingRenderFailureAborted(t *testing.T) {
	var count atomic.Int32
	var outcome RequestOutcome
	runtime := newTestRuntime(t, func(c *RuntimeConfig[string, testModel, testMessage]) {
		c.View = testView{failFragment: true}
		c.ObserveOutcome = func(_ *http.Request, got RequestOutcome) {
			count.Add(1)
			outcome = got
		}
	})
	serve(runtime.Handle(addRoute(Result{Mode: RenderFragment, Fragment: "count"})),
		httptest.NewRequest(http.MethodPost, "/add", nil))
	var aborted *responseAbortError
	if count.Load() != 1 || outcome.State != OutcomeResponseAborted || !outcome.Committed || !errors.As(outcome.Err, &aborted) {
		t.Fatalf("count=%d outcome=%+v", count.Load(), outcome)
	}
}

func TestBoundedAsyncRunnerRejectsOverloadAndDrains(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var ran atomic.Int32
	runner, err := NewAsyncCommandRunner[testMessage](1, 1, time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	blocked := mvu.CommandFunc[testMessage](func(context.Context, mvu.Dispatch[testMessage]) error {
		ran.Add(1)
		close(started)
		<-release
		return nil
	})
	queued := mvu.CommandFunc[testMessage](func(context.Context, mvu.Dispatch[testMessage]) error {
		ran.Add(1)
		return nil
	})
	if err := runner.Run(context.Background(), []mvu.Command[testMessage]{blocked}, nil); err != nil {
		t.Fatal(err)
	}
	<-started
	if err := runner.Run(context.Background(), []mvu.Command[testMessage]{queued}, nil); err != nil {
		t.Fatal(err)
	}
	if err := runner.Run(context.Background(), []mvu.Command[testMessage]{queued}, nil); !errors.Is(err, ErrCommandOverload) {
		t.Fatalf("overload error = %v", err)
	}
	close(release)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := runner.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if ran.Load() != 2 {
		t.Fatalf("ran = %d, want 2", ran.Load())
	}
	if err := runner.Run(context.Background(), nil, nil); !errors.Is(err, ErrCommandClosed) {
		t.Fatalf("closed error = %v", err)
	}
}

func TestBoundedAsyncRunnerIgnoresRequestCancellation(t *testing.T) {
	runner, err := NewAsyncCommandRunner[testMessage](1, 1, 30*time.Millisecond, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), principalContextKey{}, "alice"))
	cancel()
	type completion struct {
		err       error
		principal string
	}
	finished := make(chan completion, 1)
	command := mvu.CommandFunc[testMessage](func(ctx context.Context, _ mvu.Dispatch[testMessage]) error {
		<-ctx.Done()
		principal, _ := ctx.Value(principalContextKey{}).(string)
		finished <- completion{err: ctx.Err(), principal: principal}
		return nil
	})
	if err := runner.Run(ctx, []mvu.Command[testMessage]{command}, nil); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-finished:
		if !errors.Is(got.err, context.DeadlineExceeded) || got.principal != "alice" {
			t.Fatalf("command context = %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("command did not observe configured timeout")
	}
	closeCtx, closeCancel := context.WithTimeout(context.Background(), time.Second)
	defer closeCancel()
	if err := runner.Close(closeCtx); err != nil {
		t.Fatal(err)
	}
}

func TestAsyncFollowupAuthorizationRetainsRequestPrincipal(t *testing.T) {
	probe := &lockProbeStore{Memory: store.NewMemory[string, testModel]()}
	runner, err := NewAsyncCommandRunner[testMessage](1, 4, time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer runner.Close(context.Background())
	principals := make(chan string, 2)
	runtime := newTestRuntime(t, func(c *RuntimeConfig[string, testModel, testMessage]) {
		c.Store = probe
		c.App = commandApp{probe: probe}
		c.Commands = runner
		c.AuthorizeTransition = func(ctx context.Context, _ string, _ testModel, _ testMessage) error {
			principal, _ := ctx.Value(principalContextKey{}).(string)
			principals <- principal
			return nil
		}
	})
	request := httptest.NewRequest(http.MethodPost, "/add", nil)
	request = request.WithContext(context.WithValue(request.Context(), principalContextKey{}, "alice"))
	response := serve(runtime.Handle(addRoute(Result{Mode: RenderNothing})), request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d", response.Code)
	}
	for range 2 {
		select {
		case got := <-principals:
			if got != "alice" {
				t.Fatalf("authorized principal = %q", got)
			}
		case <-time.After(time.Second):
			t.Fatal("async follow-up authorization did not run")
		}
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		snapshot, loadErr := probe.Load(context.Background(), "client")
		if loadErr == nil && snapshot.Model.Count == 6 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("async follow-up did not commit")
}

func TestAsyncRunnerRecursiveDispatchFailsFastWithoutDeadlock(t *testing.T) {
	runner, err := NewAsyncCommandRunner[testMessage](1, 1, time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	result := make(chan error, 1)
	command := mvu.CommandFunc[testMessage](func(context.Context, mvu.Dispatch[testMessage]) error {
		close(started)
		<-release
		result <- runner.Run(context.Background(), []mvu.Command[testMessage]{
			mvu.CommandFunc[testMessage](func(context.Context, mvu.Dispatch[testMessage]) error {
				return nil
			}),
		}, nil)
		return nil
	})
	if err := runner.Run(context.Background(), []mvu.Command[testMessage]{command}, nil); err != nil {
		t.Fatal(err)
	}
	<-started
	if err := runner.Run(context.Background(), []mvu.Command[testMessage]{
		mvu.CommandFunc[testMessage](func(context.Context, mvu.Dispatch[testMessage]) error { return nil }),
	}, nil); err != nil {
		t.Fatal(err)
	}
	close(release)
	select {
	case err := <-result:
		if !errors.Is(err, ErrCommandOverload) {
			t.Fatalf("recursive enqueue error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("recursive enqueue deadlocked")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := runner.Close(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestAsyncRunnerCloseReturnsContextErrorWhileDraining(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	runner, err := NewAsyncCommandRunner[testMessage](1, 1, time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	command := mvu.CommandFunc[testMessage](func(context.Context, mvu.Dispatch[testMessage]) error {
		close(started)
		<-release
		return nil
	})
	if err := runner.Run(context.Background(), []mvu.Command[testMessage]{command}, nil); err != nil {
		t.Fatal(err)
	}
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := runner.Close(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Close error = %v", err)
	}
	close(release)
	if err := runner.Close(context.Background()); err != nil {
		t.Fatalf("draining Close error = %v", err)
	}
}

func TestAsyncCommandRunnerRejectsInvalidConfiguration(t *testing.T) {
	for _, config := range []struct {
		concurrency int
		queue       int
		timeout     time.Duration
	}{
		{concurrency: 0, queue: 1, timeout: time.Second},
		{concurrency: -1, queue: 1, timeout: time.Second},
		{concurrency: 1, queue: -1, timeout: time.Second},
		{concurrency: 1, queue: 1, timeout: 0},
		{concurrency: 1, queue: 1, timeout: -time.Second},
	} {
		if runner, err := NewAsyncCommandRunner[testMessage](
			config.concurrency, config.queue, config.timeout, nil,
		); runner != nil || !errors.Is(err, ErrInvalidCommandRunnerConfig) {
			t.Fatalf("config %+v returned runner=%v err=%v", config, runner, err)
		}
	}
}

func TestPageOutcomeIncludesReadOnlyCompletionAndInitializationCommit(t *testing.T) {
	var outcomes []RequestOutcome
	runtime := newTestRuntime(t, func(c *RuntimeConfig[string, testModel, testMessage]) {
		c.ObserveOutcome = func(_ *http.Request, outcome RequestOutcome) {
			outcomes = append(outcomes, outcome)
		}
	})
	handler := runtime.Page(Result{Mode: RenderPage})
	serve(handler, httptest.NewRequest(http.MethodGet, "/", nil))
	serve(handler, httptest.NewRequest(http.MethodGet, "/", nil))
	if len(outcomes) != 2 {
		t.Fatalf("observed %d outcomes", len(outcomes))
	}
	if outcomes[0].State != OutcomeCommitted || !outcomes[0].Committed {
		t.Fatalf("init outcome = %+v", outcomes[0])
	}
	if outcomes[1].State != OutcomeCompleted || outcomes[1].Committed {
		t.Fatalf("read-only outcome = %+v", outcomes[1])
	}
}

func TestHandleInitCommitSurvivesRejectedTransition(t *testing.T) {
	var authorized atomic.Int32
	var outcome RequestOutcome
	runtime := newTestRuntime(t, func(c *RuntimeConfig[string, testModel, testMessage]) {
		c.AuthorizeTransition = func(context.Context, string, testModel, testMessage) error {
			if authorized.Add(1) == 2 {
				return &httpx.Error{Status: http.StatusForbidden, Public: "Forbidden"}
			}
			return nil
		}
		c.ObserveOutcome = func(_ *http.Request, got RequestOutcome) { outcome = got }
	})
	response := serve(runtime.Handle(addRoute(Result{Mode: RenderNothing})),
		httptest.NewRequest(http.MethodPost, "/add", nil))
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d", response.Code)
	}
	if outcome.State != OutcomeCommitted || !outcome.Committed {
		t.Fatalf("outcome = %+v", outcome)
	}
}

func TestEffectCancellationRespondsInternalErrorButRetainsCommit(t *testing.T) {
	var outcome RequestOutcome
	runtime := newTestRuntime(t, func(c *RuntimeConfig[string, testModel, testMessage]) {
		c.App = failingEffectApp{}
		c.ObserveOutcome = func(_ *http.Request, got RequestOutcome) { outcome = got }
	})
	route := addRoute(Result{Mode: RenderNothing})
	route.Decode = StaticMessage[testMessage](1)
	// Use an effect failure that wraps context cancellation; it must not be
	// reclassified as a pre-commit request timeout.
	cancelApp := cancellationEffectApp{}
	runtime.app = cancelApp
	response := serve(runtime.Handle(route), httptest.NewRequest(http.MethodPost, "/add", nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
	if outcome.State != OutcomeEffectFailed || !outcome.Committed {
		t.Fatalf("outcome = %+v", outcome)
	}
}

type cancellationEffectApp struct{ testApp }

func (cancellationEffectApp) Update(
	_ context.Context, model testModel, message testMessage,
) (testModel, []mvu.Command[testMessage], error) {
	model.Count += int(message)
	return model, []mvu.Command[testMessage]{
		mvu.CommandFunc[testMessage](func(context.Context, mvu.Dispatch[testMessage]) error {
			return context.Canceled
		}),
	}, nil
}

func TestRequestContextReceivesExplicitLocale(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request = request.WithContext(httpx.WithLocale(request.Context(), "fr-CA"))
	if got := NewRequestContext(request, nil).Locale; got != "fr-CA" {
		t.Fatalf("Locale = %q", got)
	}
}

func TestResultCacheControlOverridesDefaultNoStore(t *testing.T) {
	runtime := newTestRuntime(t, nil)
	response := serve(runtime.Page(Result{
		Mode: RenderPage, CacheControl: "public, max-age=60",
	}), httptest.NewRequest(http.MethodGet, "/", nil))
	if got := response.Header().Get("Cache-Control"); got != "public, max-age=60" {
		t.Fatalf("Cache-Control = %q", got)
	}
}
