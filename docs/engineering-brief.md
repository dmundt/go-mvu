---
type: Engineering History
title: Engineering brief
description: Original design conversation and project history.
version: 1
---

> Current architecture is summarized in [`architecture.md`](architecture.md), with decisions recorded in [`adr/`](adr/README.md). This conversation history records evolving proposals; later decisions supersede earlier alternatives.

You said:
I want a generic framework with clear borders and components
Copilot said:
Then I would not optimize for rendering speed first. I would optimize for architecture.

The biggest mistake is:



Framework
 ├── Components
 ├── State
 ├── Router
 ├── Rendering
 ├── HTMX
 └── Storage
all mixed together.

Instead create strict boundaries.

Layer 1: MVU Core
No HTML.

No HTMX.

Pure application.



package mvu

type Model interface{}
type Msg interface{}

type App[M any] interface {
	Init() M
	Update(M, Msg) M
}
Example:



type Counter struct{}

func (Counter) Update(
	m Model,
	msg Msg,
) Model {
	...
}
Framework knows nothing about UI.

Layer 2: UI Tree
No HTTP.

No HTMX.

No storage.

Only composition.



package ui

type Component interface {
	Render(Renderer)
}
Example:



type Card struct {
	Title string
	Body  Component
}


type Dashboard struct {
	Counter CounterCard
}
Tree:



Dashboard
 ├── Sidebar
 ├── Topbar
 └── CounterCard
      └── Counter
Layer 3: Renderer
Transforms UI tree into output.



type Renderer interface {
	Open(tag string)
	Close(tag string)

	Text(string)
	Attr(name string, value string)
}
Implementations:



HTMLRenderer
MarkdownRenderer
EmailRenderer
PDFRenderer
TerminalRenderer
Same component works everywhere.



Card.Render(renderer)
Layer 4: Web Adapter
Connects HTMX and HTTP.



package web

type Route struct {
	Path string
	Msg  mvu.Msg
}


POST /counter/inc
        ↓
Increment{}
        ↓
Update()
        ↓
Dashboard component
        ↓
HTML renderer
Layer 5: Themes
Keep styling separate.

Bad:



Card{
    Class: "card shadow"
}
Better:



Card{
    Variant: Primary
}
Theme decides:




Component API
React-like but Go-native.



type Component interface {
	Render(Renderer)
}
Simple:



type Button struct {
	Text string
}
Complex:



type Card struct {
	Header Component
	Body   Component
	Footer Component
}
Layout:



type Stack struct {
	Children []Component
}
Usage:



Dashboard{
	Sidebar: Sidebar{},
	Content: Stack{
		Children: []Component{
			Card{
				Header: Text("Users"),
				Body:   Counter{},
			},
		},
	},
}
Slot System
Like React children.



type Card struct {
	Title Component
	Body  Component
}
Usage:



Card{
	Title: Text("Sales"),
	Body: Chart{},
}
No special framework magic.

Renderer API
This is the critical abstraction.



type Renderer interface {
	Element(
		tag string,
		fn func(),
	)

	Text(string)

	Attribute(
		name string,
		value string,
	)
}
Component:



func (c Card) Render(r Renderer) {
	r.Element("article", func() {

		r.Element("h2", func() {
			c.Title.Render(r)
		})

		c.Body.Render(r)
	})
}
Later:



HTMLRenderer
JSONRenderer
StaticSiteRenderer
all reuse components.

Folder structure


gomvu/

core/
 ├── app.go
 ├── model.go
 └── update.go

ui/
 ├── component.go
 ├── layout.go
 ├── card.go
 ├── form.go
 └── table.go

render/
 ├── renderer.go
 ├── html/
 ├── markdown/
 └── text/

web/
 ├── router.go
 ├── htmx.go
 └── handler.go

theme/
 ├── default/
 └── bootstrap/

example/
What I would build


Core:
  MVU

UI:
  Component tree

Render:
  Renderer interface

Transport:
  HTMX adapter

Theme:
  Separate package

Storage:
  Interface

Codegen:
  Optional later
This gives very clear borders:



MVU
  ↓

Components
  ↓

Renderer
  ↓

HTTP/HTMX
Each layer replaceable. That's a framework architecture that can survive years of evolution, unlike a direct HTML-writer framework where rendering, components, and transport become tightly coupled.





You said:
No the framework should render only html and htmx no js ssr
Copilot said:
Then I'd design it much more narrowly.

Not:



MVU
 ↓
Component Tree
 ↓
Generic Renderer
 ↓
HTML
Markdown
PDF
JSON
...
But:



MVU
 ↓
HTML Components
 ↓
HTML Renderer
 ↓
HTMX
 ↓
Browser
Single target.

No abstraction for other renderers.

No generic UI layer.

No VDOM.

No templates.

No JS framework.

Framework boundaries


app/
 ├── model
 ├── messages
 └── update

view/
 ├── components
 └── pages

