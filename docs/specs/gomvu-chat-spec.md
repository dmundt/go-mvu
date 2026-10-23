<!--
Source: "Go MVU Framework for SSR" Copilot chat, pasted by user as the gomvu
build prompt. Shared chat link requires Microsoft sign-in; this text is the
persisted copy. Verbatim below. Deviations: docs/specs/README.md.
-->
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
├── timing/
├── examples/
│   ├── counter/
│   └── users/
├── static/
├── go.mod
└── README.md

Enforce this dependency direction:

components -> html, htmx, theme
web        -> mvu, html, htmx, store, theme, timing
middleware -> html, session, timing
session    -> store
htmx       -> html
theme      -> standard library only
mvu        -> standard library only
html       -> standard library only
store      -> standard library only
timing     -> standard library only

Forbidden dependencies:

- mvu must not import html, htmx, web, components, theme, or store
- html must not import mvu, htmx, web, components, or store
- theme must not import components or web
- store must not import HTML or HTTP packages
- Update must not inspect HTTP requests
- Update must not render HTML
- Components must not access databases
- Rendering must not mutate the model
- HTTP handlers must not contain component markup

# 3. Rendering model

Do not use a VNode tree in the runtime hot path.

Do not build intermediate Element, Node, or attribute trees before rendering.

Use direct component rendering:

type Component interface {
    RenderHTML(context.Context, *Context) error
}

Add:

type ComponentFunc func(context.Context, *Context) error

func (fn ComponentFunc) RenderHTML(
    ctx context.Context,
    out *Context,
) error

Define separate marker interfaces:

type Page interface {
    Component
    IsPage()
}

type Fragment interface {
    Component
    IsFragment()
}

Provide reusable markers:

type PageMarker struct{}
func (PageMarker) IsPage()

type FragmentMarker struct{}
func (FragmentMarker) IsFragment()

A Page represents a complete HTML document.

A Fragment represents a stable HTML subtree suitable for HTMX replacement.

Page and Fragment must remain distinct to prevent accidental use of complete documents as HTMX fragment responses.

# 4. HTML rendering context

Implement:

type Context struct {
    // private fields
}

func NewContext(io.Writer, theme.Theme, theme.Mode) *Context

func Render(
    context.Context,
    io.Writer,
    Component,
    theme.Theme,
    theme.Mode,
) error

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

func (c *Context) Theme() theme.Theme
func (c *Context) ThemeMode() theme.Mode

Requirements:

- Sticky first-error behavior
- Direct streaming to io.Writer
- Correct HTML text escaping
- Correct HTML attribute escaping
- No allocation merely to escape ordinary strings
- Numeric output using stack scratch buffers and strconv.Append functions
- No unsafe package
- No reflection
- No global mutable rendering state
- Preserve component output order
- Check context cancellation between large repeated component operations
- Never continue writing after the first output error

Do not expose Raw(string).

If trusted output is required, use an explicit type:

type TrustedHTML string

func TrustStatic(string) TrustedHTML
func (c *Context) Trusted(TrustedHTML)

Document that TrustStatic accepts only audited, server-controlled HTML. It must never receive request, form, database, URL, or user-provided content.

## 4.1 Element builder and attribute allowlist

Low-level code must not need hand-written tag strings. Provide a typed,
escaping element builder:

type Attr struct {
    Name  string
    Value string
    Flag  bool // boolean attribute: name only
}

type Attrs []Attr // ordered; output order is deterministic

func A(name, value string) Attr
func Flag(name string) Attr

type Text string // escaped text component

type Element struct {
    Tag      string
    Attrs    Attrs
    Children []Component
}

func El(tag string, attrs Attrs, children ...Component) Element

func (c *Context) Attrs(Attrs)                    // all attributes
func (c *Context) Class(base string, attrs Attrs) // merged class attribute
func (c *Context) ExtraClass(attrs Attrs)         // inside an open class
func (c *Context) PassAttrs(attrs Attrs, owned ...string)

func ValidateTag(string) error
func ValidateAttr(Attr) error
func ValidateURL(string) error

