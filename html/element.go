package html

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

var (
	// ErrUnsafeAttribute reports an attribute name or value rejected by the
	// passthrough allowlist (event handlers, style, javascript: URLs, ...).
	ErrUnsafeAttribute = errors.New("unsafe attribute")
	// ErrInvalidElement reports an invalid or forbidden tag name, or
	// children passed to a void element.
	ErrInvalidElement = errors.New("invalid element")
	// ErrUnsafeURL reports a URL with a scheme other than http, https,
	// mailto or tel, or with control characters.
	ErrUnsafeURL = errors.New("unsafe URL")
)

// Attr is one HTML attribute. Flag renders a boolean attribute (name only)
// and ignores Value.
type Attr struct {
	Name  string
	Value string
	Flag  bool
}

// Attrs is an ordered attribute list. Order is preserved in the output, so
// rendering is deterministic.
type Attrs []Attr

// A returns a name="value" attribute.
func A(name, value string) Attr { return Attr{Name: name, Value: value} }

// Flag returns a boolean attribute such as disabled or hidden.
func Flag(name string) Attr { return Attr{Name: name, Flag: true} }

// Class joins every class attribute value in attrs with single spaces.
func (attrs Attrs) Class() string {
	class := ""
	for _, attr := range attrs {
		if !classAttr(attr) {
			continue
		}
		if class != "" {
			class += " "
		}
		class += attr.Value
	}
	return class
}

func (attrs Attrs) hasClass() bool {
	for _, attr := range attrs {
		if classAttr(attr) {
			return true
		}
	}
	return false
}

// Has reports whether attrs contains name (ASCII case-insensitive).
func (attrs Attrs) Has(name string) bool {
	for _, attr := range attrs {
		if strings.EqualFold(attr.Name, name) {
			return true
		}
	}
	return false
}

// Text is escaped text content.
type Text string

func (value Text) RenderHTML(_ context.Context, out *Context) error {
	out.Text(string(value))
	return out.Err()
}

// Element is a generic HTML element built at runtime. Tag must match
// [a-z][a-z0-9-]*; script, style and other active or resource-loading
// elements are rejected. Attributes pass the same allowlist as component
// passthrough attributes.
type Element struct {
	Tag      string
	Attrs    Attrs
	Children []Component
}

// El builds an Element. It is the low-level escape hatch for markup no
// component covers:
//
//	html.El("section", html.Attrs{html.A("class", "panel")},
//		html.El("h2", nil, html.Text("Title")),
//	)
func El(tag string, attrs Attrs, children ...Component) Element {
	return Element{Tag: tag, Attrs: attrs, Children: children}
}

func (element Element) RenderHTML(ctx context.Context, out *Context) error {
	if err := ValidateTag(element.Tag); err != nil {
		return err
	}
	void := voidElement(element.Tag)
	if void && len(element.Children) != 0 {
		return fmt.Errorf("%w: <%s> is void and cannot have children", ErrInvalidElement, element.Tag)
	}
	out.Static("<")
	out.Static(element.Tag)
	out.Attrs(element.Attrs)
	out.Static(">")
	if void {
		return out.Err()
	}
	out.Components(ctx, element.Children)
	out.Static("</")
	out.Static(element.Tag)
	out.Static(">")
	return out.Err()
}

// ValidateTag accepts lowercase tag names that cannot execute script, load
// stylesheets or embed foreign documents.
func ValidateTag(tag string) error {
	if tag == "" || tag[0] < 'a' || tag[0] > 'z' {
		return fmt.Errorf("%w: tag %q", ErrInvalidElement, tag)
	}
	for index := 1; index < len(tag); index++ {
		char := tag[index]
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
			return fmt.Errorf("%w: tag %q", ErrInvalidElement, tag)
		}
	}
	switch tag {
	case "script", "style", "link", "meta", "base", "iframe", "frame", "frameset",
		"object", "embed", "applet", "noscript", "template", "html", "head", "body":
		return fmt.Errorf("%w: tag <%s> is not allowed", ErrInvalidElement, tag)
	}
	return nil
}

func voidElement(tag string) bool {
	switch tag {
	case "area", "br", "col", "hr", "img", "input", "source", "track", "wbr":
		return true
	}
	return false
}

// Attrs writes every attribute as ` name="value"` (or ` name` for flags),
// recording ErrUnsafeAttribute for names or values outside the allowlist.
func (c *Context) Attrs(attrs Attrs) {
	for _, attr := range attrs {
		c.attr(attr)
	}
}

// Class writes ` class="base extra"` where extra are the class values in
// attrs. Nothing is written when both are empty. Pair with PassAttrs.
func (c *Context) Class(base string, attrs Attrs) {
	if base == "" && !attrs.hasClass() {
		return
	}
	c.write(` class="`)
	c.escape(base)
	separate := base != ""
	for _, attr := range attrs {
		if !classAttr(attr) {
			continue
		}
		if separate {
			c.write(" ")
		}
		c.escape(attr.Value)
		separate = true
	}
	c.write(`"`)
}

// ExtraClass writes " value" for each class value in attrs. Call it inside
// an open class attribute, before the closing quote.
func (c *Context) ExtraClass(attrs Attrs) {
	for _, attr := range attrs {
		if classAttr(attr) {
			c.write(" ")
			c.escape(attr.Value)
		}
	}
}