web/
 ├── router
 ├── htmx
 └── sessions

render/
 └── html
Only one renderer:



type Component interface {
	RenderHTML(*Context) error
}
Everything implements it.

Component API
React-like nesting:



Page{
	Body: Layout{
		Sidebar: Sidebar{},

		Content: Dashboard{
			Cards: []Card{
				{
					Title: "Users",
					Body: UserStats{},
				},
			},
		},
	},
}
Render chain:



Page
 ↓
Layout
 ↓
Dashboard
 ↓
Card
 ↓
UserStats
No intermediary tree.

Each component directly renders children.

Slots


type Card struct {
	Header Component
	Body   Component
	Footer Component
}
Usage:



Card{
	Header: Text("Sales"),
	Body: SalesChart{},
	Footer: Text("Updated now"),
}
Equivalent to React children.

HTMX component


type Counter struct {
	Count int
}


func (c Counter) RenderHTML(
	ctx *Context,
) error {

	ctx.Write(`
<div id="counter">
`)

	ctx.Write(`
<button
 hx-post="/counter/dec"
 hx-target="#counter"
 hx-swap="outerHTML">
 -
</button>
`)

	ctx.Int(c.Count)

	ctx.Write(`
<button
 hx-post="/counter/inc"
 hx-target="#counter"
 hx-swap="outerHTML">
 +
</button>
</div>
`)

	return ctx.Err()
}
Component owns interaction.

No JavaScript.

Pages vs Fragments
Clear distinction.



type Page interface {
	RenderPage(*Context) error
}

type Fragment interface {
	RenderHTML(*Context) error
}
Example:



GET /
returns




Example:



POST /counter/inc
returns




only.

MVU
Pure update.



func Update(
	model Model,
	msg Msg,
) Model
No HTML inside update.

No HTTP inside update.

No components inside update.

Event mapping


POST /counter/inc
      ↓
Increment{}
      ↓
Update()
      ↓
Counter component
      ↓
HTML fragment
Framework helper:



Handle(
	"/counter/inc",
	Increment{},
	RenderCounter,
)
Context
Only HTML concerns.



type Context struct {
	w   io.Writer
	err error
}
Methods:



Text(string)
Attr(string)
Raw(string) // internal
Int(int)
Bool(bool)

Component(Component)
No generic rendering abstraction.

HTML only.

Final architecture


Model
  ↓

Update
  ↓

Component Tree
  ↓

Direct HTML Renderer
  ↓

HTMX
  ↓

Browser
If the framework's mission is:



Go
SSR
HTML
HTMX
No SPA
No React
No JS app code
then the cleanest design is:



Component interface
MVU runtime
HTML renderer
HTMX adapter
and nothing else. Every extra abstraction should be justified by actual HTML/HTMX use cases, not future portability.





You said:
Show custom ecample component pagination
Copilot said:
A pagination component is a great test because it has:



State
HTMX actions
Conditional rendering
Reusable component API
Nested composition
SSR only
Model


type PaginationModel struct {
	Page       int
	PageSize   int
	TotalItems int
}
Methods:



func (m PaginationModel) Pages() int {
	return (m.TotalItems + m.PageSize - 1) / m.PageSize
}

func (m PaginationModel) HasPrevious() bool {
	return m.Page > 1
}

func (m PaginationModel) HasNext() bool {
	return m.Page < m.Pages()
}
Component


type Pagination struct {
	Model  PaginationModel
	Target string
	Route  string
}
Example usage:



Pagination{
	Model: PaginationModel{
		Page:       4,
		PageSize:   25,
		TotalItems: 240,
	},
	Route:  "/users/page",
	Target: "#users-table",
}
Component renderer


func (p Pagination) RenderHTML(
	ctx *Context,
) error {

	model := p.Model

	ctx.S(`<nav class="pagination">`)

	// Previous

	if model.HasPrevious() {

		ctx.S(`<button
			hx-get="`)
		ctx.Attr(p.Route)
		ctx.S(`?page=`)

		ctx.Int(model.Page - 1)

		ctx.S(`"
			hx-target="`)
		ctx.Attr(p.Target)
		ctx.S(`"
			hx-swap="outerHTML">
			Previous
		</button>`)
	}

	// Pages

	for page := 1; page <= model.Pages(); page++ {

		if page == model.Page {

			ctx.S(`<span class="active">`)
			ctx.Int(page)
			ctx.S(`</span>`)

			continue
		}

		ctx.S(`<button
			hx-get="`)
		ctx.Attr(p.Route)
		ctx.S(`?page=`)

		ctx.Int(page)

		ctx.S(`"
			hx-target="`)
		ctx.Attr(p.Target)
		ctx.S(`"
			hx-swap="outerHTML">`)

		ctx.Int(page)

		ctx.S(`</button>`)
	}

	// Next

	if model.HasNext() {

		ctx.S(`<button
			hx-get="`)
		ctx.Attr(p.Route)
		ctx.S(`?page=`)

		ctx.Int(model.Page + 1)

		ctx.S(`"
			hx-target="`)
		ctx.Attr(p.Target)
		ctx.S(`"
			hx-swap="outerHTML">
			Next
		</button>`)
	}

	ctx.S(`</nav>`)

	return ctx.Err()
}
Table Component