Usage:

html.El("section", html.Attrs{html.A("class", "panel")},
    html.El("h2", nil, html.Text("Title")),
    components.Button{Label: "Save"},
)

Requirements:

- Values are escaped with the attribute escaper; text with the text escaper.
- Tags match `[a-z][a-z0-9-]*`. Reject `script`, `style`, `link`, `meta`,
  `base`, `iframe`, `frame`, `frameset`, `object`, `embed`, `applet`,
  `noscript`, `template`, `html`, `head`, `body` with `ErrInvalidElement`.
- Void elements (`area`, `br`, `col`, `hr`, `img`, `input`, `source`,
  `track`, `wbr`) render no end tag; children are an `ErrInvalidElement`.
- Attribute names match `[A-Za-z_:][A-Za-z0-9_.:-]*`.
- Reject with `ErrUnsafeAttribute` (sticky, nothing of the attribute is
  written): `on*` handlers, `style`, `srcdoc`, `hx-on*`, `hx-vars` (and their
  `data-hx-*` forms), `js:`/`javascript:` values in `hx-vals`/`hx-headers`,
  and `[...]` filters in `hx-trigger`.
- URL attributes (`href`, `src`, `action`, `formaction`, `poster`, `cite`,
  `data`, `background`, `longdesc`, `manifest`, `ping`, `xlink:href`,
  `hx-get|post|put|patch|delete`, `hx-push-url`, `hx-replace-url` other than
  `true`/`false`) pass `ValidateURL`: relative, `http`, `https`, `mailto` or
  `tel`; no control characters or backslashes.
- `htmx.Action.Attrs() (html.Attrs, error)` returns the same attributes as
  `RenderAttributes` for use with `El`.
- No maps, no reflection; no allocation for the attribute path itself.

# 5. Component composition

Components compose through ordinary Go structs and explicit slots.

Example:

type Card struct {
    ID      string
    Header  html.Component
    Body    html.Component
    Footer  html.Component
    Variant CardVariant
}

Usage:

Card{
    ID: "counter-card",
    Header: Heading{
        Level: 2,
        Text: "Counter",
    },
    Body: CounterFragment{
        Count: model.Count,
    },
    Footer: Text{
        Value: "State maintained by Go",
    },
}

Nested render chain:

Document
└── AppShell
    ├── Header
    │   └── ThemeSwitcher
    ├── Sidebar
    │   └── Navigation
    ├── Main
    │   ├── Card
    │   │   └── CounterFragment
    │   └── UsersFragment
    │       ├── Table
    │       └── Pagination
    └── Footer

Do not use hidden component registries.

Do not use string names to locate component types.

Use ordinary Go composition, interfaces, and concrete structs.

Provide:

type Empty struct{}
type Text struct { Value string }
type Sequence []Component
type Optional struct {
    Show bool
    Body Component
}

Document that `Sequence` and interface-valued slots prioritize flexibility. Performance-critical components may use concrete child types and direct method calls.

High-level components and low-level constructs mix freely: an `html.El`
element accepts components as children, and every component slot accepts
an `html.El` element or an `html.ComponentFunc`.

# 6. MVU package

Implement a pure MVU core independent of HTML and HTTP.

type Message interface {
    MessageName() string
}

type Dispatch[M Message] func(context.Context, M) error

type Command[M Message] interface {
    Execute(context.Context, Dispatch[M]) error
}

type CommandFunc[M Message] func(
    context.Context,
    Dispatch[M],
) error

type Application[Model any, Msg Message] interface {
    Init(context.Context) (
        Model,
        []Command[Msg],
        error,
    )

    Update(
        context.Context,
        Model,
        Msg,
    ) (
        Model,
        []Command[Msg],
        error,
    )
}

Provide function adapters:

type Functions[Model any, Msg Message] struct {
    InitFunc   ...
    UpdateFunc ...
}

The framework must treat Update as logically pure:

- Input model plus typed message
- Output new model plus commands
- No HTML rendering
- No HTTP inspection
- No persistence
- No hidden dispatch
- No global mutation

