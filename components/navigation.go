package components

import (
	"context"
	"net/url"
	"strconv"

	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/web"
)

type Pagination struct {
	Page          int
	PageSize      int
	TotalItems    int
	Window        int
	URL           func(page int) string
	BaseURL       string
	QueryName     string
	Target        string
	Swap          web.Swap
	PushURL       bool
	PreviousLabel string
	NextLabel     string
	AriaLabel     string
	// Attrs apply to the <nav> element.
	Attrs html.Attrs
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
	out.Static("<nav class=\"pagination")
	out.ExtraClass(pagination.Attrs)
	out.Static("\" aria-label=\"")
	out.Attribute(label)
	out.Static("\"")
	out.PassAttrs(pagination.Attrs, "aria-label")
	out.Static(">")
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
		action := web.BrowserAction{Method: web.Get, URL: href, Target: pagination.Target, Swap: pagination.Swap}
		if action.Swap == "" {
			action.Swap = web.OuterHTML
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
	// Attrs apply to the <nav> element.
	Attrs html.Attrs
}

func (breadcrumbs Breadcrumbs) RenderHTML(_ context.Context, out *html.Context) error {
	out.Static("<nav aria-label=\"Breadcrumb\"")
	out.Class("", breadcrumbs.Attrs)
	out.PassAttrs(breadcrumbs.Attrs, "aria-label")
	out.Static("><ol>")
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
	// Attrs apply to the <nav> element.
	Attrs html.Attrs
}

func (navigation Navigation) RenderHTML(_ context.Context, out *html.Context) error {
	label := navigation.Label
	if label == "" {
		label = "Navigation"
	}
	out.Static("<nav aria-label=\"")
	out.Attribute(label)
	out.Static("\"")
	out.Class("", navigation.Attrs)
	out.PassAttrs(navigation.Attrs, "aria-label")
	out.Static("><ul>")
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
