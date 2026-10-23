package components

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/htmx"
	"github.com/dmundt/go-mvu/theme"
)

type Empty struct{}

func (Empty) RenderHTML(context.Context, *html.Context) error { return nil }

type Text string

func (value Text) RenderHTML(_ context.Context, out *html.Context) error {
	out.Text(string(value))
	return out.Err()
}

type Sequence []html.Component

func (items Sequence) RenderHTML(ctx context.Context, out *html.Context) error {
	out.Components(ctx, items)
	return out.Err()
}

// Paragraph renders escaped text in a <p> element.
type Paragraph string

func (value Paragraph) RenderHTML(_ context.Context, out *html.Context) error {
	out.Static("<p>")
	out.Text(string(value))
	out.Static("</p>")
	return out.Err()
}

// Heading renders <h1> through <h6>. Level must be 1–6.
type Heading struct {
	Level int
	ID    string
	Text  string
}

func (heading Heading) RenderHTML(_ context.Context, out *html.Context) error {
	if heading.Level < 1 || heading.Level > 6 {
		return fmt.Errorf("invalid heading level %d", heading.Level)
	}
	tag := "h" + strconv.Itoa(heading.Level)
	out.Static("<" + tag)
	optionalAttribute(out, "id", heading.ID)
	out.Static(">")
	out.Text(heading.Text)
	out.Static("</" + tag + ">")
	return out.Err()
}

// Output renders a computed value. Live announces changes to assistive
// technology, e.g. after an HTMX swap.
type Output struct {
	ID    string
	Value string
	Live  bool
}

func (output Output) RenderHTML(_ context.Context, out *html.Context) error {
	out.Static("<output")
	optionalAttribute(out, "id", output.ID)
	if output.Live {
		out.Static(` aria-live="polite"`)
	}
	out.Static(">")
	out.Text(output.Value)
	out.Static("</output>")
	return out.Err()
}

func optionalAttribute(out *html.Context, name, value string) {
	if value == "" {
		return
	}
	out.Static(" " + name + `="`)
	out.Attribute(value)
	out.Static(`"`)
}

type Document struct {
	html.PageMarker
	Lang        string
	Title       string
	Description string
	HTMXSource  string
	// HTMXIntegrity is an SRI hash such as static.HTMXIntegrity.
	HTMXIntegrity string
	Head          html.Component
	Body          html.Component
	Theme         theme.Theme
	Mode          theme.Mode
}