type UsersTable struct {
	Users      []User
	Pagination Pagination
}


func (t UsersTable) RenderHTML(
	ctx *Context,
) error {

	ctx.S(`
<div id="users-table">
`)

	ctx.S(`
<table>
<thead>
<tr>
<th>Name</th>
<th>Email</th>
</tr>
</thead>
<tbody>
`)

	for _, user := range t.Users {

		ctx.S(`<tr>`)

		ctx.S(`<td>`)
		ctx.Text(user.Name)
		ctx.S(`</td>`)

		ctx.S(`<td>`)
		ctx.Text(user.Email)
		ctx.S(`</td>`)

		ctx.S(`</tr>`)
	}

	ctx.S(`
</tbody>
</table>
`)

	t.Pagination.RenderHTML(ctx)

	ctx.S(`</div>`)

	return ctx.Err()
}
Page composition
Looks React-like:



func UsersPage(
	model UsersPageModel,
) Component {

	return Layout{
		Content: UsersTable{
			Users: model.Users,

			Pagination: Pagination{
				Model: model.Pagination,
				Route: "/users/page",
				Target: "#users-table",
			},
		},
	}
}
HTMX endpoint


func UsersPageFragment(
	w http.ResponseWriter,
	r *http.Request,
) {

	page, _ := strconv.Atoi(
		r.URL.Query().Get("page"),
	)

	model := loadUsers(page)

	Render(
		w,

		UsersTable{
			Users: model.Users,

			Pagination: Pagination{
				Model: model.Pagination,
				Route: "/users/page",
				Target: "#users-table",
			},
		},
	)
}
Generated HTML
Page 4 selected:




More reusable API
Instead of page-specific routes:



Pagination{
	Model: model.Pagination,
	Route: "/users/page",
	Target: "#users-table",
}
Define:



type Pagination struct {
	Page       int
	Pages      int

	GetURL func(page int) string

	Target string
}
Usage:



Pagination{
	Page:  model.Page,
	Pages: model.Pages,

	GetURL: func(page int) string {
		return fmt.Sprintf(
			"/users/page?page=%d",
			page,
		)
	},

	Target: "#users-table",
}
Now the component works for:



Users
Products
Orders
Invoices
Logs
Search results
Blog listing
with zero modification.

This is the kind of component API I'd build for the framework:



Component
├── Card
├── Stack
├── Grid
├── Table
├── Form
├── Modal
├── Pagination
├── Tabs
├── Sidebar
├── Breadcrumbs
└── Alert
all implementing:



type Component interface {
	RenderHTML(*Context) error
}
with HTMX behavior embedded directly in the component output.





You said:
Show the complete framework API
Copilot said:
Yes. Narrow framework: Go SSR + HTML components + HTMX transport + MVU runtime. Clear package borders; direct streaming renderer; no VDOM, reflection, runtime templates, hydration, JSON UI protocol, or custom browser JavaScript.

HTMX requests should receive HTML fragments, then swap those fragments into the selected target.
htmx

1. Package layout


gomvu/
├── mvu/          Pure Model-View-Update contracts
├── html/         Safe direct HTML rendering primitives
├── htmx/         Typed HTMX attributes and request metadata
├── web/          HTTP runtime, routes, page/fragment responses
├── store/        Model persistence contracts
├── session/      Session-scoped application instances
├── validation/   Form validation
├── components/   Optional reusable components
└── middleware/   CSRF, recovery, logging, security headers
Dependency direction:



components ───────→ html
web ──────────────→ mvu, html, htmx, store
session ──────────→ store
htmx ─────────────→ html
mvu ──────────────→ nothing
html ─────────────→ standard library only
store ────────────→ standard library only
Forbidden dependencies:



mvu     must not import html, htmx, web, store
html    must not import mvu, htmx, web
store   must not import html, htmx, web
Update  must not render HTML or inspect HTTP requests
View    must not mutate state
2. html package
Core contracts


package html

import (
	"context"
	"io"
)

type Component interface {
	RenderHTML(context.Context, *Context) error
}

type ComponentFunc func(context.Context, *Context) error

func (fn ComponentFunc) RenderHTML(
	ctx context.Context,
	out *Context,
) error {
	return fn(ctx, out)
}

type Page interface {
	Component
	IsPage()
}

type Fragment interface {
	Component
	IsFragment()
}
Markers prevent accidentally returning a full document to a fragment endpoint:



type PageMarker struct{}

func (PageMarker) IsPage() {}

type FragmentMarker struct{}

