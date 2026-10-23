package html

import (
	"context"
	"errors"
	"io"
	"math"
	"strings"
	"testing"
)

func render(t *testing.T, component Component) string {
	t.Helper()
	var output strings.Builder
	if err := Render(context.Background(), &output, component); err != nil {
		t.Fatal(err)
	}
	return output.String()
}

func TestEscapingTextAndAttributes(t *testing.T) {
	got := render(t, ComponentFunc(func(_ context.Context, out *Context) error {
		out.Text(`<a href='x'>&"`)
		out.Static(`<p title="`)
		out.Attribute(`" onclick="x`)
		out.Static(`">`)
		out.Text("plain\x00")
		return out.Err()
	}))
	want := `&lt;a href=&#39;x&#39;&gt;&amp;&#34;<p title="&#34; onclick=&#34;x">plain` + "\uFFFD"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestTrustedHTMLIsWrittenVerbatim(t *testing.T) {
	got := render(t, ComponentFunc(func(_ context.Context, out *Context) error {
		out.Trusted(TrustStatic("<b>ok</b>"))
		out.Text("<b>")
		return out.Err()
	}))
	if got != "<b>ok</b>&lt;b&gt;" {
		t.Fatalf("got %q", got)
	}
}

func TestRenderRetainsFirstError(t *testing.T) {
	firstErr := errors.New("first")
	component := ComponentFunc(func(ctx context.Context, out *Context) error {
		out.Text(`<script>alert("x")</script>`)
		out.Component(ctx, ComponentFunc(func(context.Context, *Context) error {
			return firstErr
		}))
		out.Static("never")
		return errors.New("second")
	})
	var output strings.Builder
	err := Render(context.Background(), &output, component)
	if !errors.Is(err, firstErr) {
		t.Fatalf("Render() error = %v, want first error", err)
	}
	if got, want := output.String(), "&lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt;"; got != want {
		t.Fatalf("rendered output = %q, want %q", got, want)
	}
}

type failingWriter struct{ writes int }

func (w *failingWriter) Write(p []byte) (int, error) {
	w.writes++
	return 0, io.ErrClosedPipe
}

func TestNoWritesAfterOutputError(t *testing.T) {
	writer := &failingWriter{}
	out := NewContext(writer)
	out.Static("a")
	out.Static("b")
	out.Int(1)
	if !errors.Is(out.Err(), io.ErrClosedPipe) || writer.writes != 1 {
		t.Fatalf("err %v after %d writes, want 1 write", out.Err(), writer.writes)
	}
}

func TestNestedSequenceOptionalAndNumbers(t *testing.T) {
	item := func(text string) Component {
		return ComponentFunc(func(_ context.Context, out *Context) error {
			out.Text(text)
			return out.Err()
		})
	}
	got := render(t, ComponentFunc(func(ctx context.Context, out *Context) error {
		out.Components(ctx, []Component{item("a"), nil, item("b")})
		out.Component(ctx, Optional{Show: false, Body: item("hidden")})
		out.Component(ctx, Optional{Show: true, Body: item("c")})
		out.Int(-42)
		out.Int64(math.MaxInt64)
		out.Uint64(7)
		out.Float64(1.5)
		out.Bool(true)
		return out.Err()
	}))
	if want := "abc-42922337203685477580771.5true"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestComponentsStopOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var output strings.Builder
	out := NewContext(&output)
	out.Components(ctx, []Component{
		ComponentFunc(func(_ context.Context, out *Context) error {
			out.Static("first")
			cancel()
			return nil
		}),
		ComponentFunc(func(_ context.Context, out *Context) error {
			out.Static("second")
			return nil
		}),
	})
	if !errors.Is(out.Err(), context.Canceled) || output.String() != "first" {
		t.Fatalf("err %v output %q", out.Err(), output.String())
	}
}

func TestOrdinaryTextDoesNotAllocate(t *testing.T) {
	out := NewContext(io.Discard)
	allocs := testing.AllocsPerRun(100, func() {
		out.Text("ordinary text without specials")
		out.Attribute("value")
		out.Int(12345)
	})
	if allocs != 0 {
		t.Fatalf("allocations = %v, want 0", allocs)
	}
}
