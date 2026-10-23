package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/dmundt/go-mvu/core"
)

func TestRuntimeEventsWithoutOutcomeCallback(t *testing.T) {
	for _, test := range []struct {
		name      string
		event     Event
		configure func(*RuntimeConfig[string, testModel, testMessage])
		expected  bool
	}{
		{"authorization", AuthorizationRejected, func(c *RuntimeConfig[string, testModel, testMessage]) {
			c.AuthorizeTransition = func(context.Context, string, testModel, testMessage) error {
				return &Error{Status: 403}
			}
		}, false},
		{"stale", StaleAction, func(c *RuntimeConfig[string, testModel, testMessage]) {}, true},
		{"effect", CommittedEffectFailure, func(c *RuntimeConfig[string, testModel, testMessage]) {
			c.Commands = failingEventCommands{}
			c.App = core.Functions[testModel, testMessage]{
				UpdateFunc: func(_ context.Context, model testModel, _ testMessage) (testModel, []core.Command[testMessage], error) {
					return model, []core.Command[testMessage]{core.CommandFunc[testMessage](func(context.Context, core.Dispatch[testMessage]) error { return nil })}, nil
				},
			}
		}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			events := map[Event]int{}
			ctx := WithEventObserver(context.Background(), func(event Event) { events[event]++ })
			runtime := newTestRuntime(t, test.configure)
			route := addRoute(Result{Mode: RenderNothing})
			if test.expected {
				route.ExpectedVersion = func(*http.Request) (core.Version, error) { return 999, nil }
			}
			runtime.Handle(route).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/add", nil).WithContext(ctx))
			if events[test.event] != 1 {
				t.Fatalf("events = %v", events)
			}
		})
	}
}

type failingEventCommands struct{}

func (failingEventCommands) Run(context.Context, []core.Command[testMessage], core.Dispatch[testMessage]) error {
	return errors.New("effect failed")
}

func TestAsyncEventsAndRepeatedClose(t *testing.T) {
	var mu sync.Mutex
	events := make(map[Event]int)
	ctx := WithEventObserver(context.Background(), func(event Event) {
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
	work := core.CommandFunc[testMessage](func(context.Context, core.Dispatch[testMessage]) error {
		close(entered)
		<-release
		return errors.New("downstream failed")
	})
	if err := runner.Run(ctx, []core.Command[testMessage]{work}, nil); err != nil {
		t.Fatal(err)
	}
	<-entered
	noop := core.CommandFunc[testMessage](func(context.Context, core.Dispatch[testMessage]) error { return nil })
	if err := runner.Run(ctx, []core.Command[testMessage]{noop}, nil); err != nil {
		t.Fatal(err)
	}
	if err := runner.Run(ctx, []core.Command[testMessage]{noop}, nil); !errors.Is(err, ErrCommandOverload) {
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
	if events[CommandOverload] != 1 || events[CommittedEffectFailure] != 1 {
		t.Fatalf("async counters = %v", events)
	}
}
