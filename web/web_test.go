package web

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/mvu"
	"github.com/dmundt/go-mvu/store"
	"github.com/dmundt/go-mvu/theme"
	"github.com/dmundt/go-mvu/timing"
)

type testModel struct{ Count int }

type testMessage int

func (testMessage) MessageName() string { return "test.add" }

type testApp struct{ initCommand bool }

func (app testApp) Init(context.Context) (testModel, []mvu.Command[testMessage], error) {
	if !app.initCommand {
		return testModel{}, nil, nil
	}
	initCommand := mvu.CommandFunc[testMessage](
		func(ctx context.Context, dispatch mvu.Dispatch[testMessage]) error {
			return dispatch(ctx, 10)
		},
	)
	return testModel{}, []mvu.Command[testMessage]{initCommand}, nil
}

func (testApp) Update(
	_ context.Context, model testModel, message testMessage,
) (testModel, []mvu.Command[testMessage], error) {
	if message < 0 {
		return model, nil, errors.New("negative")
	}
	model.Count += int(message)
	return model, nil, nil
}

type testFragment struct {
	html.FragmentMarker
	Count int
	Fail  bool
}

func (fragment testFragment) RenderHTML(_ context.Context, out *html.Context) error {
	out.Static("<output>")
	out.Int(fragment.Count)
	out.Static("</output>")
	if fragment.Fail {
		out.Fail(errors.New("render broke"))
	}
	return out.Err()
}

type testPage struct {
	html.PageMarker
	Count int
	Mode  theme.Mode
	URL   string
}

func (page testPage) RenderHTML(_ context.Context, out *html.Context) error {
	out.Static("<!doctype html><main data-mode=\"")
	out.Text(page.Mode.String())
	out.Static("\" data-url=\"")
	out.Text(page.URL)
	out.Static("\">")
	out.Int(page.Count)
	out.Static("</main>")
	return out.Err()
}

type testView struct{ failFragment bool }

func (testView) Page(_ context.Context, model testModel, request RequestContext) (html.Page, error) {
	return testPage{Count: model.Count, Mode: request.Mode, URL: request.CurrentURL}, nil
}

func (view testView) Fragment(
	_ context.Context, name string, model testModel, _ RequestContext,
) (html.Fragment, error) {
	if name != "count" {
		return nil, ErrUnknownFragment
	}
	return testFragment{Count: model.Count, Fail: view.failFragment}, nil
}

func fixedKey(*http.Request) (string, error) { return "client", nil }

