package htmx

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/dmundt/go-mvu/html"
)

type Method uint8

const (
	Get Method = iota + 1
	Post
	Put
	Patch
	Delete
)

type Swap string

const (
	InnerHTML   Swap = "innerHTML"
	OuterHTML   Swap = "outerHTML"
	BeforeBegin Swap = "beforebegin"
	AfterBegin  Swap = "afterbegin"
	BeforeEnd   Swap = "beforeend"
	AfterEnd    Swap = "afterend"
	DeleteSwap  Swap = "delete"
	NoSwap      Swap = "none"
)

var ErrInvalidAction = errors.New("invalid HTMX action")

type Action struct {
	Method     Method
	URL        string
	Target     string
	Swap       Swap
	Trigger    string
	Select     string
	Include    string
	Indicator  string
	Confirm    string
	PushURL    string
	ReplaceURL string
	// DisabledElt is a selector disabled while the request is in flight.
	DisabledElt string
	Boost       bool
}

func (a Action) Validate() error {
	if a.Method < Get || a.Method > Delete || a.URL == "" ||
		a.URL != strings.TrimSpace(a.URL) || strings.ContainsAny(a.URL, "\x00\r\n\t\f\v\\") {
		return ErrInvalidAction
	}
	if !plainPath(a.URL) {
		parsedURL, err := url.Parse(a.URL)
		scheme := ""
		if parsedURL != nil {
			scheme = strings.ToLower(parsedURL.Scheme)
		}
		if err != nil || (scheme != "" && scheme != "http" && scheme != "https") {
			return ErrInvalidAction
		}
	}
	for _, selector := range [...]string{a.Target, a.Select, a.Include, a.Indicator, a.DisabledElt} {
		if strings.ContainsAny(selector, "\x00\r\n") {
			return ErrInvalidAction
		}
	}
	switch a.Swap {
	case "", InnerHTML, OuterHTML, BeforeBegin, AfterBegin, BeforeEnd, AfterEnd, DeleteSwap, NoSwap:
		return nil
	default:
		return ErrInvalidAction
	}
}

// plainPath reports a root-relative path without escapes or authority.
// Such URLs carry no scheme, so Validate skips url.Parse (allocation).
func plainPath(value string) bool {
	return len(value) > 0 && value[0] == '/' &&
		(len(value) == 1 || value[1] != '/') && !strings.ContainsRune(value, '%')
}

var methodAttributes = [...]string{"", "hx-get", "hx-post", "hx-put", "hx-patch", "hx-delete"}

func (a Action) RenderAttributes(out *html.Context) error {
	if err := a.Validate(); err != nil {
		return err
	}
	writeAttr(out, methodAttributes[a.Method], a.URL)
	writeOptional(out, "hx-target", a.Target)
	writeOptional(out, "hx-swap", string(a.Swap))
	writeOptional(out, "hx-trigger", a.Trigger)
	writeOptional(out, "hx-select", a.Select)
	writeOptional(out, "hx-include", a.Include)
	writeOptional(out, "hx-indicator", a.Indicator)
	writeOptional(out, "hx-confirm", a.Confirm)
	writeOptional(out, "hx-push-url", a.PushURL)
	writeOptional(out, "hx-replace-url", a.ReplaceURL)
	writeOptional(out, "hx-disabled-elt", a.DisabledElt)
	if a.Boost {
		writeAttr(out, "hx-boost", "true")
	}
	return out.Err()
}

func writeOptional(out *html.Context, name, value string) {
	if value != "" {
		writeAttr(out, name, value)
	}
}

func writeAttr(out *html.Context, name, value string) {
	out.Static(" ")
	out.Static(name)
	out.Static(`="`)
	out.Attribute(value)
	out.Static(`"`)
}

type Request struct {
	IsHTMX      bool
	IsBoosted   bool
	IsHistory   bool
	CurrentURL  string
	Target      string
	Trigger     string
	TriggerName string
	Prompt      string
}

func FromRequest(r *http.Request) Request {
	return Request{
		IsHTMX:      r.Header.Get("HX-Request") == "true",
		IsBoosted:   r.Header.Get("HX-Boosted") == "true",
		IsHistory:   r.Header.Get("HX-History-Restore-Request") == "true",
		CurrentURL:  r.Header.Get("HX-Current-URL"),
		Target:      r.Header.Get("HX-Target"),
		Trigger:     r.Header.Get("HX-Trigger"),
		TriggerName: r.Header.Get("HX-Trigger-Name"),
		Prompt:      r.Header.Get("HX-Prompt"),
	}
}

func Redirect(w http.ResponseWriter, location string)   { w.Header().Set("HX-Redirect", location) }
func Refresh(w http.ResponseWriter)                     { w.Header().Set("HX-Refresh", "true") }
func PushURL(w http.ResponseWriter, location string)    { w.Header().Set("HX-Push-Url", location) }
func ReplaceURL(w http.ResponseWriter, location string) { w.Header().Set("HX-Replace-Url", location) }
func Retarget(w http.ResponseWriter, target string)     { w.Header().Set("HX-Retarget", target) }
func Reswap(w http.ResponseWriter, swap Swap)           { w.Header().Set("HX-Reswap", string(swap)) }
func Reselect(w http.ResponseWriter, selector string)   { w.Header().Set("HX-Reselect", selector) }
func Trigger(w http.ResponseWriter, events string)      { w.Header().Set("HX-Trigger", events) }
