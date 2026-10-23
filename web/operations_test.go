package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/dmundt/go-mvu/httpx"
	"github.com/dmundt/go-mvu/mvu"
	"github.com/dmundt/go-mvu/store"
)

func TestRuntimeEventsWithoutOutcomeCallback(t *testing.T) {
	for _, test := range []struct {
		name      string
		event     httpx.Event
		configure func(*RuntimeConfig[string, testModel, testMessage])
		expected  bool
	}{
		{"authorization", httpx.AuthorizationRejected, func(c *RuntimeConfig[string, testModel, testMessage]) {
			c.AuthorizeTransition = func(context.Context, string, testModel, testMessage) error {
				return &httpx.Error{Status: 403}
			}
		}, false},
		{"stale", httpx.StaleAction, func(c *RuntimeConfig[string, testModel, testMessage]) {}, true},
		{"effect", httpx.CommittedEffectFailure, func(c *RuntimeConfig[string, testModel, testMessage]) {
			c.Commands = failingEventCommands{}
			c.App = mvu.Functions[testModel, testMessage]{
				UpdateFunc: func(_ context.Context, model testModel, _ testMessage) (testModel, []mvu.Command[testMessage], error) {
					return model, []mvu.Command[testMessage]{mvu.CommandFunc[testMessage](func(context.Context, mvu.Dispatch[testMessage]) error { return nil })}, nil
				},
			}
		}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			events := map[httpx.Event]int{}
			ctx := httpx.WithEventObserver(context.Background(), func(event httpx.Event) { events[event]++ })
			runtime := newTestRuntime(t, test.configure)
			route := addRoute(Result{Mode: RenderNothing})
			if test.expected {
				route.ExpectedVersion = func(*http.Request) (store.Version, error) { return 999, nil }
			}
			runtime.Handle(route).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/add", nil).WithContext(ctx))
			if events[test.event] != 1 {
				t.Fatalf("events = %v", events)
			}
		})
	}
}

type failingEventCommands struct{}

func (failingEventCommands) Run(context.Context, []mvu.Command[testMessage], mvu.Dispatch[testMessage]) error {
	return errors.New("effect failed")
}

func TestAsyncEventsAndRepeatedClose(t *testing.T) {
	var mu sync.Mutex
	events := make(map[httpx.Event]int)
	ctx := httpx.WithEventObserver(context.Background(), func(event httpx.Event) {
		mu.Lock()
		defer mu.Unlock()
		events[event]++
	})
	reported := make(chan error, 1)
	runner, err := NewAsyncCommandRunner[testMessage](1, 1, time.Minute, func(err error) { reported <- err })
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	work := mvu.CommandFunc[testMessage](func(context.Context, mvu.Dispatch[testMessage]) error {
		close(entered)
		<-release
		return errors.New("downstream failed")
	})
	if err := runner.Run(ctx, []mvu.Command[testMessage]{work}, nil); err != nil {
		t.Fatal(err)
	}
	<-entered
	noop := mvu.CommandFunc[testMessage](func(context.Context, mvu.Dispatch[testMessage]) error { return nil })
	if err := runner.Run(ctx, []mvu.Command[testMessage]{noop}, nil); err != nil {
		t.Fatal(err)
	}
	if err := runner.Run(ctx, []mvu.Command[testMessage]{noop}, nil); !errors.Is(err, ErrCommandOverload) {
		t.Fatalf("saturation = %v", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	done := runner.done
	for range 100 {
		if err := runner.Close(canceled); !errors.Is(err, context.Canceled) || runner.done != done {
			t.Fatal("repeated Close must use shared completion, not new waiters")
		}
	}
	close(release)
	<-reported
	if err := runner.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Once drained, completion takes precedence over a canceled wait context.
	if err := runner.Close(canceled); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if events[httpx.CommandOverload] != 1 || events[httpx.CommittedEffectFailure] != 1 {
		t.Fatalf("async counters = %v", events)
	}
}
