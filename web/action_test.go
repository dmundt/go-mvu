package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dmundt/go-mvu/htmx"
)

func TestActionSharesRouteAndHTMXConfiguration(t *testing.T) {
	runtime := newTestRuntime(t, func(config *RuntimeConfig[string, testModel, testMessage]) {
		config.App = testApp{initCommand: true}
	})
	action := Action[testMessage]{
		Method: htmx.Post,
		Path:   "/add",
		Decode: StaticMessage[testMessage]{Message: 1},
		Result: Result{Mode: RenderAuto, Fragment: "count"},
	}
	browser := action.HTMX()
	if browser.Method != htmx.Post || browser.URL != action.Path ||
		browser.Target != "#count" || browser.Swap != htmx.OuterHTML {
		t.Fatalf("HTMX action = %+v", browser)
	}

	mux := http.NewServeMux()
	if err := runtime.Register(mux, action); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, action.Path, nil)
	request.Header.Set("HX-Request", "true")
	response := serve(mux, request)
	if response.Code != http.StatusOK || response.Body.String() != "<output>11</output>" {
		t.Fatalf("HTMX response = %d %q", response.Code, response.Body.String())
	}

	response = serve(mux, httptest.NewRequest(http.MethodPost, action.Path, nil))
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != action.Path {
		t.Fatalf("plain response = %d %v", response.Code, response.Header())
	}
}

func TestActionRegisterRejectsInvalidConfiguration(t *testing.T) {
	runtime := newTestRuntime(t, nil)
	valid := Action[testMessage]{
		Method: htmx.Post,
		Path:   "/add",
		Decode: StaticMessage[testMessage]{Message: 1},
		Result: Result{Mode: RenderAuto, Fragment: "count"},
	}
	tests := map[string]func(*Action[testMessage]){
		"method":  func(action *Action[testMessage]) { action.Method = 0 },
		"path":    func(action *Action[testMessage]) { action.Path = "//outside" },
		"decoder": func(action *Action[testMessage]) { action.Decode = nil },
		"target":  func(action *Action[testMessage]) { action.Target = "\n" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			action := valid
			mutate(&action)
			if err := runtime.Register(http.NewServeMux(), action); !errors.Is(err, ErrInvalidAction) {
				t.Fatalf("Register error = %v, want ErrInvalidAction", err)
			}
		})
	}
}