func (document Document) RenderHTML(ctx context.Context, out *html.Context) error {
	language := document.Lang
	if language == "" {
		language = "en"
	}
	selectedTheme := document.Theme
	if selectedTheme == nil {
		selectedTheme = theme.Default
	}
	out.Static("<!doctype html><html lang=\"")
	out.Attribute(language)
	out.Static("\" data-theme=\"")
	out.Static(document.Mode.String())
	out.Static("\" data-theme-name=\"")
	out.Attribute(selectedTheme.Name())
	if class := theme.DocumentClass(selectedTheme, document.Mode); class != "" {
		out.Static("\" class=\"")
		out.Attribute(class)
	}
	out.Static("\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">")
	out.Static("<title>")
	out.Text(document.Title)
	out.Static("</title>")
	if document.Description != "" {
		out.Static("<meta name=\"description\" content=\"")
		out.Attribute(document.Description)
		out.Static("\">")
	}
	out.Static("<style>")
	out.Static(selectedTheme.Stylesheet())
	out.Static("</style>")
	out.Component(ctx, document.Head)
	if document.HTMXSource != "" {
		if err := validateURL(document.HTMXSource); err != nil {
			return fmt.Errorf("invalid HTMX source URL: %w", err)
		}
		out.Static("<script src=\"")
		out.Attribute(document.HTMXSource)
		out.Static("\"")
		optionalAttribute(out, "integrity", document.HTMXIntegrity)
		out.Static(" defer></script>")
	}
	out.Static("</head><body>")
	out.Component(ctx, document.Body)
	out.Static("</body></html>")
	return out.Err()
}

// AppShell is the spec name for Shell.
type AppShell = Shell

// Shell lays out header, optional sidebar, main and footer landmarks.
type Shell struct {
	Header  html.Component
	Sidebar html.Component
	Main    html.Component
	Footer  html.Component
}

func (shell Shell) RenderHTML(ctx context.Context, out *html.Context) error {
	if shell.Header != nil {
		out.Static("<header>")
		out.Component(ctx, shell.Header)
		out.Static("</header>")
	}
	out.Static("<div class=\"shell\">")
	if shell.Sidebar != nil {
		out.Static("<aside>")
		out.Component(ctx, shell.Sidebar)
		out.Static("</aside>")
	}
	out.Static("<main>")
	out.Component(ctx, shell.Main)
	out.Static("</main></div>")
	if shell.Footer != nil {
		out.Static("<footer>")
		out.Component(ctx, shell.Footer)
		out.Static("</footer>")
	}
	return out.Err()
}

type Stack struct {
	Gap      string
	Children []html.Component
}

func (stack Stack) RenderHTML(ctx context.Context, out *html.Context) error {
	class := "stack"
	if stack.Gap != "" {
		gap, ok := gapToken(stack.Gap)
		if !ok {
			return fmt.Errorf("invalid stack gap %q", stack.Gap)
		}
		class += " stack--gap-" + gap
	}
	out.Static(`<div class="`)
	out.Static(class)
	out.Static(`">`)
	out.Components(ctx, stack.Children)
	out.Static("</div>")
	return out.Err()
}

type Grid struct {
	Columns  int
	Gap      string
	Children []html.Component
}

func (grid Grid) RenderHTML(ctx context.Context, out *html.Context) error {
	columns := grid.Columns
	if columns < 1 {
		columns = 1
	}
	if columns > 12 {
		return fmt.Errorf("grid columns %d exceeds maximum of 12", columns)
	}
	gap := "md"
	if grid.Gap != "" {
		var ok bool
		gap, ok = gapToken(grid.Gap)
		if !ok {
			return fmt.Errorf("invalid grid gap %q", grid.Gap)
		}
	}
	out.Static(`<div class="grid grid--columns-`)
	out.Int(columns)
	out.Static(" grid--gap-")
	out.Static(gap)
	out.Static(`">`)
	out.Components(ctx, grid.Children)
	out.Static("</div>")
	return out.Err()
}

type CardVariant string

const (
	CardDefault CardVariant = "default"
	CardPrimary CardVariant = "primary"
	CardSuccess CardVariant = "success"
	CardWarning CardVariant = "warning"
	CardDanger  CardVariant = "danger"
)

type Card struct {
	ID      string
	Header  html.Component
	Body    html.Component
	Footer  html.Component
	Variant CardVariant
}

func (card Card) RenderHTML(ctx context.Context, out *html.Context) error {
	variant := card.Variant
	if variant == "" {
		variant = CardDefault
	}
	if variant != CardDefault && variant != CardPrimary && variant != CardSuccess && variant != CardWarning && variant != CardDanger {
		return fmt.Errorf("invalid card variant %q", variant)
	}
	out.Static("<article class=\"card card--")
	out.Attribute(string(variant))
	out.Static("\"")
	if card.ID != "" {
		out.Static(" id=\"")
		out.Attribute(card.ID)
		out.Static("\"")
	}
	out.Static(">")
	if card.Header != nil {
		out.Static("<header class=\"card__header\">")
		out.Component(ctx, card.Header)
		out.Static("</header>")
	}
	out.Static("<div class=\"card__body\">")
	out.Component(ctx, card.Body)
	out.Static("</div>")
	if card.Footer != nil {
		out.Static("<footer class=\"card__footer\">")
		out.Component(ctx, card.Footer)
		out.Static("</footer>")
	}
	out.Static("</article>")
	return out.Err()
}

type AlertKind string

const (
	AlertInfo    AlertKind = "info"
	AlertSuccess AlertKind = "success"
	AlertWarning AlertKind = "warning"
	AlertDanger  AlertKind = "danger"
)

type Alert struct {
	ID      string
	Kind    AlertKind
	Title   string
	Message string
}

func (alert Alert) RenderHTML(_ context.Context, out *html.Context) error {
	kind := alert.Kind
	if kind == "" {
		kind = AlertInfo
	}
	if kind != AlertInfo && kind != AlertSuccess && kind != AlertWarning && kind != AlertDanger {
		return fmt.Errorf("invalid alert kind %q", kind)
	}
	out.Static("<div class=\"alert alert--")
	out.Attribute(string(kind))
	out.Static("\" role=\"status\"")
	if alert.ID != "" {
		out.Static(" id=\"")
		out.Attribute(alert.ID)
		out.Static("\"")
	}
	out.Static(">")
	if alert.Title != "" {
		out.Static("<strong>")
		out.Text(alert.Title)
		out.Static("</strong> ")
	}
	out.Text(alert.Message)
	out.Static("</div>")
	return out.Err()
}

type BadgeKind string

type Badge struct {
	Kind  BadgeKind
	Label string
}

func (badge Badge) RenderHTML(_ context.Context, out *html.Context) error {
	if badge.Kind != "" && badge.Kind != "info" && badge.Kind != "success" && badge.Kind != "warning" && badge.Kind != "danger" {
		return fmt.Errorf("invalid badge kind %q", badge.Kind)
	}
	out.Static("<span class=\"badge")
	if badge.Kind != "" {
		out.Static(" badge--")
		out.Attribute(string(badge.Kind))
	}
	out.Static("\">")
	out.Text(badge.Label)
	out.Static("</span>")
	return out.Err()
}

type ButtonType string
type ButtonVariant string

const (
	ButtonSubmit ButtonType = "submit"
	ButtonReset  ButtonType = "reset"
	ButtonButton ButtonType = "button"

	ButtonPrimary   ButtonVariant = "primary"
	ButtonSecondary ButtonVariant = "secondary"
	ButtonDanger    ButtonVariant = "danger"
	ButtonGhost     ButtonVariant = "ghost"
)

type Button struct {
	ID       string
	Type     ButtonType
	Label    string
	Variant  ButtonVariant
	Disabled bool
	Action   htmx.Action
	// AriaLabel names icon-only or symbol buttons such as "+" or "×".
	AriaLabel string
}

func validButtonVariant(variant ButtonVariant) bool {
	switch variant {
	case "", ButtonPrimary, ButtonSecondary, ButtonDanger, ButtonGhost:
		return true
	default:
		return false
	}
}

func buttonClass(out *html.Context, variant ButtonVariant) {
	out.Static(`class="button`)
	if variant != "" {
		out.Static(" button--")
		out.Attribute(string(variant))
	}
	out.Static(`"`)
}

// ThemeSwitcher posts mode=system|light|dark to Action (default "/theme").
// Works without JavaScript (303 redirect) and with HTMX (HX-Refresh).
// Pair with theme.Preference.Handler.
type ThemeSwitcher struct {
	Mode   theme.Mode
	Action string
	// Target is an optional HTMX target selector (for example "#app")
	// swapped with outerHTML; empty means no swap (the handler refreshes).
	Target string
	// Return is the same-origin path to come back to after a non-HTMX post.
	Return    string
	CSRFField html.Component
}

func (switcher ThemeSwitcher) RenderHTML(ctx context.Context, out *html.Context) error {
	path := switcher.Action
	if path == "" {
		path = "/theme"
	}
	if err := validateURL(path); err != nil {
		return fmt.Errorf("invalid theme switcher path: %w", err)
	}
	out.Static(`<form class="theme-switcher" method="post" action="`)
	out.Attribute(path)
	out.Static(`" aria-label="Color theme"`)
	action := htmx.Action{Method: htmx.Post, URL: path, Swap: htmx.NoSwap}
	if switcher.Target != "" {
		action.Target = switcher.Target
		action.Swap = htmx.OuterHTML
	}
	if err := action.RenderAttributes(out); err != nil {
		return err
	}
	out.Static(">")
	out.Component(ctx, switcher.CSRFField)
	if switcher.Return != "" {
		out.Static(`<input type="hidden" name="` + theme.ReturnField + `" value="`)
		out.Attribute(theme.SafeReturnPath(switcher.Return))
		out.Static(`">`)
	}
	for _, mode := range theme.Modes {
		out.Static(`<button type="submit" name="` + theme.ModeField + `" value="`)
		out.Static(mode.String())
		out.Static(`" class="button button--`)
		if mode == switcher.Mode {
			out.Static(`primary" aria-pressed="true">`)
		} else {
			out.Static(`secondary" aria-pressed="false">`)
		}
		out.Text(mode.Label())
		out.Static("</button>")
	}
	out.Static("</form>")
	return out.Err()
}
func (button Button) RenderHTML(_ context.Context, out *html.Context) error {
	buttonType := button.Type
	if buttonType == "" {
		buttonType = ButtonButton
	}
	if buttonType != ButtonButton && buttonType != ButtonSubmit && buttonType != ButtonReset {
		return fmt.Errorf("invalid button type %q", buttonType)
	}
	if !validButtonVariant(button.Variant) {
		return fmt.Errorf("invalid button variant %q", button.Variant)
	}
	out.Static("<button")
	optionalAttribute(out, "id", button.ID)
	out.Static(` type="`)
	out.Attribute(string(buttonType))
	out.Static(`" `)
	buttonClass(out, button.Variant)
	optionalAttribute(out, "aria-label", button.AriaLabel)
	if button.Disabled {
		out.Static(" disabled")
	}
	if button.Action.Method != 0 {
		if err := button.Action.RenderAttributes(out); err != nil {
			return err
		}
	}
	out.Static(">")
	out.Text(button.Label)
	out.Static("</button>")
	return out.Err()
}

type Link struct {
	Label  string
	URL    string
	Action htmx.Action
	// Variant renders the link as a button (class "button button--<variant>").
	Variant ButtonVariant
}

func (link Link) RenderHTML(_ context.Context, out *html.Context) error {
	if err := validateURL(link.URL); err != nil {
		return err
	}
	if !validButtonVariant(link.Variant) {
		return fmt.Errorf("invalid link variant %q", link.Variant)
	}
	out.Static("<a href=\"")
	out.Attribute(link.URL)
	out.Static("\"")
	if link.Variant != "" {
		out.Static(" ")
		buttonClass(out, link.Variant)
	}
	if link.Action.Method != 0 {
		if err := link.Action.RenderAttributes(out); err != nil {
			return err
		}
	}
	out.Static(">")
	out.Text(link.Label)
	out.Static("</a>")
	return out.Err()
}

type InputType string

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

func (input Input) RenderHTML(_ context.Context, out *html.Context) error {
	id := input.ID
	if id == "" {
		id = input.Name
	}
	inputType := input.Type
	if inputType == "" {
		inputType = "text"
	}
	if input.Label != "" {
		out.Static("<label for=\"")
		out.Attribute(id)
		out.Static("\">")
		out.Text(input.Label)
		out.Static("</label>")
	}
	out.Static("<input class=\"input\" id=\"")
	out.Attribute(id)
	out.Static("\" name=\"")
	out.Attribute(input.Name)
	out.Static("\" type=\"")
	out.Attribute(string(inputType))
	out.Static("\" value=\"")
	out.Attribute(input.Value)
	out.Static("\"")
	if input.Placeholder != "" {
		out.Static(" placeholder=\"")
		out.Attribute(input.Placeholder)
		out.Static("\"")
	}
	if input.AutoComplete != "" {
		out.Static(" autocomplete=\"")
		out.Attribute(input.AutoComplete)
		out.Static("\"")
	}
	if input.Required {
		out.Static(" required")
	}
	if input.Disabled {
		out.Static(" disabled")
	}
	if input.Error != "" {
		out.Static(" aria-invalid=\"true\" aria-describedby=\"")
		out.Attribute(id + "-error")
		out.Static("\"")
	}
	out.Static(">")
	if input.Error != "" {
		out.Static("<span id=\"")
		out.Attribute(id + "-error")
		out.Static("\" class=\"field-error\" role=\"alert\">")
		out.Text(input.Error)
		out.Static("</span>")
	}
	return out.Err()
}

type Option struct {
	Value    string
	Label    string
	Disabled bool
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

func (selectInput Select) RenderHTML(_ context.Context, out *html.Context) error {
	id := selectInput.ID
	if id == "" {
		id = selectInput.Name
	}
	if selectInput.Error != "" && id == "" {
		return fmt.Errorf("select validation error requires ID or Name")
	}
	if selectInput.Label != "" {
		out.Static("<label for=\"")
		out.Attribute(id)
		out.Static("\">")
		out.Text(selectInput.Label)
		out.Static("</label>")
	}
	out.Static("<select class=\"input\" id=\"")
	out.Attribute(id)
	out.Static("\" name=\"")
	out.Attribute(selectInput.Name)
	out.Static(`"`)
	if selectInput.Error != "" {
		out.Static(` aria-invalid="true" aria-describedby="`)
		out.Attribute(id + "-error")
		out.Static(`"`)
	}
	if selectInput.Required {
		out.Static(" required")
	}
	out.Static(">")
	for _, option := range selectInput.Options {
		out.Static("<option value=\"")
		out.Attribute(option.Value)
		out.Static("\"")
		if option.Value == selectInput.Value {
			out.Static(" selected")
		}
		if option.Disabled {
			out.Static(" disabled")
		}
		out.Static(">")
		out.Text(option.Label)
		out.Static("</option>")
	}
	out.Static("</select>")
	if selectInput.Error != "" {
		out.Static("<span class=\"field-error\" id=\"")
		out.Attribute(id + "-error")
		out.Static("\" role=\"alert\">")
		out.Text(selectInput.Error)
		out.Static("</span>")
	}
	return out.Err()
}

// Table renders a semantic table inside a horizontally scrollable wrapper.
// Header, Body, and Footer are wrapped in thead/tbody/tfoot. Sections, when
// set, are rendered as-is instead (use TableHeader and TableBody).
type Table struct {
	ID       string
	Caption  string
	Header   html.Component
	Body     html.Component
	Footer   html.Component
	Sections []html.Component
}

func (table Table) RenderHTML(ctx context.Context, out *html.Context) error {
	out.Static("<div class=\"table-scroll\"><table")
	if table.ID != "" {
		out.Static(" id=\"")
		out.Attribute(table.ID)
		out.Static("\"")
	}
	out.Static(">")
	if table.Caption != "" {
		out.Static("<caption>")
		out.Text(table.Caption)
		out.Static("</caption>")
	}
	if table.Sections != nil {
		out.Components(ctx, table.Sections)
		out.Static("</table></div>")
		return out.Err()
	}
	if table.Header != nil {
		out.Static("<thead>")
		out.Component(ctx, table.Header)
		out.Static("</thead>")
	}
	if table.Body != nil {
		out.Static("<tbody>")
		out.Component(ctx, table.Body)
		out.Static("</tbody>")
	}
	if table.Footer != nil {
		out.Static("<tfoot>")
		out.Component(ctx, table.Footer)
		out.Static("</tfoot>")
	}
	out.Static("</table></div>")
	return out.Err()
}

type Pagination struct {
	Page          int
	PageSize      int
	TotalItems    int
	Window        int
	URL           func(page int) string
	BaseURL       string
	QueryName     string
	Target        string
	Swap          htmx.Swap
	PushURL       bool
	PreviousLabel string
	NextLabel     string
	AriaLabel     string
}

// Pages returns the page count; 0 when PageSize or TotalItems ≤ 0.
func (pagination Pagination) Pages() int {
	if pagination.PageSize <= 0 || pagination.TotalItems <= 0 {
		return 0
	}
	return (pagination.TotalItems + pagination.PageSize - 1) / pagination.PageSize
}

// Current returns Page clamped to [1, Pages()].
func (pagination Pagination) Current() int {
	return min(max(pagination.Page, 1), max(pagination.Pages(), 1))
}

// RenderHTML renders nothing for ≤ 1 page. Otherwise: previous, page 1,
// ellipsis, window around current, ellipsis, last page, next.
func (pagination Pagination) RenderHTML(_ context.Context, out *html.Context) error {
	pages := pagination.Pages()
	if pages <= 1 {
		return nil
	}
	page := pagination.Current()
	window := pagination.Window
	if window < 1 {
		window = 2
	}
	label := pagination.AriaLabel
	if label == "" {
		label = "Pagination"
	}
	out.Static("<nav class=\"pagination\" aria-label=\"")
	out.Attribute(label)
	out.Static("\">")
	if page > 1 {
		if err := pagination.link(out, page-1, defaultString(pagination.PreviousLabel, "Previous"), "prev"); err != nil {
			return err
		}
	}
	first := max(page-window, 1)
	last := min(page+window, pages)
	if first > 1 {
		if err := pagination.number(out, 1, page); err != nil {
			return err
		}
		if first > 2 {
			out.Static(`<span class="pagination__gap" aria-hidden="true">…</span>`)
		}
	}
	for number := first; number <= last; number++ {
		if err := pagination.number(out, number, page); err != nil {
			return err
		}
	}
	if last < pages {
		if last < pages-1 {
			out.Static(`<span class="pagination__gap" aria-hidden="true">…</span>`)
		}
		if err := pagination.number(out, pages, page); err != nil {
			return err
		}
	}
	if page < pages {
		if err := pagination.link(out, page+1, defaultString(pagination.NextLabel, "Next"), "next"); err != nil {
			return err
		}
	}
	out.Static("</nav>")
	return out.Err()
}

func (pagination Pagination) number(out *html.Context, number, current int) error {
	if number == current {
		out.Static("<span aria-current=\"page\">")
		out.Int(number)
		out.Static("</span>")
		return out.Err()
	}
	return pagination.link(out, number, strconv.Itoa(number), "")
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
func (pagination Pagination) link(out *html.Context, page int, label, rel string) error {
	href := pagination.pageURL(page)
	if err := validateURL(href); err != nil {
		return err
	}
	out.Static("<a href=\"")
	out.Attribute(href)
	out.Static("\"")
	optionalAttribute(out, "rel", rel)
	if pagination.Target != "" {
		action := htmx.Action{Method: htmx.Get, URL: href, Target: pagination.Target, Swap: pagination.Swap}
		if action.Swap == "" {
			action.Swap = htmx.OuterHTML
		}
		if pagination.PushURL {
			action.PushURL = "true"
		}
		if err := action.RenderAttributes(out); err != nil {
			return err
		}
	}
	out.Static(">")
	out.Text(label)
	out.Static("</a>")
	return out.Err()
}

func (pagination Pagination) pageURL(page int) string {
	if pagination.URL != nil {
		return pagination.URL(page)
	}
	base := pagination.BaseURL
	if base == "" {
		base = "/"
	}
	queryName := pagination.QueryName
	if queryName == "" {
		queryName = "page"
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return base
	}
	query := parsed.Query()
	query.Set(queryName, strconv.Itoa(page))
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

type Breadcrumb struct {
	Label string
	URL   string
}

type Breadcrumbs struct {
	Items []Breadcrumb
}

func (breadcrumbs Breadcrumbs) RenderHTML(_ context.Context, out *html.Context) error {
	out.Static("<nav aria-label=\"Breadcrumb\"><ol>")
	for index, item := range breadcrumbs.Items {
		if item.URL != "" {
			if err := validateURL(item.URL); err != nil {
				return err
			}
		}
		out.Static("<li>")
		if index < len(breadcrumbs.Items)-1 && item.URL != "" {
			out.Static("<a href=\"")
			out.Attribute(item.URL)
			out.Static("\">")
			out.Text(item.Label)
			out.Static("</a>")
		} else {
			out.Static("<span aria-current=\"page\">")
			out.Text(item.Label)
			out.Static("</span>")
		}
		out.Static("</li>")
	}
	out.Static("</ol></nav>")
	return out.Err()
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

func (navigation Navigation) RenderHTML(_ context.Context, out *html.Context) error {
	label := navigation.Label
	if label == "" {
		label = "Navigation"
	}
	out.Static("<nav aria-label=\"")
	out.Attribute(label)
	out.Static("\"><ul>")
	for _, item := range navigation.Items {
		if item.URL != "" {
			if err := validateURL(item.URL); err != nil {
				return err
			}
		}
		out.Static("<li>")
		if item.Disabled || item.URL == "" {
			out.Static("<span")
			if item.Disabled {
				out.Static(" aria-disabled=\"true\"")
			}
			out.Static(">")
			out.Text(item.Label)
			out.Static("</span>")
		} else {
			out.Static("<a href=\"")
			out.Attribute(item.URL)
			out.Static("\"")
			if item.Current {
				out.Static(" aria-current=\"page\"")
			}
			out.Static(">")
			out.Text(item.Label)
			out.Static("</a>")
		}
		if len(item.Children) != 0 {
			out.Static("<ul>")
			for _, child := range item.Children {
				if err := validateURL(child.URL); err != nil {
					return err
				}
				out.Static("<li><a href=\"")
				out.Attribute(child.URL)
				out.Static("\">")
				out.Text(child.Label)
				out.Static("</a></li>")
			}
			out.Static("</ul>")
		}
		out.Static("</li>")
	}
	out.Static("</ul></nav>")
	return out.Err()
}

func gapToken(value string) (string, bool) {
	switch value {
	case "0", "0px":
		return "none", true
	case "0.25rem", "4px":
		return "xs", true
	case "0.5rem", "8px":
		return "sm", true
	case "1rem", "16px":
		return "md", true
	case "1.5rem", "24px":
		return "lg", true
	case "2rem", "32px":
		return "xl", true
	default:
		return "", false
	}
}

func validateURL(value string) error {
	if strings.TrimSpace(value) == "" || value != strings.TrimSpace(value) ||
		strings.ContainsAny(value, "\x00\r\n\t\f\v\\") {
		return fmt.Errorf("invalid URL %q", value)
	}
	parsedURL, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("invalid URL %q: %w", value, err)
	}
	switch strings.ToLower(parsedURL.Scheme) {
	case "", "http", "https", "mailto", "tel":
		return nil
	default:
		return fmt.Errorf("unsupported URL scheme %q", parsedURL.Scheme)
	}
}