func newTestRuntime(t *testing.T, mutate func(*RuntimeConfig[string, testModel, testMessage])) *Runtime[string, testModel, testMessage] {
	t.Helper()
	config := RuntimeConfig[string, testModel, testMessage]{
		App:    testApp{},
		Store:  store.NewMemory[string, testModel](),
		Keys:   KeyResolverFunc[string](fixedKey),
		View:   testView{},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	if mutate != nil {
		mutate(&config)
	}
	runtime, err := NewRuntime(config)
	if err != nil {
		t.Fatal(err)
	}
	return runtime
}

func addRoute(result Result) Route[testMessage] {
	return Route[testMessage]{
		Method: http.MethodPost, Pattern: "/add",
		Decode: StaticMessage[testMessage]{Message: 1},
		Result: result,
	}
}

func serve(handler http.Handler, request *http.Request) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestNewRuntimeRejectsIncompleteConfig(t *testing.T) {
	if _, err := NewRuntime(RuntimeConfig[string, testModel, testMessage]{}); !errors.Is(err, ErrInvalidRuntime) {
		t.Fatalf("err = %v, want ErrInvalidRuntime", err)
	}
}

func TestHandleInitializesUpdatesRunsCommandsAndRendersFragment(t *testing.T) {
	runtime := newTestRuntime(t, func(c *RuntimeConfig[string, testModel, testMessage]) {
		c.App = testApp{initCommand: true}
	})
	response := serve(runtime.Handle(addRoute(Result{Mode: RenderFragment, Fragment: "count"})),
		httptest.NewRequest(http.MethodPost, "/add", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body)
	}
	if got := response.Body.String(); got != "<output>11</output>" {
		t.Fatalf("body = %q", got)
	}
	if got := response.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
}

func TestPageRendersFullDocumentWithRequestContext(t *testing.T) {
	runtime := newTestRuntime(t, nil)
	request := httptest.NewRequest(http.MethodGet, "/?page=2", nil)
	request = request.WithContext(theme.WithMode(request.Context(), theme.Dark))
	response := serve(runtime.Page(Result{Mode: RenderPage}), request)
	want := `<!doctype html><main data-mode="dark" data-url="/?page=2">0</main>`
	if response.Code != http.StatusOK || response.Body.String() != want {
		t.Fatalf("got %d %q", response.Code, response.Body)
	}
}

type fixedResolver struct{ mode theme.Mode }

func (r fixedResolver) Resolve(*http.Request) (theme.Mode, error) { return r.mode, nil }
func (fixedResolver) Persist(http.ResponseWriter, *http.Request, theme.Mode) error {
	return nil
}

func TestThemeResolverOverridesContextMode(t *testing.T) {
	runtime := newTestRuntime(t, func(c *RuntimeConfig[string, testModel, testMessage]) {
		c.Themes = fixedResolver{mode: theme.Light}
	})
	response := serve(runtime.Page(Result{Mode: RenderPage}),
		httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(response.Body.String(), `data-mode="light"`) {
		t.Fatalf("body = %q", response.Body)
	}
}

func TestRenderAutoSelectsFragmentRedirectOrPage(t *testing.T) {
	runtime := newTestRuntime(t, nil)
	handler := runtime.Handle(addRoute(Result{Mode: RenderAuto, Fragment: "count", Location: "/"}))

	htmxRequest := httptest.NewRequest(http.MethodPost, "/add", nil)
	htmxRequest.Header.Set("HX-Request", "true")
	if got := serve(handler, htmxRequest); got.Body.String() != "<output>1</output>" {
		t.Fatalf("htmx body = %q", got.Body)
	}

	plain := serve(handler, httptest.NewRequest(http.MethodPost, "/add", nil))
	if plain.Code != http.StatusSeeOther || plain.Header().Get("Location") != "/" {
		t.Fatalf("plain = %d %q", plain.Code, plain.Header().Get("Location"))
	}

	pageOnly := runtime.Handle(addRoute(Result{Mode: RenderAuto, Fragment: "count"}))
	boosted := httptest.NewRequest(http.MethodPost, "/add", nil)
	boosted.Header.Set("HX-Request", "true")
	boosted.Header.Set("HX-Boosted", "true")
	if got := serve(pageOnly, boosted); !strings.HasPrefix(got.Body.String(), "<!doctype html>") {
		t.Fatalf("boosted body = %q", got.Body)
	}
}

func TestResultStatusAndHTMXHeaders(t *testing.T) {
	runtime := newTestRuntime(t, nil)
	handler := runtime.Handle(addRoute(Result{
		Mode: RenderFragment, Fragment: "count", Status: http.StatusAccepted,
		PushURL: "/pushed", ReplaceURL: "/replaced",
	}))
	response := serve(handler, httptest.NewRequest(http.MethodPost, "/add", nil))
	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Header().Get("HX-Push-Url") != "/pushed" || response.Header().Get("HX-Replace-Url") != "/replaced" {
		t.Fatalf("headers = %v", response.Header())
	}
}

func TestRenderNothingWritesNoContent(t *testing.T) {
	runtime := newTestRuntime(t, nil)
	response := serve(runtime.Handle(addRoute(Result{Mode: RenderNothing})),
		httptest.NewRequest(http.MethodPost, "/add", nil))
	if response.Code != http.StatusNoContent || response.Body.Len() != 0 {
		t.Fatalf("got %d %q", response.Code, response.Body)
	}
}

func TestRouteRejectsWrongMethodAndPath(t *testing.T) {
	handler := newTestRuntime(t, nil).Handle(addRoute(Result{Mode: RenderNothing}))
	if got := serve(handler, httptest.NewRequest(http.MethodGet, "/add", nil)); got.Code != http.StatusMethodNotAllowed || got.Header().Get("Allow") != http.MethodPost {
		t.Fatalf("method = %d %q", got.Code, got.Header().Get("Allow"))
	}
	if got := serve(handler, httptest.NewRequest(http.MethodPost, "/other", nil)); got.Code != http.StatusNotFound {
		t.Fatalf("path = %d", got.Code)
	}
}

func TestErrorsUseErrorHandlerWithoutLeakingDetails(t *testing.T) {
	var captured error
	runtime := newTestRuntime(t, func(c *RuntimeConfig[string, testModel, testMessage]) {
		c.OnError = func(w http.ResponseWriter, r *http.Request, err error) {
			captured = err
			InternalServerError(w, r, err)
		}
	})
	response := serve(runtime.Handle(addRoute(Result{Mode: RenderFragment, Fragment: "missing"})),
		httptest.NewRequest(http.MethodPost, "/add", nil))
	if response.Code != http.StatusInternalServerError || !errors.Is(captured, ErrUnknownFragment) {
		t.Fatalf("got %d err=%v", response.Code, captured)
	}
	if strings.Contains(response.Body.String(), "unknown") {
		t.Fatalf("error leaked: %q", response.Body)
	}

	update := Route[testMessage]{Decode: StaticMessage[testMessage]{Message: -1}, Result: Result{Mode: RenderNothing}}
	if got := serve(runtime.Handle(update), httptest.NewRequest(http.MethodPost, "/", nil)); got.Code != http.StatusInternalServerError {
		t.Fatalf("update error status = %d", got.Code)
	}
}

func TestRenderFailureAfterResponseStartedIsLoggedNotRewritten(t *testing.T) {
	handlerCalled := false
	runtime := newTestRuntime(t, func(c *RuntimeConfig[string, testModel, testMessage]) {
		c.View = testView{failFragment: true}
		c.OnError = func(http.ResponseWriter, *http.Request, error) { handlerCalled = true }
	})
	response := serve(runtime.Handle(addRoute(Result{Mode: RenderFragment, Fragment: "count"})),
		httptest.NewRequest(http.MethodPost, "/add", nil))
	if handlerCalled {
		t.Fatal("OnError called after response started")
	}
	if response.Code != http.StatusOK || response.Body.String() != "<output>1</output>" {
		t.Fatalf("got %d %q", response.Code, response.Body)
	}
}

// conflictStore fails the first CAS to force a retry.
type conflictStore struct {
	*store.Memory[string, testModel]
	conflicts int
}

func (s *conflictStore) CompareAndSwap(
	ctx context.Context, key string, version store.Version, model testModel,
) (store.Snapshot[testModel], error) {
	if s.conflicts > 0 {
		s.conflicts--
		if _, err := s.Memory.CompareAndSwap(ctx, key, version, testModel{Count: 100}); err != nil {
			return store.Snapshot[testModel]{}, err
		}
		return store.Snapshot[testModel]{}, store.ErrConflict
	}
	return s.Memory.CompareAndSwap(ctx, key, version, model)
}

func TestConflictRetriesAgainstLatestModel(t *testing.T) {
	backing := &conflictStore{Memory: store.NewMemory[string, testModel](), conflicts: 1}
	runtime := newTestRuntime(t, func(c *RuntimeConfig[string, testModel, testMessage]) { c.Store = backing })
	response := serve(runtime.Handle(addRoute(Result{Mode: RenderFragment, Fragment: "count"})),
		httptest.NewRequest(http.MethodPost, "/add", nil))
	if got := response.Body.String(); got != "<output>101</output>" {
		t.Fatalf("body = %q", got)
	}

	backing.conflicts = 10
	if got := serve(runtime.Handle(addRoute(Result{Mode: RenderNothing})),
		httptest.NewRequest(http.MethodPost, "/add", nil)); got.Code != http.StatusInternalServerError {
		t.Fatalf("exhausted retries status = %d", got.Code)
	}
}

// lockProbeStore reports whether a command ran while CAS was in progress.
type lockProbeStore struct {
	*store.Memory[string, testModel]
	mu     sync.Mutex
	inCAS  bool
	inside bool
}

func (s *lockProbeStore) CompareAndSwap(
	ctx context.Context, key string, version store.Version, model testModel,
) (store.Snapshot[testModel], error) {
	s.mu.Lock()
	s.inCAS = true
	s.mu.Unlock()
	defer func() { s.mu.Lock(); s.inCAS = false; s.mu.Unlock() }()
	return s.Memory.CompareAndSwap(ctx, key, version, model)
}

type commandApp struct {
	testApp
	probe *lockProbeStore
}

func (app commandApp) Update(
	ctx context.Context, model testModel, message testMessage,
) (testModel, []mvu.Command[testMessage], error) {
	model, _, err := app.testApp.Update(ctx, model, message)
	if message != 1 {
		return model, nil, err
	}
	command := mvu.CommandFunc[testMessage](func(ctx context.Context, dispatch mvu.Dispatch[testMessage]) error {
		app.probe.mu.Lock()
		app.probe.inside = app.probe.inside || app.probe.inCAS
		app.probe.mu.Unlock()
		return dispatch(ctx, 5)
	})
	return model, []mvu.Command[testMessage]{command}, err
}

func TestCommandsRunOutsideStoreAndDispatchFollowups(t *testing.T) {
	probe := &lockProbeStore{Memory: store.NewMemory[string, testModel]()}
	runtime := newTestRuntime(t, func(c *RuntimeConfig[string, testModel, testMessage]) {
		c.Store = probe
		c.App = commandApp{probe: probe}
	})
	response := serve(runtime.Handle(addRoute(Result{Mode: RenderFragment, Fragment: "count"})),
		httptest.NewRequest(http.MethodPost, "/add", nil))
	if got := response.Body.String(); got != "<output>6</output>" {
		t.Fatalf("body = %q", got)
	}
	if probe.inside {
		t.Fatal("command ran during compare-and-swap")
	}
}

func TestAsyncCommandsCompleteAfterResponse(t *testing.T) {
	probe := &lockProbeStore{Memory: store.NewMemory[string, testModel]()}
	runtime := newTestRuntime(t, func(c *RuntimeConfig[string, testModel, testMessage]) {
		c.Store = probe
		c.App = commandApp{probe: probe}
		c.Commands = AsyncCommands[testMessage]{OnError: func(err error) { t.Error(err) }}
	})
	serve(runtime.Handle(addRoute(Result{Mode: RenderNothing})), httptest.NewRequest(http.MethodPost, "/add", nil))
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snapshot, err := probe.Load(context.Background(), "client")
		if err == nil && snapshot.Model.Count == 6 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("async command did not dispatch follow-up")
}

func TestNewRequestContextPrefersHTMXCurrentURL(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/add", nil)
	request.Header.Set("HX-Request", "true")
	request.Header.Set("HX-Current-URL", "http://example.test/users?page=3")
	got := NewRequestContext(request, nil, nil)
	if !got.IsHTMX || got.CurrentURL != "http://example.test/users?page=3" || got.Theme == nil || got.Mode != theme.System {
		t.Fatalf("request context = %+v", got)
	}
}

func TestHandleRequireModelReturnsNotFoundWithoutInit(t *testing.T) {
	memory := store.NewMemory[string, testModel]()
	runtime := newTestRuntime(t, func(c *RuntimeConfig[string, testModel, testMessage]) {
		c.Store = memory
	})
	route := addRoute(Result{Mode: RenderFragment, Fragment: "count"})
	route.RequireModel = true
	handler := runtime.Handle(route)
	response := serve(handler, httptest.NewRequest(http.MethodPost, "/add", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
	if _, err := memory.Load(context.Background(), "client"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("model created: err = %v", err)
	}
	if _, err := memory.Create(context.Background(), "client", testModel{Count: 5}); err != nil {
		t.Fatal(err)
	}
	response = serve(handler, httptest.NewRequest(http.MethodPost, "/add", nil))
	if response.Code != http.StatusOK || response.Body.String() != "<output>6</output>" {
		t.Fatalf("got %d %q", response.Code, response.Body)
	}
}

func TestSyncCommandsStopOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ran := 0
	first := mvu.CommandFunc[testMessage](func(context.Context, mvu.Dispatch[testMessage]) error {
		ran++
		cancel()
		return nil
	})
	second := mvu.CommandFunc[testMessage](func(context.Context, mvu.Dispatch[testMessage]) error {
		ran++
		return nil
	})
	err := SyncCommands[testMessage]{}.Run(ctx, []mvu.Command[testMessage]{first, nil, second}, nil)
	if !errors.Is(err, context.Canceled) || ran != 1 {
		t.Fatalf("err = %v, ran = %d", err, ran)
	}
}

func TestHandleCanceledRequestDoesNotRender(t *testing.T) {
	runtime := newTestRuntime(t, func(c *RuntimeConfig[string, testModel, testMessage]) {
		c.App = testApp{initCommand: true}
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest(http.MethodPost, "/add", nil).WithContext(ctx)
	response := serve(runtime.Handle(addRoute(Result{Mode: RenderFragment, Fragment: "count"})), request)
	if response.Code == http.StatusOK {
		t.Fatalf("canceled request rendered: %q", response.Body)
	}
}

func phaseNames(trace *timing.Trace) []string {
	var names []string
	for _, phase := range trace.Phases() {
		names = append(names, phase.Name)
	}
	return names
}

func TestRuntimeRecordsTimingPhases(t *testing.T) {
	runtime := newTestRuntime(t, nil)
	trace := timing.New()
	request := httptest.NewRequest(http.MethodPost, "/add", nil)
	serve(runtime.Handle(addRoute(Result{Mode: RenderFragment, Fragment: "count"})),
		request.WithContext(timing.WithTrace(request.Context(), trace)))
	want := []string{timing.PhaseDecode, timing.PhaseDispatch, timing.PhaseLoad,
		timing.PhaseView, timing.PhaseRender}
	if got := phaseNames(trace); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("Handle phases = %v, want %v", got, want)
	}

	trace = timing.New()
	request = httptest.NewRequest(http.MethodGet, "/", nil)
	serve(runtime.Page(Result{Mode: RenderPage}),
		request.WithContext(timing.WithTrace(request.Context(), trace)))
	want = []string{timing.PhaseLoad, timing.PhaseView, timing.PhaseRender}
	if got := phaseNames(trace); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("Page phases = %v, want %v", got, want)
	}
}
