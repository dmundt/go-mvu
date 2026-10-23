package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestActionSharesRouteAndHTMXConfiguration(t *testing.T) {
	runtime := newTestRuntime(t, func(config *RuntimeConfig[string, testModel, testMessage]) {
		config.App = testApp{initCommand: true}
	})
	action := Action[testMessage]{
		Method: Post,
		Path:   "/add",
		Decode: StaticMessage[testMessage](1),
		Result: Result{Mode: RenderAuto, Fragment: "count"},
	}
	browser := action.HTMX()
	if browser.Method != Post || browser.URL != action.Path ||
		browser.Target != "#count" || browser.Swap != OuterHTML {
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
		Method: Post,
		Path:   "/add",
		Decode: StaticMessage[testMessage](1),
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

func TestBrowserAndRuntimeActionErrorsRemainDistinct(t *testing.T) {
	browser := BrowserAction{Method: Post, URL: "/add", Target: "\n"}
	if err := browser.Validate(); !errors.Is(err, ErrInvalidHTMXAction) || errors.Is(err, ErrInvalidAction) {
		t.Fatalf("browser error = %v, want only ErrInvalidHTMXAction", err)
	}
	runtime := newTestRuntime(t, nil)
	action := PostMessage[testMessage]("/add", 1, Result{Mode: RenderAuto, Fragment: "count"})
	action.Target = browser.Target
	err := runtime.Register(http.NewServeMux(), action)
	if !errors.Is(err, ErrInvalidAction) || !errors.Is(err, ErrInvalidHTMXAction) {
		t.Fatalf("runtime error = %v, want both runtime and browser causes", err)
	}
	action.Target = ""
	action.Decode = nil
	err = runtime.Register(http.NewServeMux(), action)
	if !errors.Is(err, ErrInvalidAction) || errors.Is(err, ErrInvalidHTMXAction) {
		t.Fatalf("decoder error = %v, want only ErrInvalidAction", err)
	}
}

func TestPostMessageAndBatchRegistration(t *testing.T) {
	runtime := newTestRuntime(t, nil)
	result := Result{Mode: RenderAuto, Fragment: "count", Location: "/"}
	add := PostMessage[testMessage]("/add", 1, result)
	reset := PostMessage[testMessage]("/reset", 2, result)
	message, err := add.Decode(httptest.NewRequest(http.MethodPost, "/add", nil))
	if err != nil || message != 1 {
		t.Fatalf("decoded message = %v, error = %v", message, err)
	}
	if add.Method != Post || add.Result != result || add.HTMX().Target != "#count" {
		t.Fatalf("action = %+v", add)
	}
	if err := runtime.RegisterAll(nil); !errors.Is(err, ErrInvalidAction) {
		t.Fatalf("nil mux error = %v", err)
	}
	var missing *Runtime[string, testModel, testMessage]
	if err := missing.RegisterAll(http.NewServeMux()); !errors.Is(err, ErrInvalidAction) {
		t.Fatalf("nil runtime error = %v", err)
	}
	mux := http.NewServeMux()
	if err := runtime.RegisterAll(mux, add, reset); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/add", "/reset"} {
		response := serve(mux, httptest.NewRequest(http.MethodPost, path, nil))
		if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/" {
			t.Fatalf("%s response = %d %v", path, response.Code, response.Header())
		}
	}
	bad := add
	bad.Path = "//outside"
	mux = http.NewServeMux()
	if err := runtime.RegisterAll(mux, add, bad, reset); !errors.Is(err, ErrInvalidAction) {
		t.Fatalf("batch error = %v", err)
	}
	if response := serve(mux, httptest.NewRequest(http.MethodPost, "/add", nil)); response.Code != http.StatusSeeOther {
		t.Fatalf("earlier action not retained: %d", response.Code)
	}
	if response := serve(mux, httptest.NewRequest(http.MethodPost, "/reset", nil)); response.Code != http.StatusNotFound {
		t.Fatalf("batch continued after error: %d", response.Code)
	}
}
