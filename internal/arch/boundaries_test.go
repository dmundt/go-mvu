package arch

import (
	"fmt"
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
	"mvu":                        {},
	"html":                       {},
	"store":                      {},
	"theme":                      {},
	"internal/arch":              {},
	"internal/browsertest":       {},
	"internal/example/jsonstore": {"store"},
	"internal/example/server":    {"web"},
	"htmx":                       {"html"},
	"components":                 {"html", "htmx", "theme", "web"},
	"web":                        {"mvu", "html", "htmx", "store", "theme"},
}

func TestPublicFrameworkPackages(t *testing.T) {
	root := filepath.Join("..", "..")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var packages []string
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") ||
			slices.Contains([]string{"benchmarks", "examples", "internal", "node_modules"}, entry.Name()) {
			continue
		}
		files, err := os.ReadDir(filepath.Join(root, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".go") && !strings.HasSuffix(file.Name(), "_test.go") {
				packages = append(packages, entry.Name())
				break
			}
		}
	}
	want := []string{"components", "html", "htmx", "mvu", "store", "theme", "web"}
	if !slices.Equal(packages, want) {
		t.Fatalf("public framework packages = %v, want %v", packages, want)
	}
}

func TestPackageBoundaries(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, err := range packageViolations(root) {
		t.Error(err)
	}
}

func TestWebCannotImportComponents(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "web")
	if err := os.MkdirAll(directory, 0755); err != nil {
		t.Fatal(err)
	}
	source := "package web\nimport _ \"" + module + "components\"\n"
	if err := os.WriteFile(filepath.Join(directory, "cycle.go"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	if found := packageViolations(root); len(found) != 1 || !strings.Contains(found[0].Error(), "imports components") {
		t.Fatalf("web to components dependency violations = %v", found)
	}
}

func packageViolations(root string) []error {
	var violations []error
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != root && (strings.HasPrefix(entry.Name(), ".") ||
				entry.Name() == "node_modules" || entry.Name() == "testdata" ||
				relative == "examples" || relative == "benchmarks") {
				return filepath.SkipDir
			}
			return nil
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		pkg := filepath.ToSlash(filepath.Dir(relative))
		permitted, declared := allowed[pkg]
		if !declared {
			violations = append(violations, fmt.Errorf("%s: production package needs a boundary declaration", relative))
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range file.Imports {
			path := strings.Trim(spec.Path.Value, `"`)
			if path == "unsafe" || path == "reflect" && (pkg == "html" || pkg == "components") {
				violations = append(violations, fmt.Errorf("%s/%s imports %s", pkg, name, path))
			}
			if path == "html/template" || path == "text/template" {
				violations = append(violations, fmt.Errorf("%s/%s imports template engine %s", pkg, name, path))
			}
			if !strings.HasPrefix(path, module) {
				continue
			}
			target := strings.TrimPrefix(path, module)
			if !slices.Contains(permitted, target) {
				violations = append(violations, fmt.Errorf("%s/%s imports %s; allowed %v", pkg, name, target, permitted))
			}
		}
		return nil
	})
	if err != nil {
		violations = append(violations, err)
	}
	return violations
}

func TestPackageCoverageIncludesNestedAndNewPackages(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"html/nested/render.go", "newpkg/api.go", "html/forbidden.go"} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("package fixture\nimport _ \"html/template\"\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	full := filepath.Join(root, "web", "example.go")
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("package web\nimport _ \""+module+"internal/example/server\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	found := packageViolations(root)
	if len(found) != 4 {
		t.Fatalf("got %v, want two undeclared packages, a template import, and an example helper import", found)
	}
}