func (FragmentMarker) IsFragment() {}
Renderer context


type Context struct {
	writer io.Writer
	err    error
}

func NewContext(w io.Writer) *Context
func Render(context.Context, io.Writer, Component) error

func (c *Context) Err() error
func (c *Context) Writer() io.Writer

func (c *Context) Static(string)
func (c *Context) Text(string)
func (c *Context) Attribute(string)

func (c *Context) Int(int)
func (c *Context) Int64(int64)
func (c *Context) Uint64(uint64)
func (c *Context) Float64(float64)
func (c *Context) Bool(bool)

func (c *Context) Component(context.Context, Component)
func (c *Context) Components(context.Context, []Component)
Rules:



Static       framework/developer-owned HTML only
Text         escaped HTML text
Attribute    escaped HTML attribute value
Component    nested rendering
Err          first sticky rendering error
Trusted HTML
Do not expose Raw(string).



type Trusted string

func TrustStatic(value string) Trusted

func (c *Context) Trusted(value Trusted) {
	c.Static(string(value))
}
Document loudly:



TrustStatic accepts only compile-time or audited server-controlled HTML.
Never pass request, database, form, URL, or user content.
Optional element builder
Keep outside core render path:



type Attributes func(*Context)

func Attr(name, value string) Attributes
func BoolAttr(name string, enabled bool) Attributes

func Element(
	ctx context.Context,
	out *Context,
	tag string,
	attributes Attributes,
	body Component,
) error
Primary components should still use fixed static chunks for speed.

3. Component composition
Empty component


type Empty struct{}

func (Empty) RenderHTML(context.Context, *Context) error {
	return nil
}
Text component


type Text string

func (value Text) RenderHTML(
	_ context.Context,
	out *Context,
) error {
	out.Text(string(value))
	return out.Err()
}
Sequence
Useful for dynamic component lists. Not guaranteed zero-allocation if constructed per render.



type Sequence []Component

func (items Sequence) RenderHTML(
	ctx context.Context,
	out *Context,
) error {
	for _, item := range items {
		out.Component(ctx, item)

		if out.Err() != nil {
			break
		}
	}

	return out.Err()
}
Conditional component


type Optional struct {
	Show      bool
	Component Component
}
Preferred hot-path form:



if card.Footer != nil {
	out.Component(ctx, card.Footer)
}
Slot-based component


type Card struct {
	ID      string
	Header  Component
	Body    Component
	Footer  Component
	Variant CardVariant
}
Rendering:



func (card Card) RenderHTML(
	ctx context.Context,
	out *html.Context,
) error {
	out.Static(`<article`)

	if card.ID != "" {
		out.Static(` id="`)
		out.Attribute(card.ID)
		out.Static(`"`)
	}

	out.Static(` class="card card--`)
	out.Attribute(string(card.Variant))
	out.Static(`">`)

	if card.Header != nil {
		out.Static(`<header class="card__header">`)
		out.Component(ctx, card.Header)
		out.Static(`</header>`)
	}

	out.Static(`<div class="card__body">`)
	out.Component(ctx, card.Body)
	out.Static(`</div>`)

	if card.Footer != nil {
		out.Static(`<footer class="card__footer">`)
		out.Component(ctx, card.Footer)
		out.Static(`</footer>`)
	}

	out.Static(`</article>`)
	return out.Err()
}
4. htmx package
HTMX attributes describe the request URL, target, and swap behavior. The server normally responds with HTML rather than JSON.
htmx

Typed method


package htmx

type Method uint8

const (
	Get Method = iota + 1
	Post
	Put
	Patch
	Delete
)
Typed swap


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
Action


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
	Disabled   string
	Boost      bool
}

func (a Action) RenderAttributes(*html.Context)
func (a Action) Validate() error
Usage:



Action{
	Method: Post,
	URL:    "/counter/increment",
	Target: "#counter",
	Swap:   OuterHTML,
}.RenderAttributes(out)
Output:




Request metadata


type Request struct {
	IsHTMX         bool
	IsBoosted      bool
	IsHistory      bool
	CurrentURL     string
	Target         string
	Trigger        string
	TriggerName    string
	Prompt         string
}

func FromRequest(*http.Request) Request
Response headers


func Redirect(http.ResponseWriter, string)
func Refresh(http.ResponseWriter)
func PushURL(http.ResponseWriter, string)
func ReplaceURL(http.ResponseWriter, string)
func Retarget(http.ResponseWriter, string)
func Reswap(http.ResponseWriter, Swap)
func Reselect(http.ResponseWriter, string)
func Trigger(http.ResponseWriter, string)
5. mvu package
MVU core stays unaware of HTTP and HTML.



package mvu

import "context"

type Message interface {
	MessageName() string
}

type Command[M Message] interface {
	Execute(context.Context, Dispatch[M]) error
}

type CommandFunc[M Message] func(
	context.Context,
	Dispatch[M],
) error

