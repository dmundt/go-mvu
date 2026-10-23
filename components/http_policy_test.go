package components

import (
	"context"
	"encoding/json"
	stdhtml "html"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/html"
)

func TestDocumentHTMXValidationResponseHandling(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		var out strings.Builder
		if err := html.Render(context.Background(), &out, Document{
			HTMXSwapValidationErrors: enabled,
			HTMXSource:               "/static/htmx.min.js",
		}); err != nil {
			t.Fatal(err)
		}
		prefix := `<meta name="htmx-config" content="`
		_, after, found := strings.Cut(out.String(), prefix)
		if found != enabled {
			t.Fatalf("configuration presence = %v, want %v", found, enabled)
		}
		if !enabled {
			continue
		}
		content, _, _ := strings.Cut(after, `"`)
		var configuration struct {
			ResponseHandling []struct {
				Code  string `json:"code"`
				Swap  bool   `json:"swap"`
				Error bool   `json:"error"`
			} `json:"responseHandling"`
		}
		if err := json.Unmarshal([]byte(stdhtml.UnescapeString(content)), &configuration); err != nil {
			t.Fatal(err)
		}
		rules := configuration.ResponseHandling
		if len(rules) != 5 || rules[0].Code != "204" || rules[0].Swap ||
			rules[1].Code != "422" || !rules[1].Swap || rules[1].Error ||
			rules[3].Code != "[45].." || rules[3].Swap || !rules[3].Error {
			t.Fatalf("rules = %+v", rules)
		}
		if strings.Index(out.String(), prefix) > strings.Index(out.String(), "<script") {
			t.Fatal("HTMX loaded before configuration")
		}
	}
}