Commands represent external effects and may asynchronously dispatch additional messages.

# 7. Store package

Implement versioned persistence:

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

    Create(
        context.Context,
        K,
        M,
    ) (Snapshot[M], error)

    Delete(context.Context, K) error
}

Provide errors:

var ErrNotFound error
var ErrAlreadyExists error
var ErrConflict error

Implement an in-memory store:

func NewMemory Store[K, M]

Requirements:

- Race-safe
- Versioned updates
- Never hold locks during rendering
- Never hold locks during command execution
- Copying policy documented
- Models containing maps, slices, or pointers require application-level snapshot discipline unless a clone function is configured

Optionally support:

func NewMemoryWithClone[
    K comparable,
    M any,
](clone func(M) M) Store[K, M]

# 8. Web views

Keep MVU transition logic separate from HTML projection.

Implement:

type View[Model any] interface {
    Page(
        context.Context,
        Model,
        RequestContext,
    ) (html.Page, error)

    Fragment(
        context.Context,
        string,
        Model,
        RequestContext,
    ) (html.Fragment, error)
}

RequestContext may contain only presentation-related request information:

type RequestContext struct {
    Theme     theme.Mode
    IsHTMX    bool
    CurrentURL string
}

It must not expose response writers or persistence.

Provide ViewFunctions function adapter.

# 9. HTMX package

Implement typed HTMX configuration.

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

type Action struct {
    Method      Method
    URL         string
    Target      string
    Swap        Swap
    Trigger     string
    Select      string
    Include     string
    Indicator   string
    Confirm     string
    PushURL     string
    ReplaceURL  string
    DisabledElt string
    Boost       bool
}

func (a Action) Validate() error
func (a Action) RenderAttributes(*html.Context)

All dynamic attribute values must use Context.Attribute.

Do not permit dynamic attribute names.

Implement HTMX request metadata:

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

func FromRequest(*http.Request) Request

Implement response helpers:

func Redirect(http.ResponseWriter, string)
func Refresh(http.ResponseWriter)
func PushURL(http.ResponseWriter, string)
func ReplaceURL(http.ResponseWriter, string)
func Retarget(http.ResponseWriter, string)
func Reswap(http.ResponseWriter, Swap)
func Reselect(http.ResponseWriter, string)
func Trigger(http.ResponseWriter, string)

# 10. HTTP runtime

Implement:

type Decoder[Msg mvu.Message] interface {
    Decode(*http.Request) (Msg, error)
}

type DecoderFunc[Msg mvu.Message] func(
    *http.Request,
) (Msg, error)

type StaticMessage[Msg mvu.Message] struct {
    Message Msg
}

type KeyResolver[K comparable] interface {
    Resolve(*http.Request) (K, error)
}

type KeyResolverFunc[K comparable] func(
    *http.Request,
) (K, error)

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

type Route[Msg mvu.Message] struct {
    Method  string
    Pattern string
    Decode  Decoder[Msg]
    Result  Result
}

type Runtime[
    Key comparable,
    Model any,
    Msg mvu.Message,
] struct {
    // private fields
}

type RuntimeConfig[
    Key comparable,
    Model any,
    Msg mvu.Message,
] struct {
    App        mvu.Application[Model, Msg]
    Store      store.Store[Key, Model]
    Keys       KeyResolver[Key]
    View       View[Model]
    Theme      theme.Theme
    Themes     ThemeResolver
    Commands   CommandRunner[Msg]
    OnError    ErrorHandler
    MaxRetries int
}

func NewRuntimeRuntimeConfig[...],
 (*Runtime[...], error)

func (r *Runtime[...]) Handle(
    Route[Msg],
) http.Handler

func (r *Runtime[...]) Page(
    result Result,
) http.Handler

Request lifecycle:

1. Resolve model key.
2. Load versioned model.
3. Initialize model if missing when allowed by route.
4. Decode typed message.
5. Call Update.
6. Compare-and-swap updated model.
7. Retry bounded version conflicts.
8. Stop using store locks.
9. Execute commands.
10. Resolve theme mode.
11. Select full Page or named Fragment.
12. Stream escaped HTML response.
13. Return correct status and headers.

