package shared

import (
	"context"
	"errors"
	"fmt"

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/html"
	"github.com/dmundt/go-mvu/htmx"
)

// AccordionItem is one collapsible section of an Accordion.
type AccordionItem struct {
	Title string
	Body  html.Component
	Open  bool
}

// Accordion renders native details/summary sections. Exclusive groups share
// a details name so opening one section closes the others.
type Accordion struct {
	ID        string
	Exclusive bool
	Items     []AccordionItem
	Attrs     html.Attrs
}

func (accordion Accordion) RenderHTML(ctx context.Context, out *html.Context) error {
	if accordion.Exclusive || accordion.ID != "" {
		if err := validID(accordion.ID); err != nil {
			return err
		}
	}
	if len(accordion.Items) == 0 {
		return errors.New("shared: accordion requires items")
	}
	open := 0
	for index, item := range accordion.Items {
		if item.Title == "" {
			return fmt.Errorf("shared: accordion item %d requires title", index)
		}
		if item.Open {
			open++
		}
	}
	if accordion.Exclusive && open > 1 {
		return errors.New("shared: exclusive accordion allows one open item")
	}
	out.Static(`<div class="accordion`)
	out.ExtraClass(accordion.Attrs)
	out.Static(`"`)
	optionalAttr(out, "id", accordion.ID)
	out.PassAttrs(accordion.Attrs, when(accordion.ID, "id"))
	out.Static(">")
	for _, item := range accordion.Items {
		out.Static(`<details class="accordion__item"`)
		if accordion.Exclusive {
			attr(out, "name", accordion.ID)
		}
		flag(out, "open", item.Open)
		out.Static(`><summary class="accordion__trigger">`)
		out.Text(item.Title)
		out.Static(`</summary><div class="accordion__content">`)
		out.Component(ctx, item.Body)
		out.Static("</div></details>")
	}
	out.Static("</div>")
	return out.Err()
}

// Tab is one server-rendered tab link.
type Tab struct {
	ID    string
	Label string
	URL   string
}

// Tabs renders server-selected tab navigation and the active panel. Each tab
// is an ordinary link; HTMX optionally swaps the tab set in place.
type Tabs struct {
	ID     string
	Label  string
	Items  []Tab
	Active string
	Panel  html.Component
	HTMX   bool
	// Attrs apply to the div.tabset root.
	Attrs html.Attrs
}

func (tabs Tabs) RenderHTML(ctx context.Context, out *html.Context) error {
	if err := validID(tabs.ID); err != nil {
		return err
	}
	if tabs.Label == "" || len(tabs.Items) == 0 {
		return errors.New("shared: tabs require label and items")
	}
	seen := make(map[string]bool, len(tabs.Items))
	for _, tab := range tabs.Items {
		if err := validID(tab.ID); err != nil {
			return err
		}
		if seen[tab.ID] {
			return fmt.Errorf("shared: duplicate tab %q", tab.ID)
		}
		seen[tab.ID] = true
		if tab.Label == "" {
			return fmt.Errorf("shared: tab %q requires label", tab.ID)
		}
		if err := validURL(tab.URL); err != nil {
			return err
		}
	}
	if !seen[tabs.Active] {
		return fmt.Errorf("shared: unknown active tab %q", tabs.Active)
	}
	out.Static(`<div class="tabset`)
	out.ExtraClass(tabs.Attrs)
	out.Static(`"`)
	attr(out, "id", tabs.ID)
	out.PassAttrs(tabs.Attrs, "id")
	out.Static(`><nav class="tabset__list"`)
	attr(out, "aria-label", tabs.Label)
	out.Static(">")
	for _, tab := range tabs.Items {
		out.Static(`<a class="tabset__tab"`)
		attr(out, "id", tabs.ID+"-tab-"+tab.ID)
		attr(out, "href", tab.URL)
		if tab.ID == tabs.Active {
			out.Static(` aria-current="page"`)
		}
		if tabs.HTMX {
			action := htmx.Action{
				Method: htmx.Get, URL: tab.URL, Target: "#" + tabs.ID,
				Select: "#" + tabs.ID, Swap: htmx.OuterHTML, PushURL: "true",
			}
			if err := action.RenderAttributes(out); err != nil {
				return err
			}
		}
		out.Static(">")
		out.Text(tab.Label)
		out.Static("</a>")
	}
	out.Static(`</nav><section class="tabset__panel"`)
	attr(out, "id", tabs.ID+"-panel")
	attr(out, "aria-labelledby", tabs.ID+"-tab-"+tabs.Active)
	out.Static(">")
	out.Component(ctx, tabs.Panel)
	out.Static("</section></div>")
	return out.Err()
}