func (fn CommandFunc[M]) Execute(
	ctx context.Context,
	dispatch Dispatch[M],
) error {
	return fn(ctx, dispatch)
}

type Dispatch[M Message] func(context.Context, M) error
Application contract:



type Application[Model any, Msg Message] interface {
	Init(context.Context) (Model, []Command[Msg], error)

	Update(
		context.Context,
		Model,
		Msg,
	) (Model, []Command[Msg], error)
}
Optional pure-function adapter:



type Functions[Model any, Msg Message] struct {
	InitFunc func(context.Context) (
		Model,
		[]Command[Msg],
		error,
	)

	UpdateFunc func(
		context.Context,
		Model,
		Msg,
	) (
		Model,
		[]Command[Msg],
		error,
	)
}

func (f Functions[Model, Msg]) Init(
	ctx context.Context,
) (Model, []Command[Msg], error)

func (f Functions[Model, Msg]) Update(
	ctx context.Context,
	model Model,
	msg Msg,
) (Model, []Command[Msg], error)
No View in the core application interface. View belongs to web composition:



MVU core owns transitions
Web view owns HTML projection
6. store package


package store

import "context"

type Version uint64

type Snapshot[M any] struct {
	Model   M
	Version Version
}

type Store[K comparable, M any] interface {
	Load(context.Context, K) (Snapshot[M], error)

	CompareAndSwap(
		context.Context,
		K,
		Version,
		M,
	) (Snapshot[M], error)

	Delete(context.Context, K) error
}

var (
	ErrNotFound = errors.New("model not found")
	ErrConflict = errors.New("model version conflict")
)
In-memory implementation:



func NewMemory Store[K, M]
Benefits:



No lock during rendering
No lost updates
Replaceable with SQL/Redis/custom storage
Explicit version conflict
7. web package
View contract


package web

type View[Model any] interface {
	Page(context.Context, Model) (html.Page, error)

	Fragment(
		context.Context,
		string,
		Model,
	) (html.Fragment, error)
}
Function adapter:



type ViewFunctions[Model any] struct {
	PageFunc func(
		context.Context,
		Model,
	) (html.Page, error)

	FragmentFunc func(
		context.Context,
		string,
		Model,
	) (html.Fragment, error)
}
Message decoder


type Decoder[Msg mvu.Message] interface {
	Decode(*http.Request) (Msg, error)
}

type DecoderFunc[Msg mvu.Message] func(
	*http.Request,
) (Msg, error)

func (fn DecoderFunc[Msg]) Decode(
	r *http.Request,
) (Msg, error) {
	return fn(r)
}
Model key resolver


type KeyResolver[K comparable] interface {
	Resolve(*http.Request) (K, error)
}

type KeyResolverFunc[K comparable] func(
	*http.Request,
) (K, error)
Typical session key:



KeyResolverFuncfunc(r *http.Request (string, error) {
	return sessionID(r)
})
Endpoint result


type RenderMode uint8

const (
	RenderPage RenderMode = iota + 1
	RenderFragment
	RenderNothing
)

type Result struct {
	Mode       RenderMode
	Fragment   string
	Status     int
	Location   string
	PushURL    string
	ReplaceURL string
}
Route definition


type Route[Msg mvu.Message] struct {
	Method  string
	Pattern string
	Decode  Decoder[Msg]
	Result  Result
}
Example:



web.Route[CounterMessage]{
	Method:  http.MethodPost,
	Pattern: "/counter/increment",

	Decode: web.StaticMessage[CounterMessage]{
		Message: Increment{},
	},

	Result: web.Result{
		Mode:     web.RenderFragment,
		Fragment: "counter",
		Status:   http.StatusOK,
	},
}
Runtime


type Runtime[
	Key comparable,
	Model any,
	Msg mvu.Message,
] struct {
	App       mvu.Application[Model, Msg]
	Store     store.Store[Key, Model]
	Keys      KeyResolver[Key]
	View      View[Model]
	OnError   ErrorHandler
	Commands  CommandRunner[Msg]
	MaxRetries int
}

func NewRuntime[
	Key comparable,
	Model any,
	Msg mvu.Message,
](RuntimeConfig[Key, Model, Msg]) *Runtime[Key, Model, Msg]

func (r *Runtime[Key, Model, Msg]) Handle(
	Route[Msg],
) http.Handler

func (r *Runtime[Key, Model, Msg]) Page(
	func(*http.Request) Result,
) http.Handler
Request lifecycle


1. Resolve model key
2. Load versioned model snapshot
3. Decode typed message
4. Call pure Update
5. Compare-and-swap updated model
6. Retry limited version conflicts
7. Release storage resources
8. Execute commands
9. Select page or named fragment
10. Stream escaped HTML
Page/fragment writer


func WritePage(
	context.Context,
	http.ResponseWriter,
	int,
	html.Page,
) error

