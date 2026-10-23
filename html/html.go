package html

import (
	"context"
	"io"
	"strconv"
)

// Component renders itself directly to a Context. Implementations must not
// build intermediate node trees.
type Component interface {
	RenderHTML(context.Context, *Context) error
}

// ComponentFunc adapts a function to Component.
type ComponentFunc func(context.Context, *Context) error

func (fn ComponentFunc) RenderHTML(ctx context.Context, out *Context) error {
	return fn(ctx, out)
}

// Page is a complete HTML document.
type Page interface {
	Component
	IsPage()
}

// Fragment is a stable HTML subtree suitable for HTMX replacement.
type Fragment interface {
	Component
	IsFragment()
}

type PageMarker struct{}

func (PageMarker) IsPage() {}

type FragmentMarker struct{}

func (FragmentMarker) IsFragment() {}

// TrustedHTML is audited, server-controlled markup written without escaping.
type TrustedHTML string

// TrustStatic marks audited, server-controlled HTML as trusted. Never pass
// request, form, database, URL, or other user-provided content.
func TrustStatic(value string) TrustedHTML { return TrustedHTML(value) }

// Optional renders Body only when Show is true.
type Optional struct {
	Show bool
	Body Component
}

func (optional Optional) RenderHTML(ctx context.Context, out *Context) error {
	if optional.Show {
		out.Component(ctx, optional.Body)
	}
	return out.Err()
}

// Context streams escaped HTML to an io.Writer. The first error is sticky:
// after it, every write is a no-op.
type Context struct {
	writer  io.Writer
	err     error
	scratch [64]byte
}

func NewContext(w io.Writer) *Context { return &Context{writer: w} }

// Render streams component to w.
func Render(ctx context.Context, w io.Writer, component Component) error {
	if component == nil {
		return nil
	}
	out := NewContext(w)
	out.Component(ctx, component)
	return out.Err()
}

func (c *Context) Err() error        { return c.err }
func (c *Context) Writer() io.Writer { return c.writer }

// Fail records err unless an earlier error exists.
func (c *Context) Fail(err error) {
	if c.err == nil && err != nil {
		c.err = err
	}
}

func (c *Context) write(value string) {
	if c.err != nil || value == "" {
		return
	}
	_, c.err = io.WriteString(c.writer, value)
}

func (c *Context) writeBytes(value []byte) {
	if c.err != nil {
		return
	}
	_, c.err = c.writer.Write(value)
}

// Static writes developer-controlled markup unescaped.
func (c *Context) Static(value string) { c.write(value) }

// Trusted writes audited markup unescaped.
func (c *Context) Trusted(value TrustedHTML) { c.write(string(value)) }

// Text writes HTML-escaped text content.
func (c *Context) Text(value string) { c.escape(value) }

// Attribute writes an HTML-escaped attribute value. Callers supply quotes.
func (c *Context) Attribute(value string) { c.escape(value) }

// escape writes unescaped runs directly and replaces only special bytes, so
// ordinary strings cause no allocation.
func (c *Context) escape(value string) {
	start := 0
	for index := 0; index < len(value); index++ {
		var replacement string
		switch value[index] {
		case '&':
			replacement = "&amp;"
		case '<':
			replacement = "&lt;"
		case '>':
			replacement = "&gt;"
		case '"':
			replacement = "&#34;"
		case '\'':
			replacement = "&#39;"
		case 0:
			replacement = "\uFFFD"
		default:
			continue
		}
		c.write(value[start:index])
		c.write(replacement)
		start = index + 1
	}
	c.write(value[start:])
}

func (c *Context) Int(value int)     { c.writeBytes(strconv.AppendInt(c.scratch[:0], int64(value), 10)) }
func (c *Context) Int64(value int64) { c.writeBytes(strconv.AppendInt(c.scratch[:0], value, 10)) }
func (c *Context) Uint64(value uint64) {
	c.writeBytes(strconv.AppendUint(c.scratch[:0], value, 10))
}
func (c *Context) Float64(value float64) {
	c.writeBytes(strconv.AppendFloat(c.scratch[:0], value, 'f', -1, 64))
}
func (c *Context) Bool(value bool) { c.writeBytes(strconv.AppendBool(c.scratch[:0], value)) }

// Component renders a nested component unless an error is already recorded
// or ctx is cancelled.
func (c *Context) Component(ctx context.Context, component Component) {
	if c.err != nil || component == nil {
		return
	}
	if err := ctx.Err(); err != nil {
		c.err = err
		return
	}
	c.Fail(component.RenderHTML(ctx, c))
}

// Components renders components in order and stops at the first error or
// cancellation.
func (c *Context) Components(ctx context.Context, components []Component) {
	for _, component := range components {
		c.Component(ctx, component)
		if c.err != nil {
			return
		}
	}
}