Never render while holding a store lock.

Never execute commands while holding a store lock.

If rendering fails after response output begins, log the failure but do not attempt to replace the partially written response with `http.Error`.

# 11. Theme system

Implement server-controlled light and dark modes.

type Mode uint8

const (
    System Mode = iota
    Light
    Dark
)

func (m Mode) String() string
func ParseMode(string) (Mode, error)

System mode must use CSS media queries, not client-side JavaScript.

Implement semantic theme tokens:

type Theme interface {
    Name() string
    Stylesheet() string
    DocumentClass(Mode) string
    ComponentClass(ComponentKind, Variant, Mode) string
}

However, avoid expensive dynamic theme lookup in every component where CSS variables can solve the problem.

Preferred strategy:

- Components emit stable semantic classes
- Document emits `data-theme="light"`, `data-theme="dark"`, or `data-theme="system"`
- CSS custom properties define colors and visual tokens
- Component CSS consumes custom properties
- System mode uses `@media (prefers-color-scheme: dark)`
- Components never embed literal theme colors
- Components may expose semantic variants such as primary, secondary, success, warning, danger, ghost

Create a default theme stylesheet containing tokens:

--color-background
--color-surface
--color-surface-raised
--color-text
--color-text-muted
--color-border
--color-primary
--color-primary-text
--color-success
--color-warning
--color-danger
--focus-ring
--shadow-small
--shadow-medium
--radius-small
--radius-medium
--space-1 through --space-8

Provide:

type Default struct{}

func (Default) Name() string
func (Default) Stylesheet() string
func (Default) DocumentClass(Mode) string
func (Default) ComponentClass(
    ComponentKind,
    Variant,
    Mode,
) string

Theme mode resolution:

type ThemeResolver interface {
    Resolve(*http.Request) (theme.Mode, error)
    Persist(
        http.ResponseWriter,
        *http.Request,
        theme.Mode,
    ) error
}

Provide cookie-based implementation:

type CookieThemeResolver struct {
    Name     string
    Path     string
    MaxAge   int
    Secure   bool
    SameSite http.SameSite
}

Cookie values:

- system
- light
- dark

Use HttpOnly when appropriate, Secure configuration, SameSite, and explicit path.

# 12. Theme switcher

Implement an SSR and HTMX-compatible ThemeSwitcher component.

type ThemeSwitcher struct {
    Mode   theme.Mode
    Action string
    Target string
}

It must render a normal HTML form that works without HTMX:

<form method="post" action="/theme">
    ...
</form>

Enhance the same form with HTMX when available.

Do not require custom JavaScript.

Support three explicit choices:

- System
- Light
- Dark

Theme route behavior:

POST /theme

Input:

mode=system
mode=light
mode=dark

Requirements:

- Validate mode
- Persist cookie
- Redirect back to a validated local return URL for ordinary form requests
- For HTMX requests, return an appropriate response that refreshes or replaces the complete themed application shell
- Prevent open redirects
- Include CSRF protection
- Preserve accessibility
- Use button labels, not icon-only controls
- Mark current choice with aria-current or equivalent semantic state

# 13. Reusable component library

Implement these components:

Layout:

- Document
- AppShell
- Header
- Sidebar
- Main
- Footer
- Stack
- Inline
- Grid

Content:

- Heading
- Paragraph
- Card
- Alert
- Badge
- EmptyState

Navigation:

- Navigation
- NavigationItem
- Breadcrumbs
- BreadcrumbItem
- Pagination

Forms:

- Form
- Label
- Input
- Textarea
- Select
- Option
- Checkbox
- FieldError
- Button

Data:

- Table
- TableHeader
- TableBody
- TableRow
- TableCell

Each component must:

