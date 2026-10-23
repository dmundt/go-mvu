package htmx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/html"
)

func TestActionEscapesAttributeValuesAndRejectsUnsafeURLs(t *testing.T) {
	var out strings.Builder
	action := Action{Method: Post, URL: `/items?a=1&b="2`, Target: `#items"x`, Swap: OuterHTML}
	if err := action.RenderAttributes(html.NewContext(&out)); err != nil {
		t.Fatalf("RenderAttributes() error = %v", err)
	}
	if got, want := out.String(), ` hx-post="/items?a=1&amp;b=&#34;2" hx-target="#items&#34;x" hx-swap="outerHTML"`; got != want {
		t.Fatalf("attributes = %q, want %q", got, want)
	}
	if err := (Action{Method: Get, URL: "javascript:alert(1)"}).Validate(); err == nil {
		t.Fatal("Validate() accepted javascript URL")
	}
}

func TestEveryMethodRendersMatchingAttribute(t *testing.T) {
	for method, name := range map[Method]string{
		Get: "hx-get", Post: "hx-post", Put: "hx-put", Patch: "hx-patch", Delete: "hx-delete",
	} {
		var out strings.Builder
		if err := (Action{Method: method, URL: "/x"}).RenderAttributes(html.NewContext(&out)); err != nil {
			t.Fatal(err)
		}
		if want := " " + name + `="/x"`; out.String() != want {
			t.Errorf("method %d = %q, want %q", method, out.String(), want)
		}
	}
	if (Action{Method: 0, URL: "/x"}).Validate() == nil || (Action{Method: 9, URL: "/x"}).Validate() == nil {
		t.Fatal("invalid method accepted")
	}
}

func TestSwapAndTargetValidation(t *testing.T) {
	for _, swap := range []Swap{InnerHTML, OuterHTML, BeforeBegin, AfterBegin, BeforeEnd, AfterEnd, DeleteSwap, NoSwap} {
		if err := (Action{Method: Get, URL: "/", Swap: swap}).Validate(); err != nil {
			t.Errorf("swap %q rejected: %v", swap, err)
		}
	}
	for name, action := range map[string]Action{
		"unknown swap":      {Method: Get, URL: "/", Swap: "morph"},
		"newline target":    {Method: Get, URL: "/", Target: "#a\nb"},
		"nul select":        {Method: Get, URL: "/", Select: "#a\x00"},
		"empty url":         {Method: Get},
		"backslash url":     {Method: Get, URL: `\\evil`},
		"data url":          {Method: Get, URL: "data:text/html,x"},
		"padded url":        {Method: Get, URL: " /x"},
		"carriage disabled": {Method: Get, URL: "/", DisabledElt: "a\rb"},
	} {
		if action.Validate() == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestRequestHeadersAndResponseHelpers(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	for name, value := range map[string]string{
		"HX-Request": "true", "HX-Boosted": "true", "HX-History-Restore-Request": "true",
		"HX-Current-URL": "/now", "HX-Target": "list", "HX-Trigger": "btn",
		"HX-Trigger-Name": "go", "HX-Prompt": "yes",
	} {
		request.Header.Set(name, value)
	}
	got := FromRequest(request)
	want := Request{IsHTMX: true, IsBoosted: true, IsHistory: true, CurrentURL: "/now",
		Target: "list", Trigger: "btn", TriggerName: "go", Prompt: "yes"}
	if got != want {
		t.Fatalf("FromRequest = %+v", got)
	}
	if FromRequest(httptest.NewRequest(http.MethodGet, "/", nil)).IsHTMX {
		t.Fatal("plain request detected as HTMX")
	}

	w := httptest.NewRecorder()
	Redirect(w, "/r")
	Refresh(w)
	PushURL(w, "/p")
	ReplaceURL(w, "/q")
	Retarget(w, "#t")
	Reswap(w, InnerHTML)
	Reselect(w, "#s")
	Trigger(w, "saved")
	for name, value := range map[string]string{
		"HX-Redirect": "/r", "HX-Refresh": "true", "HX-Push-Url": "/p", "HX-Replace-Url": "/q",
		"HX-Retarget": "#t", "HX-Reswap": "innerHTML", "HX-Reselect": "#s", "HX-Trigger": "saved",
	} {
		if w.Header().Get(name) != value {
			t.Errorf("%s = %q, want %q", name, w.Header().Get(name), value)
		}
	}
}
