package arch

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const module = "github.com/dmundt/go-mvu/"

// allowed lists permitted in-module imports per package (non-test files).
// Spec: docs/specs/gomvu-chat-spec.md §2.
var allowed = map[string][]string{
	"mvu":        {},
	"html":       {},
	"store":      {},
	"theme":      {},
	"static":     {},
	"htmx":       {"html"},
	"session":    {"store"},
	"validation": {},
	"middleware": {"html", "session"},
	"components": {"html", "htmx", "theme"},
	"shared":     {"components", "html", "htmx"},
	"web":        {"mvu", "html", "htmx", "store", "theme"},
}

func TestPackageBoundaries(t *testing.T) {
	root := filepath.Join("..", "..")
	for pkg, permitted := range allowed {
		dir := filepath.Join(root, pkg)
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("%s: %v", pkg, err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, name), nil, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			for _, spec := range file.Imports {
				path := strings.Trim(spec.Path.Value, `"`)
				if path == "unsafe" || path == "reflect" && (pkg == "html" || pkg == "components") {
					t.Errorf("%s/%s imports %s", pkg, name, path)
				}
				if path == "html/template" || path == "text/template" {
					t.Errorf("%s/%s imports template engine %s", pkg, name, path)
				}
				if !strings.HasPrefix(path, module) {
					continue
				}
				target := strings.TrimPrefix(path, module)
				if !slices.Contains(permitted, target) {
					t.Errorf("%s/%s imports %s; allowed %v", pkg, name, target, permitted)
				}
			}
		}
	}
}