- Implement html.Component
- Use semantic HTML
- Escape dynamic text
- Escape dynamic attributes
- Support accessible labels
- Emit stable CSS classes
- Use semantic variants
- Work in light and dark themes
- Avoid database access
- Avoid mutating models
- Avoid arbitrary custom markup arguments where typed fields suffice
- Expose `Attrs html.Attrs` for attribute passthrough (see §13.1)

## 13.1 Attribute passthrough

Every high-level component in `components` and `shared` has an
`Attrs html.Attrs` field. It adds `id`, `class`, `aria-*`, `data-*`,
`hx-*` and other allowlisted attributes (§4.1) without dropping to the
low level:

components.Card{
    Body:  body,
    Attrs: html.Attrs{html.A("class", "wide"), html.A("hx-get", "/stats")},
}

Rules:

- Attrs apply to the component root element. Form controls (`Input`,
  `Select`, `Checkbox`, `Textarea`, `Switch`) apply them to the control
  itself; `Dialog` to the popover panel; `Table` to `<table>`.
- `class` values merge into the component class list; they never replace
  component classes.
- Component-owned attributes win. Names the component writes (for example
  `type` on `Button`, `href` on `Link`, `role` on `Alert`, `id` when the `ID`
  field is set) are skipped from passthrough. When an `htmx.Action` is set,
  the component owns every `hx-*` name.
- Unsafe attributes fail rendering with `html.ErrUnsafeAttribute`.
- Empty Attrs cost nothing measurable on the render hot path.

# 14. Pagination component

Implement production-quality pagination.

type Pagination struct {
    Page       int
    PageSize   int
    TotalItems int
    Window     int

    BaseURL    string
    QueryName  string
    Target     string
    Swap       htmx.Swap
    PushURL    bool

    PreviousLabel string
    NextLabel     string
    AriaLabel     string
}

Requirements:

- Calculate total pages safely
- Handle zero items
- Clamp invalid page values
- Previous and next links
- First and last page visibility
- Configurable page window
- Ellipsis where page ranges are omitted
- aria-current="page"
- Accessible navigation label
- Normal href navigation without HTMX
- HTMX enhancement using hx-get
- Stable target fragment
- OuterHTML replacement option
- Optional browser-history update
- URL query escaping with net/url
- Preserve non-page query parameters when supplied
- No fmt.Sprintf in rendering hot path
- No closure required to generate URLs

Example output:

<a
    href="/users?page=3"
    hx-get="/users?page=3"
    hx-target="#users-results"
    hx-swap="outerHTML"
    hx-push-url="true"
    aria-current="page">
    3
</a>

# 15. Counter example

Build a complete counter application.

Model:

type CounterModel struct {
    Count int
}

Messages:

- Increment
- Decrement
- Reset

Routes:

GET  /
POST /counter/increment
POST /counter/decrement
POST /counter/reset
POST /theme

GET / returns a complete themed Document.

Counter routes return only:

<div id="counter">...</div>

Counter buttons use:

hx-post
hx-target="#counter"
hx-swap="outerHTML"

The page must contain:

Document
└── AppShell
    ├── Header
    │   └── ThemeSwitcher
    ├── Sidebar
    │   └── Navigation
    ├── Main
    │   └── Card
    │       └── CounterFragment
    └── Footer

# 16. Users and pagination example

Build a users application demonstrating pagination.

Model:

type UsersModel struct {
    Users      []User
    Page       int
    PageSize   int
    TotalItems int
}

Fragment root:

<section id="users-results">
    <table>...</table>
    <nav>...</nav>
</section>

Routes:

GET /users
GET /users?page=N
POST /theme

Ordinary request returns complete Document.

HTMX pagination request returns only UsersFragment.

UsersFragment contains:

- Semantic table
- Empty state when no users exist
- Pagination
- Stable root ID
- Light/dark styling
- Accessible navigation

Use deterministic sample users. Clearly document that sample data is synthetic.

# 17. Validation package

Implement:

type Errors map[string][]string

func (e Errors) Add(field, message string)
func (e Errors) Has(field string) bool
func (e Errors) First(field string) string
func (e Errors) Empty() bool