// Dialog renders a trigger button and a native popover dialog. The browser
// opens, closes, and light-dismisses it without JavaScript.
type Dialog struct {
	ID             string
	Title          string
	TriggerLabel   string
	TriggerVariant components.ButtonVariant
	CloseLabel     string
	Body           html.Component
	Footer         html.Component
	// Attrs apply to the popover div (not the trigger button).
	Attrs html.Attrs
}

func (dialog Dialog) RenderHTML(ctx context.Context, out *html.Context) error {
	if err := validID(dialog.ID); err != nil {
		return err
	}
	if dialog.Title == "" || dialog.TriggerLabel == "" {
		return errors.New("shared: dialog requires title and trigger label")
	}
	closeLabel := dialog.CloseLabel
	if closeLabel == "" {
		closeLabel = "Close"
	}
	out.Component(ctx, components.Button{
		Label:   dialog.TriggerLabel,
		Variant: dialog.TriggerVariant,
		Attrs:   html.Attrs{html.A("popovertarget", dialog.ID)},
	})
	out.Static(`<div class="dialog`)
	out.ExtraClass(dialog.Attrs)
	out.Static(`" popover role="dialog"`)
	attr(out, "id", dialog.ID)
	attr(out, "aria-labelledby", dialog.ID+"-title")
	out.PassAttrs(dialog.Attrs, "popover", "role", "id", "aria-labelledby")
	out.Static(`><header class="dialog__header"><h2 class="dialog__title"`)
	attr(out, "id", dialog.ID+"-title")
	out.Static(">")
	out.Text(dialog.Title)
	out.Static(`</h2><button type="button" class="dialog__close" popovertargetaction="hide"`)
	attr(out, "popovertarget", dialog.ID)
	attr(out, "aria-label", closeLabel)
	out.Static(`><span aria-hidden="true">&times;</span></button></header><div class="dialog__body">`)
	out.Component(ctx, dialog.Body)
	out.Static("</div>")
	if dialog.Footer != nil {
		out.Static(`<footer class="dialog__footer">`)
		out.Component(ctx, dialog.Footer)
		out.Static("</footer>")
	}
	out.Static("</div>")
	return out.Err()
}

// MenuItem is one Dropdown link.
type MenuItem struct {
	Label string
	URL   string
}

// Dropdown renders a disclosure menu of links using details/summary.
type Dropdown struct {
	Label    string
	Items    []MenuItem
	AlignEnd bool
	Attrs    html.Attrs
}

func (dropdown Dropdown) RenderHTML(_ context.Context, out *html.Context) error {
	if dropdown.Label == "" || len(dropdown.Items) == 0 {
		return errors.New("shared: dropdown requires label and items")
	}
	for _, item := range dropdown.Items {
		if item.Label == "" {
			return errors.New("shared: dropdown item requires label")
		}
		if err := validURL(item.URL, "mailto", "tel"); err != nil {
			return err
		}
	}
	out.Static(`<details class="dropdown`)
	if dropdown.AlignEnd {
		out.Static(" dropdown--end")
	}
	out.ExtraClass(dropdown.Attrs)
	out.Static(`"`)
	out.PassAttrs(dropdown.Attrs)
	out.Static(`><summary>`)
	out.Text(dropdown.Label)
	out.Static(`</summary><ul class="dropdown__menu">`)
	for _, item := range dropdown.Items {
		out.Static("<li><a")
		attr(out, "href", item.URL)
		out.Static(">")
		out.Text(item.Label)
		out.Static("</a></li>")
	}
	out.Static("</ul></details>")
	return out.Err()
}

// Tooltip renders a button that toggles an accessible, dismissible tooltip.
type Tooltip struct {
	ID      string
	Label   string
	Content string
	// Attrs apply to the span.tip wrapper.
	Attrs html.Attrs
}

func (tooltip Tooltip) RenderHTML(ctx context.Context, out *html.Context) error {
	if err := validID(tooltip.ID); err != nil {
		return err
	}
	if tooltip.Label == "" || tooltip.Content == "" {
		return errors.New("shared: tooltip requires label and content")
	}
	out.Static(`<span class="tip`)
	out.ExtraClass(tooltip.Attrs)
	out.Static(`"`)
	out.PassAttrs(tooltip.Attrs)
	out.Static(">")
	out.Component(ctx, components.Button{
		Label:   tooltip.Label,
		Variant: components.ButtonSecondary,
		Attrs: html.Attrs{
			html.A("aria-describedby", tooltip.ID),
			html.A("aria-controls", tooltip.ID),
			html.A("popovertarget", tooltip.ID),
		},
	})
	out.Static(`<span class="tip__content" popover="auto" role="tooltip"`)
	attr(out, "id", tooltip.ID)
	out.Static(">")
	out.Text(tooltip.Content)
	out.Static("</span></span>")
	return out.Err()
}
