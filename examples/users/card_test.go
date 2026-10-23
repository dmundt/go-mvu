package main

import (
	"context"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/html"
)

func TestCardEscapesDynamicContent(t *testing.T) {
	var output strings.Builder
	err := html.Render(context.Background(), &output, UserCard{
		Name:  `<script>alert("x")</script>`,
		Email: `person@example.test&other`,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		`&lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt;`,
		`person@example.test&amp;other`,
	} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("rendered card missing escaped value %q: %s", expected, output.String())
		}
	}
	if strings.Contains(output.String(), "<script>") {
		t.Fatalf("rendered markup contains unescaped user content: %s", output.String())
	}
}