type Form[T any] struct {
    Value  T
    Errors Errors
    Dirty  bool
    Valid  bool
}

Do not build reflection-based automatic binding.

Use explicit decoders:

type Decoder[T any] interface {
    Decode(*http.Request) (T, Errors, error)
}

# 18. Middleware

Implement:

- Panic recovery
- Request logging using log/slog
- Security headers
- CSRF protection
- Request ID
- Rendering time measurement and diagnostics
- Optional response compression only if it does not compromise streaming correctness

Security headers should include documented defaults for:

- Content-Security-Policy
- X-Content-Type-Options
- Referrer-Policy
- Permissions-Policy
- Frame protection via CSP frame-ancestors

Do not use unsafe-inline for script policy unless explicitly documented as an opt-in compromise.

Prefer serving HTMX locally from `/static/htmx.min.js`.

Do not depend on a production CDN.

## 18.1 Rendering time measurement and diagnostics

Package `timing` (standard library only) carries a per-request `Trace` in
the request context. All methods are nil-safe: without a trace, instrumented
code costs one context lookup and no allocation.

The web runtime records these phases:

| Phase | Measures |
| --- | --- |
| `decode` | `Decoder.Decode` |
| `dispatch` | Update, store compare-and-swap, synchronous commands |
| `load` | store load, Init for missing models |
| `view` | `View.Page` / `View.Fragment` (component tree construction) |
| `render` | HTML streaming (`html.Render` into the response) |

Repeated phases are summed. Applications may record their own phases with
`timing.FromContext(ctx).Start("name")`.

`middleware.NewDiagnostics(DiagnosticsOptions)` returns a collector with:

- `Middleware`: attaches a trace; measures time to first byte (`ttfb`)
  and `total`; records status and bytes.
- Per-request `log/slog` record `request diagnostics`: Debug level; Warn
  when `SlowThreshold` is exceeded. Includes request ID and phases.
- In-memory aggregates per phase: count, total, mean, max; request, slow,
  and 5xx counters. `Snapshot`, `Reset`.
- `Handler`: `GET`/`HEAD` JSON snapshot, `Cache-Control: no-store`.
  Mount only on internal or authenticated routes.
- Optional `ServerTiming` (default off): `Server-Timing` header with phases
  finished before the first byte plus `ttfb`; `Server-Timing` trailer with
  later phases (`render`) plus `total`. Rendering streams, so render time
  cannot be a header. Metric names sanitized to HTTP token characters.

Rules:

- Measurement must not buffer or delay streaming output.
- The runtime must not import `middleware`; both depend on `timing`.
- Diagnostics are opt-in; Server-Timing exposes server timings to clients and
  is for development or trusted networks.
- Place `Diagnostics.Middleware` inside `RequestID`.
# 19. CSS

Create a complete stylesheet.

Requirements:

- Light theme
- Dark theme
- System theme
- CSS variables
- Responsive shell
- Accessible focus states
- Minimum useful contrast
- Reduced-motion support
- Pagination states
- Disabled states
- Current-page state
- Table responsiveness
- Form validation states
- Card variants
- Alert variants
- Theme-switcher styling

Modes:

html[data-theme="light"] {
    color-scheme: light;
}

html[data-theme="dark"] {
    color-scheme: dark;
}

html[data-theme="system"] {
    color-scheme: light dark;
}

System theme must use:

@media (prefers-color-scheme: dark) {
    html[data-theme="system"] {
        ...
    }
}

No JavaScript theme initialization.

# 20. Testing requirements

HTML tests:

- Text escaping
- Attribute escaping
- Trusted HTML isolation
- First-error handling
- Nested components
- Component sequence
- Conditional component
- Context cancellation
- Numeric rendering

HTMX tests:

- Every HTTP method
- Swap validation
- Escaped action attributes
- Invalid target handling
- Header parsing
- Response helpers

MVU tests:

- Initialization
- Pure state transitions
- Commands
- Context cancellation
- Error propagation

Store tests:

- Create
- Load
- Compare-and-swap
- Conflict
- Delete
- Concurrent access
- Clone behavior

