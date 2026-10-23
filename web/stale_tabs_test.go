package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/dmundt/go-mvu/core"
)

func TestTwoTabsCannotCommitSameExpectedVersion(t *testing.T) {
	memory := core.NewMemory[string, testModel]()
	snapshot, err := memory.Create(context.Background(), "client", testModel{})
	if err != nil {
		t.Fatal(err)
	}
	var barrier sync.WaitGroup
	barrier.Add(2)
	runtime := newTestRuntime(t, func(c *RuntimeConfig[string, testModel, testMessage]) {
		c.Store = memory
		c.AuthorizeTransition = func(context.Context, string, testModel, testMessage) error {
			barrier.Done()
			barrier.Wait()
			return nil
		}
	})
	route := addRoute(Result{Mode: RenderNothing})
	route.ExpectedVersion = func(*http.Request) (core.Version, error) {
		return snapshot.Version, nil
	}
	handler := runtime.Handle(route)
	statuses := make(chan int, 2)
	var group sync.WaitGroup
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/add", nil))
			statuses <- w.Code
		}()
	}
	group.Wait()
	close(statuses)
	counts := map[int]int{}
	for status := range statuses {
		counts[status]++
	}
	if counts[204] != 1 || counts[409] != 1 {
		t.Fatalf("tab responses = %v", counts)
	}
	final, err := memory.Load(context.Background(), "client")
	if err != nil || final.Model.Count != 1 {
		t.Fatalf("stale intent was reapplied: %+v, %v", final, err)
	}
}