func WriteFragment(
	context.Context,
	http.ResponseWriter,
	int,
	html.Fragment,
) error

func NoContent(http.ResponseWriter)
8. Form API
Field errors


package validation

type Errors map[string][]string

func (e Errors) Add(field, message string)
func (e Errors) Has(field string) bool
func (e Errors) First(field string) string
func (e Errors) Empty() bool
Form state


type Form[T any] struct {
	Value  T
	Errors Errors
	Dirty  bool
	Valid  bool
}
Decoder


type Decoder[T any] interface {
	Decode(*http.Request) (T, Errors, error)
}
Prefer explicit decoding, not reflection:



type LoginDecoder struct{}

func (LoginDecoder) Decode(
	r *http.Request,
) (LoginInput, validation.Errors, error) {
	// Parse, normalize, validate.
}
9. Reusable components package
Keep optional. Application-specific components stay in application package.



package components
Layout


type Document struct {
	html.PageMarker

	Lang        string
	Title       string
	Description string
	Stylesheets []string
	HTMXSource  string
	Head        html.Component
	Body        html.Component
}

type Shell struct {
	Header  html.Component
	Sidebar html.Component
	Main    html.Component
	Footer  html.Component
}

type Stack struct {
	Gap      Gap
	Children []html.Component
}

type Grid struct {
	Columns  int
	Gap      Gap
	Children []html.Component
}
Content


type Card struct {
	ID      string
	Header  html.Component
	Body    html.Component
	Footer  html.Component
	Variant CardVariant
}

type Alert struct {
	ID      string
	Kind    AlertKind
	Title   string
	Message string
}

type Badge struct {
	Kind  BadgeKind
	Label string
}

type EmptyState struct {
	Title       string
	Description string
	Action      html.Component
}
Controls


type Button struct {
	Type     ButtonType
	Label    string
	Variant  ButtonVariant
	Disabled bool
	Action   htmx.Action
}

type Link struct {
	Label  string
	URL    string
	Action htmx.Action
}

type Input struct {
	ID           string
	Name         string
	Type         InputType
	Value        string
	Label        string
	Placeholder  string
	Required     bool
	Disabled     bool
	AutoComplete string
	Error        string
}

type Select struct {
	ID       string
	Name     string
	Label    string
	Value    string
	Options  []Option
	Required bool
	Error    string
}

type Option struct {
	Value    string
	Label    string
	Disabled bool
}
Data components


type Table struct {
	ID      string
	Caption string
	Header  html.Component
	Body    html.Component
	Footer  html.Component
}

type Pagination struct {
	Page        int
	PageSize    int
	TotalItems  int
	Window      int
	URL         func(page int) string
	Target      string
	Swap        htmx.Swap
	PushURL     bool
	Previous    string
	Next        string
	AriaLabel   string
}
Function-valued URL is ergonomic but can allocate if a closure captures state. Strict hot path alternative:



type PageURL interface {
	PageURL(page int) string
}
Or avoid generated strings entirely:



type Pagination struct {
	BaseURL string
	Query   string
	// ...
}
Navigation


type Breadcrumb struct {
	Label string
	URL   string
}

type Breadcrumbs struct {
	Items []Breadcrumb
}

type NavItem struct {
	Label    string
	URL      string
	Current  bool
	Disabled bool
	Children []NavItem
}

type Navigation struct {
	Label string
	Items []NavItem
}
10. Complete pagination example


type UsersFragment struct {
	html.FragmentMarker

	Users      []User
	Page       int
	PageSize   int
	TotalItems int
}

func (fragment UsersFragment) RenderHTML(
	ctx context.Context,
	out *html.Context,
) error {
	out.Static(`<section id="users-results">`)
	out.Static(`<table><thead><tr>`)
	out.Static(`<th>Name</th><th>Email</th>`)
	out.Static(`</tr></thead><tbody>`)

	for i := range fragment.Users {
		user := &fragment.Users[i]

		out.Static(`<tr><td>`)
		out.Text(user.Name)
		out.Static(`</td><td>`)
		out.Text(user.Email)
		out.Static(`</td></tr>`)
	}

	out.Static(`</tbody></table>`)

	out.Component(ctx, components.Pagination{
		Page:       fragment.Page,
		PageSize:   fragment.PageSize,
		TotalItems: fragment.TotalItems,
		Window:     2,
		BaseURL:    "/users",
		QueryName:  "page",
		Target:     "#users-results",
		Swap:       htmx.OuterHTML,
		PushURL:    true,
		Previous:   "Previous",
		Next:       "Next",
		AriaLabel:  "Users pagination",
	})

	out.Static(`</section>`)
	return out.Err()
}
Pagination internally renders links rather than buttons:




This preserves normal navigation without HTMX while HTMX enhances fragment updates.

11. Application composition


type CounterModel struct {
	Count int
}

type CounterMessage interface {
	mvu.Message
	isCounterMessage()
}

