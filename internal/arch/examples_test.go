package arch

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExamplesImportOnlySharedExampleHelpers(t *testing.T) {
	for _, directory := range []string{"examples", filepath.Join("internal", "example")} {
		for _, err := range exampleImportViolations(filepath.Join("..", "..", directory)) {
			t.Error(err)
		}
	}
}

func exampleImportViolations(root string) []error {
	var violations []error
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), ".") || entry.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range file.Imports {
			importPath := strings.Trim(spec.Path.Value, `"`)
			if strings.HasPrefix(importPath, module+"examples/") {
				violations = append(violations, fmt.Errorf("%s imports another example: %s", path, importPath))
			}
		}
		return nil
	})
	if err != nil {
		violations = append(violations, err)
	}
	return violations
}

func TestExampleImportRuleAllowsPrivateHelpers(t *testing.T) {
	root := t.TempDir()
	source := "package fixture\nimport _ \"" + module + "internal/example/jsonstore\"\n"
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	if got := exampleImportViolations(root); len(got) != 0 {
		t.Fatalf("private helper imports must be allowed: %v", got)
	}
}

func TestExampleImportRuleIncludesNestedModulesAndTests(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"production/auth.go", "durable/main_test.go"} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		source := "package fixture\nimport _ \"" + module + "examples/other/helper\"\n"
		if err := os.WriteFile(path, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if got := exampleImportViolations(root); len(got) != 2 {
		t.Fatalf("violations = %v, want two cross-example imports", got)
	}
}