Web tests:

- Full-page response
- Fragment response
- HTMX request detection
- Correct content type
- Correct status
- Version-conflict retry
- Unknown fragment
- Rendering failure
- Command execution outside store lock

Theme tests:

- Mode parsing
- Cookie persistence
- Light mode
- Dark mode
- System mode
- Invalid mode rejection
- Secure cookie configuration
- Local return URL validation
- Theme form without HTMX
- Theme action with HTMX

Pagination tests:

- Zero items
- One page
- First page
- Middle page
- Last page
- Invalid negative page
- Page beyond final page
- Window calculation
- Ellipsis placement
- URL encoding
- Existing query preservation
- Accessible current-page output
- Normal href plus HTMX attributes

Security tests:

- Script injection attempts
- Attribute injection attempts
- Open redirect attempts
- CSRF failure
- Invalid dynamic URL handling
- CSP headers

Run:

gofmt
go vet ./...
go test ./...
go test -race ./...
go test -bench=. -benchmem ./...

Fix all failures before completion.

# 21. Benchmark requirements

Add benchmarks for:

- Static component
- Dynamic escaped text
- Counter fragment
- Nested dashboard
- Pagination with 10 pages
- Pagination with 1,000 logical pages using a fixed window
- Direct rendering versus html/template reference implementation

Do not invent benchmark results.

Run benchmarks and report actual observed output.

Do not claim the complete HTTP request has zero allocations.

State performance claims precisely:

- The rendering API is designed to minimize framework allocations.
- Specific components may achieve zero allocations with a suitable writer.
- io.Writer implementations, HTTP internals, interfaces, model construction, URLs, and dynamic data may allocate.
- Benchmark evidence determines actual behavior.

# 22. Documentation

README must include:

- Product purpose
- Non-goals
- Package boundaries
- Dependency rules
- MVU lifecycle
- Full page versus fragment rendering
- Component authoring
- Slot composition
- HTMX enhancement
- No-HTMX fallback
- Light/dark/system themes
- Theme cookies
- Pagination
- Store concurrency model
- Command execution
- Rendering safety
- CSRF
- CSP
- Deployment
- HTMX local asset serving
- Benchmark interpretation
- Known limitations
- Extension guidelines

Include diagrams:

Request lifecycle:

Browser
  -> HTTP route
  -> typed message
  -> Update
  -> versioned store
  -> View
  -> Page or Fragment
  -> direct HTML stream
  -> HTMX swap

Package boundaries:

mvu       state transitions
store     model persistence
html      safe rendering
htmx      hypermedia attributes
theme     semantic visual tokens
components reusable HTML structures
web       HTTP orchestration

# 23. Quality rules

- Standard library preferred
- Minimize dependencies
- No pseudocode
- No TODO placeholders
- No omitted implementations
- No generated fake output
- No invented test results
- No invented benchmark results
- No panics for ordinary request errors
- No mutable global application model
- No runtime HTML template parsing
- No reflection in renderer
- No unsafe package
- No VNode runtime
- No hidden service locator
- No component database access
- No application JavaScript
- No inline event handlers
- No arbitrary user-controlled raw HTML
- No rendering under storage locks
- No commands under storage locks

# 24. Implementation sequence

Implement without pausing for confirmation:

1. Repository skeleton
2. HTML context and escaping
3. Theme contracts and default CSS
4. HTMX actions
5. MVU contracts
6. Versioned memory store
7. Web runtime
8. Component library
9. Theme switcher
10. Pagination
11. Counter example
12. Users example
13. Middleware and security
14. Tests
15. Benchmarks
16. Documentation
17. Final validation

After each stage, compile and fix errors before continuing.

# 25. Final response

Return:

1. Repository tree
2. Concise architecture summary
3. Complete content of every created source file
4. Validation commands run
5. Actual test results
6. Actual race-test results
7. Actual benchmark results
8. Important tradeoffs
9. Known limitations

If response length prevents showing every file, create the complete files in the working directory and provide the repository as a downloadable archive. Do not replace files with shortened snippets.