type Increment struct{}
type Decrement struct{}
type Reset struct{}

func (Increment) MessageName() string { return "counter.increment" }
func (Decrement) MessageName() string { return "counter.decrement" }
func (Reset) MessageName() string     { return "counter.reset" }

func (Increment) isCounterMessage() {}
func (Decrement) isCounterMessage() {}
func (Reset) isCounterMessage()     {}
Application:



type CounterApp struct{}

func (CounterApp) Init(
	context.Context,
) (CounterModel, []mvu.Command[CounterMessage], error) {
	return CounterModel{}, nil, nil
}

func (CounterApp) Update(
	_ context.Context,
	model CounterModel,
	msg CounterMessage,
) (
	CounterModel,
	[]mvu.Command[CounterMessage],
	error,
) {
	switch msg.(type) {
	case Increment:
		model.Count++
	case Decrement:
		model.Count--
	case Reset:
		model.Count = 0
	}

	return model, nil, nil
}
Fragment:



type CounterFragment struct {
	html.FragmentMarker
	Count int
}

func (counter CounterFragment) RenderHTML(
	_ context.Context,
	out *html.Context,
) error {
	out.Static(`<div id="counter">`)
	out.Static(`<button type="button"`)

	htmx.Action{
		Method: htmx.Post,
		URL:    "/counter/decrement",
		Target: "#counter",
		Swap:   htmx.OuterHTML,
	}.RenderAttributes(out)

	out.Static(`>-</button><output>`)
	out.Int(counter.Count)
	out.Static(`</output><button type="button"`)

	htmx.Action{
		Method: htmx.Post,
		URL:    "/counter/increment",
		Target: "#counter",
		Swap:   htmx.OuterHTML,
	}.RenderAttributes(out)

	out.Static(`>+</button></div>`)
	return out.Err()
}
View:



type CounterView struct{}

func (CounterView) Page(
	ctx context.Context,
	model CounterModel,
) (html.Page, error) {
	return components.Document{
		Title: "Counter",
		Body: CounterFragment{
			Count: model.Count,
		},
	}, nil
}

func (CounterView) Fragment(
	ctx context.Context,
	name string,
	model CounterModel,
) (html.Fragment, error) {
	switch name {
	case "counter":
		return CounterFragment{
			Count: model.Count,
		}, nil

	default:
		return nil, web.ErrUnknownFragment
	}
}
12. Design decisions


Component            Only HTML-renderable object
Page                 Complete HTML document
Fragment             Swap-safe HTML subtree
MVU application      State transitions only
View                 Model-to-HTML projection
HTMX action          Typed hypermedia attributes
Route                 HTTP-to-message mapping
Store                 Versioned model persistence
Command               External side effect
Context               Escaped direct HTML writer
Avoid:



Generic Renderer interface
VNode/DOM tree
Runtime html/template
Reflection-based props
Global mutable model
Stringly typed fragment selection everywhere
HTML generation inside Update
HTTP inspection inside Update
Hidden persistence
Component-owned database access
The framework core should remain small. Layouts, tables, pagination, forms, and alerts are a component library built on top, not privileged renderer features.


Reasoned for 1s




Favicon type
Sources
You said:
Add dark and light themes
Copilot said:
Add themes as a first-class framework concept, not CSS files sprinkled throughout.

Theme Layer


MVU
 ↓

Components
 ↓

Theme
 ↓

HTML Renderer
 ↓

HTMX
Components never know colors.

Bad:



Card{
	Class: "bg-gray-900 text-white"
}
Good:



Card{
	Variant: CardPrimary,
}
Theme decides appearance.

Theme API


package theme

type Mode uint8

const (
	Light Mode = iota
	Dark
)
Theme contract:



type Theme interface {
	Name() string

	DocumentClass(mode Mode) string

	Card(CardVariant, Mode) string
	Button(ButtonVariant, Mode) string
	Input(InputVariant, Mode) string
	Table(TableVariant, Mode) string

	Variables(Mode) string
}
Context
Renderer always carries theme.



type Context struct {
	writer io.Writer
	err    error

	Theme Theme
	Mode  theme.Mode
}
Component:



func (c Card) RenderHTML(
	ctx context.Context,
	out *html.Context,
) error {

	out.Static(`<article class="`)
	out.Static(
		out.Theme.Card(
			c.Variant,
			out.Mode,
		),
	)
	out.Static(`">`)

	// render body

	out.Static(`</article>`)

	return out.Err()
}
Component knows:



CardPrimary
Theme knows:



bg colors
border colors
padding
radius
Light Theme


type ModernTheme struct{}


func (ModernTheme) Card(
	v CardVariant,
	mode Mode,
) string {

	if mode == Dark {

		return "card card-dark"
	}

	return "card card-light"
}
CSS Variables
Framework emits:




or:




CSS:



:root {

    --bg: #ffffff;
    --fg: #111111;

    --card-bg: #f8f8f8;
    --card-border: #e5e5e5;

    --primary: #2b6fff;
}
Dark:



html[data-theme="dark"] {

    --bg: #0d1117;
    --fg: #f3f4f6;

    --card-bg: #161b22;
    --card-border: #2d333b;

    --primary: #4f8cff;
}
Card:



.card {

    background: var(--card-bg);

    color: var(--fg);

    border: 1px solid var(--card-border);
}
Every component automatically adapts.

Theme Switcher Component


type ThemeSwitcher struct{}


func (t ThemeSwitcher) RenderHTML(
	ctx context.Context,
	out *html.Context,
) error {

	out.Static(`
<form
 hx-post="/theme/toggle"
 hx-swap="none">
`)

	if out.Mode == theme.Dark {

		out.Static(`
<button>
☀ Light
</button>
`)
	} else {

		out.Static(`
<button>
🌙 Dark
</button>
`)
	}

	out.Static(`</form>`)

	return out.Err()
}
Theme Session


type Session struct {
	Theme theme.Mode
}
Handler:



func ToggleTheme(
	w http.ResponseWriter,
	r *http.Request,
) {

	session.Theme =
		theme.Dark

	if session.Theme ==
		theme.Dark {

		session.Theme =
			theme.Light
	}

	htmx.Refresh(w)
}
Browser reloads page.

New theme applied.

No JavaScript.

Document


type Document struct {
	Title string
	Body  Component
}
Render:



func (d Document) RenderHTML(
	ctx context.Context,
	out *html.Context,
) error {

	out.Static(`
<!doctype html>
<html data-theme="`)


if out.Mode == theme.Dark {
	out.Static("dark")
} else {
	out.Static("light")
}


out.Static(`">`)
Theme now affects entire application.

Component Variants
Card:



type CardVariant uint8

const (
	CardDefault CardVariant = iota
	CardPrimary
	CardSuccess
	CardWarning
	CardDanger
)
Button:



type ButtonVariant uint8

const (
	ButtonPrimary ButtonVariant = iota
	ButtonSecondary
	ButtonDanger
	ButtonGhost
)
Framework theme maps:



Component variant
        ↓
Theme token
        ↓
CSS variable
        ↓
Actual color
Final Framework Architecture


Model
 ↓

Update
 ↓

View Components
 ↓

Theme System
 ↓

HTML Renderer
 ↓

HTMX
 ↓

Browser
Framework packages:



mvu/
html/
htmx/
web/
store/
theme/
components/
Theme package owns:



Light
Dark
Colors
Typography
Spacing
Radius
Shadows
Component styles
Components own:



Structure
Semantics
HTMX behavior
Accessibility
Renderer owns:



Escaping
Streaming
HTML generation
This keeps dark/light mode completely orthogonal to components and scales cleanly to custom themes later:



theme.Default
theme.Dark
theme.Modern
theme.Minimal
theme.Corporate
theme.MyBrand
without changing a single component.





You said:
Create Ai prompt
Copilot said:
Use this prompt with a coding agent. It defines scope, borders, components, MVU, HTMX, pagination, and server-controlled light/dark themes.



Act as a senior Go framework architect and implementation engineer.

Build a complete, production-oriented Go framework named "gomvu" for server-rendered HTML applications using MVU architecture and HTMX.

Do not ask for confirmation between stages. Implement, test, inspect failures, fix them, and finish the repository.

# 1. Product goal

Create a generic but deliberately narrow framework with clear architectural borders:

- Go
- net/http
- Server-side rendering
- HTML output only
- HTMX for progressive enhancement
- MVU state management
- Nested reusable components
- Full-page rendering
- HTML fragment rendering
- Light and dark themes
- No React
- No JSX
- No SPA
- No hydration
- No virtual DOM
- No JSON UI protocol
- No client-side application state
- No custom application JavaScript
- No runtime html/template dependency
- No reflection-based rendering
- No generic renderer for Markdown, PDF, terminal, native UI, or other targets

HTMX may be included as a browser dependency. Application behavior, validation, state transitions, persistence, and HTML rendering must remain on the Go server.

The framework must render HTML directly to io.Writer.

# 2. Architecture

Use these packages:

gomvu/
├── mvu/
├── html/
├── htmx/
├── web/
├── store/
├── session/
├── validation/
├── theme/
├── components/
├── middleware/
├── examples/
│   ├── counter/
│   └── users/
├── static/
├── go.mod
└── README.md

Enforce this dependency direction:

components -> html, htmx, theme
web        -> mvu, html, htmx, store, theme
session    -> store
htmx       -> html
theme      -> standard library only
mvu        -> standard library only
html       -> standard library only
store      -> standard library only

Forbidden dependencies:

- mvu must not import html, htmx, web, components, theme, or store
- html must not import mvu, htmx, web, components, or store
- theme must not import components or web

Reasoned for 1s