func classAttr(attr Attr) bool {
	return !attr.Flag && attr.Value != "" && strings.EqualFold(attr.Name, "class")
}

// PassAttrs writes component passthrough attributes. It skips class (merge
// it with Class or ExtraClass) and every name in owned, which the component
// already wrote: component-owned attributes win over passthrough values.
// An owned entry ending in "*" matches a prefix, e.g. "hx-*". Empty owned
// entries match nothing.
func (c *Context) PassAttrs(attrs Attrs, owned ...string) {
	for _, attr := range attrs {
		if strings.EqualFold(attr.Name, "class") || ownedName(attr.Name, owned) {
			continue
		}
		c.attr(attr)
	}
}

func ownedName(name string, owned []string) bool {
	for _, candidate := range owned {
		if candidate == "" {
			continue
		}
		if prefix, ok := strings.CutSuffix(candidate, "*"); ok {
			if len(name) >= len(prefix) && strings.EqualFold(name[:len(prefix)], prefix) {
				return true
			}
			continue
		}
		if strings.EqualFold(name, candidate) {
			return true
		}
	}
	return false
}

func (c *Context) attr(attr Attr) {
	if c.err != nil {
		return
	}
	if err := ValidateAttr(attr); err != nil {
		c.Fail(err)
		return
	}
	c.write(" ")
	c.write(attr.Name)
	if attr.Flag {
		return
	}
	c.write(`="`)
	c.escape(attr.Value)
	c.write(`"`)
}

// ValidateAttr applies the attribute allowlist:
//   - names match [A-Za-z_:][A-Za-z0-9_.:-]*;
//   - event handlers (on*), style, srcdoc, hx-on*, hx-vars and their data-
//     prefixed htmx forms are rejected;
//   - URL attributes (href, src, action, hx-get, ...) must pass ValidateURL;
//   - hx-vals/hx-headers must not use js:/javascript: evaluation and
//     hx-trigger must not contain [...] JavaScript filters.
func ValidateAttr(attr Attr) error {
	name := attr.Name
	if !validAttrName(name) {
		return fmt.Errorf("%w: name %q", ErrUnsafeAttribute, name)
	}
	lower := strings.ToLower(name)
	htmxName := strings.TrimPrefix(lower, "data-")
	switch {
	case strings.HasPrefix(lower, "on"),
		lower == "style", lower == "srcdoc",
		strings.HasPrefix(htmxName, "hx-on"),
		htmxName == "hx-vars":
		return fmt.Errorf("%w: %q is not allowed", ErrUnsafeAttribute, name)
	}
	if attr.Flag {
		return nil
	}
	value := attr.Value
	switch htmxName {
	case "hx-vals", "hx-headers":
		trimmed := strings.ToLower(strings.TrimSpace(value))
		if strings.HasPrefix(trimmed, "js:") || strings.HasPrefix(trimmed, "javascript:") {
			return fmt.Errorf("%w: %s evaluates JavaScript", ErrUnsafeAttribute, name)
		}
		return nil
	case "hx-trigger":
		if strings.ContainsRune(value, '[') {
			return fmt.Errorf("%w: hx-trigger filters evaluate JavaScript", ErrUnsafeAttribute)
		}
		return nil
	case "hx-push-url", "hx-replace-url":
		if value == "true" || value == "false" {
			return nil
		}
	}
	if urlAttribute(lower, htmxName) {
		if err := ValidateURL(value); err != nil {
			return fmt.Errorf("%w: %s: %w", ErrUnsafeAttribute, name, err)
		}
	}
	return nil
}

func validAttrName(name string) bool {
	if name == "" {
		return false
	}
	for index := 0; index < len(name); index++ {
		char := name[index]
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z', char == '_', char == ':':
		case index > 0 && (char >= '0' && char <= '9' || char == '-' || char == '.'):
		default:
			return false
		}
	}
	return true
}

func urlAttribute(lower, htmxName string) bool {
	switch lower {
	case "href", "src", "action", "formaction", "poster", "cite", "data",
		"background", "longdesc", "manifest", "ping", "xlink:href":
		return true
	}
	switch htmxName {
	case "hx-get", "hx-post", "hx-put", "hx-patch", "hx-delete", "hx-push-url", "hx-replace-url":
		return true
	}
	return false
}

// ValidateURL accepts relative URLs and http, https, mailto and tel URLs
// without control characters or backslashes.
func ValidateURL(value string) error {
	if strings.TrimSpace(value) == "" || value != strings.TrimSpace(value) ||
		strings.ContainsAny(value, "\x00\r\n\t\f\v\\") {
		return fmt.Errorf("%w: %q", ErrUnsafeURL, value)
	}
	if plainURL(value) {
		return nil
	}
	parsedURL, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("%w: %q: %w", ErrUnsafeURL, value, err)
	}
	switch strings.ToLower(parsedURL.Scheme) {
	case "", "http", "https", "mailto", "tel":
		return nil
	default:
		return fmt.Errorf("%w: scheme %q", ErrUnsafeURL, parsedURL.Scheme)
	}
}

// plainURL reports a root-relative path or fragment without escapes or
// authority. It carries no scheme, so url.Parse (an allocation) is skipped.
func plainURL(value string) bool {
	switch value[0] {
	case '#':
		return true
	case '/':
		return (len(value) == 1 || value[1] != '/') && !strings.ContainsRune(value, '%')
	}
	return false
}
