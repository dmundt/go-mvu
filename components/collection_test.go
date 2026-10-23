package components

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/dmundt/go-mvu/html"
)

func TestMapAndTextCellsPreserveOrderAndSemantics(t *testing.T) {
	rows := Map([]string{"<first>", "second"}, func(value string) html.Component {
		return TableRow{Cells: []html.Component{RowHeader(value), TextCell(value)}}
	})
	want := `<tr><th scope="row">&lt;first&gt;</th><td>&lt;first&gt;</td></tr>` +
		`<tr><th scope="row">second</th><td>second</td></tr>`
	if got := render(t, rows); got != want {
		t.Fatalf("rows = %s, want %s", got, want)
	}
	calls := 0
	empty := Map([]string(nil), func(string) html.Component { calls++; return nil })
	if got := render(t, empty); got != "" || calls != 0 {
		t.Fatalf("empty map = %q, calls = %d", got, calls)
	}
}

func TestMappedComponentsPropagateRenderErrors(t *testing.T) {
	failure := errors.New("render failed")
	items := Map([]int{1}, func(int) html.Component {
		return html.ComponentFunc(func(context.Context, *html.Context) error { return failure })
	})
	if err := html.Render(context.Background(), io.Discard, items); !errors.Is(err, failure) {
		t.Fatalf("render error = %v", err)
	}
}
