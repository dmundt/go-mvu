package benchmarks

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/examples/durable/jsonstore"
	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/middleware"
	"github.com/dmundt/go-mvu/mvu"
	"github.com/dmundt/go-mvu/store"
	"github.com/dmundt/go-mvu/web"
)

type httpBenchMessage struct{ Delta int }

func (httpBenchMessage) MessageName() string { return "benchmark.increment" }

type httpBenchModel struct{ Count int }

type httpBenchApp struct{}

func (httpBenchApp) Init(context.Context) (httpBenchModel, []mvu.Command[httpBenchMessage], error) {
	return httpBenchModel{}, nil, nil
}

func (httpBenchApp) Update(
	_ context.Context,
	model httpBenchModel,
	message httpBenchMessage,
) (httpBenchModel, []mvu.Command[httpBenchMessage], error) {
	model.Count += message.Delta
	return model, nil, nil
}

type httpBenchView struct{}

func (httpBenchView) Page(
	_ context.Context,
	model httpBenchModel,
	_ web.RequestContext,
) (html.Page, error) {
	return components.Document{
		Title: "HTTP benchmark",
		Body:  components.Output{Value: fmt.Sprint(model.Count)},
	}, nil
}

func (httpBenchView) Fragment(
	_ context.Context,
	name string,
	model httpBenchModel,
	_ web.RequestContext,
) (html.Fragment, error) {
	if name != "counter" {
		return nil, web.ErrUnknownFragment
	}
	return httpBenchFragment{Count: model.Count}, nil
}

type httpBenchFragment struct {
	html.FragmentMarker
	Count int
}

func (fragment httpBenchFragment) RenderHTML(_ context.Context, out *html.Context) error {
	out.Static(`<output>`)
	out.Int(fragment.Count)
	out.Static(`</output>`)
	return out.Err()
}

func newHTTPBenchmarkServer(
	b *testing.B,
	modelStore store.Store[string, httpBenchModel],
) *httptest.Server {
	b.Helper()
	if modelStore == nil {
		modelStore = store.NewMemory[string, httpBenchModel]()
	}
	runtime, err := web.NewRuntime(web.RuntimeConfig[string, httpBenchModel, httpBenchMessage]{
		App:   httpBenchApp{},
		Store: modelStore,
		Keys: web.KeyResolverFunc[string](func(r *http.Request) (string, error) {
			if key := r.Header.Get("X-Bench-Session"); key != "" {
				return key, nil
			}
			return "benchmark", nil
		}),
		View: httpBenchView{},
	})
	if err != nil {
		b.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("GET /", runtime.Page(web.Result{Mode: web.RenderPage}))
	mux.Handle("POST /increment", runtime.Handle(web.Route[httpBenchMessage]{
		Method:  http.MethodPost,
		Pattern: "/increment",
		Decode:  web.StaticMessage[httpBenchMessage]{Message: httpBenchMessage{Delta: 1}},
		Result: web.Result{
			Mode: web.RenderFragment, Fragment: "counter",
		},
	}))
	handler := middleware.Chain(
		mux, middleware.RequestID, middleware.SecurityHeaders,
		middleware.Recover, middleware.NoStore,
	)
	server := httptest.NewServer(handler)
	b.Cleanup(server.Close)
	return server
}

func benchmarkHTTPRequests(b *testing.B, method, path string) {
	b.Helper()
	server := newHTTPBenchmarkServer(b, nil)
	client := server.Client()
	url := server.URL + path
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		request, err := http.NewRequest(method, url, nil)
		if err != nil {
			b.Fatal(err)
		}
		response, err := client.Do(request)
		if err != nil {
			b.Fatal(err)
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			b.Fatalf("%s %s: status %d", method, path, response.StatusCode)
		}
		if _, err := io.Copy(io.Discard, response.Body); err != nil {
			response.Body.Close()
			b.Fatal(err)
		}
		if err := response.Body.Close(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHTTPPageGET(b *testing.B) {
	benchmarkHTTPRequests(b, http.MethodGet, "/")
}

func BenchmarkHTTPFragmentPOST(b *testing.B) {
	benchmarkHTTPRequests(b, http.MethodPost, "/increment")
}

func BenchmarkHTTPMixedRequests(b *testing.B) {
	server := newHTTPBenchmarkServer(b, nil)
	client := server.Client()
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		method, path := http.MethodGet, "/"
		if i%10 == 0 {
			method, path = http.MethodPost, "/increment"
		}
		request, err := http.NewRequest(method, server.URL+path, nil)
		if err != nil {
			b.Fatal(err)
		}
		response, err := client.Do(request)
		if err != nil {
			b.Fatal(err)
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			b.Fatalf("%s %s: status %d", method, path, response.StatusCode)
		}
		if _, err := io.Copy(io.Discard, response.Body); err != nil {
			response.Body.Close()
			b.Fatal(err)
		}
		if err := response.Body.Close(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHTTPParallelMemory(b *testing.B) {
	benchmarkHTTPParallel(b, nil)
}

func BenchmarkHTTPParallelJSONFile(b *testing.B) {
	models, err := jsonstore.Open[httpBenchModel](filepath.Join(b.TempDir(), "models"))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := models.Close(); err != nil {
			b.Errorf("close JSON file store: %v", err)
		}
	})
	benchmarkHTTPParallel(b, models)
}

func benchmarkHTTPParallel(
	b *testing.B,
	modelStore store.Store[string, httpBenchModel],
) {
	server := newHTTPBenchmarkServer(b, modelStore)
	client := server.Client()
	url := server.URL
	var nextSession atomic.Uint64
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		key := fmt.Sprintf("worker-%d", nextSession.Add(1))
		requestCount := 0
		for pb.Next() {
			method, path := http.MethodGet, "/"
			if requestCount%10 == 0 {
				method, path = http.MethodPost, "/increment"
			}
			requestCount++
			request, err := http.NewRequest(method, url+path, nil)
			if err != nil {
				b.Error(err)
				return
			}
			request.Header.Set("X-Bench-Session", key)
			response, err := client.Do(request)
			if err != nil {
				b.Error(err)
				return
			}
			if response.StatusCode != http.StatusOK {
				response.Body.Close()
				b.Errorf("%s %s: status %d", method, path, response.StatusCode)
				return
			}
			if _, err := io.Copy(io.Discard, response.Body); err != nil {
				response.Body.Close()
				b.Error(err)
				return
			}
			if err := response.Body.Close(); err != nil {
				b.Error(err)
				return
			}
		}
	})
}

type fixedRTTTransport struct {
	base http.RoundTripper
}

func (transport fixedRTTTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	time.Sleep(10 * time.Millisecond)
	response, err := transport.base.RoundTrip(request)
	if err != nil {
		return nil, err
	}
	time.Sleep(10 * time.Millisecond)
	return response, nil
}

func BenchmarkHTTPLoopbackRTT20ms(b *testing.B) {
	server := newHTTPBenchmarkServer(b, nil)
	client := &http.Client{
		Transport: fixedRTTTransport{base: server.Client().Transport},
	}
	url := server.URL + "/"
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		request, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			b.Fatal(err)
		}
		response, err := client.Do(request)
		if err != nil {
			b.Fatal(err)
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			b.Fatalf("GET /: status %d", response.StatusCode)
		}
		if _, err := io.Copy(io.Discard, response.Body); err != nil {
			response.Body.Close()
			b.Fatal(err)
		}
		if err := response.Body.Close(); err != nil {
			b.Fatal(err)
		}
	}
}
