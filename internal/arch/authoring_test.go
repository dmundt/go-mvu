package arch

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Only these demonstrations intentionally cross authoring levels. Framework
// component implementations remain free to stream HTML directly.
var lowLevelExamples = map[string]string{
	"examples/themes/lowlevel.go":     "class-free semantic HTML theme demonstration",
	"examples/components/lowlevel.go": "generic element/component interoperability demonstration",
}

func TestExampleAuthoringBoundaries(t *testing.T) {
	root := filepath.Join("..", "..")
	command := exec.Command("go", "list", "-export", "-deps", "-f", "{{.ImportPath}}|{{.Export}}", "./examples/...")
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("load example dependency exports: %v\n%s", err, output)
	}
	exports := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		pair := strings.SplitN(strings.TrimSpace(line), "|", 2)
		if len(pair) == 2 && pair[1] != "" {
			exports[pair[0]] = pair[1]
		}
	}
	fset := token.NewFileSet()
	dependencies := importer.ForCompiler(fset, "gc", func(path string) (io.ReadCloser, error) {
		export, ok := exports[path]
		if !ok {
			return nil, fmt.Errorf("missing export for %s", path)
		}
		return os.Open(export)
	})
	seen := make(map[string]bool)
	err = filepath.WalkDir(filepath.Join(root, "examples"), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		packages, err := parser.ParseDir(fset, path, func(info os.FileInfo) bool {
			return strings.HasSuffix(info.Name(), ".go") && !strings.HasSuffix(info.Name(), "_test.go")
		}, 0)
		if err != nil {
			return err
		}
		for _, pkg := range packages {
			var files []*ast.File
			for name, file := range pkg.Files {
				files = append(files, file)
				relative, err := filepath.Rel(root, name)
				if err != nil {
					return err
				}
				seen[filepath.ToSlash(relative)] = true
			}
			info := &types.Info{Uses: make(map[*ast.Ident]types.Object)}
			config := types.Config{Importer: dependencies}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			if _, err := config.Check(module+filepath.ToSlash(relative), fset, files, info); err != nil {
				return err
			}
			for _, file := range files {
				name, err := filepath.Rel(root, fset.Position(file.Pos()).Filename)
				if err != nil {
					return err
				}
				if _, exempt := lowLevelExamples[filepath.ToSlash(name)]; exempt {
					continue
				}
				for _, violation := range authoringViolations(file, fset, info) {
					t.Error(violation)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for path := range lowLevelExamples {
		if !seen[path] {
			t.Errorf("stale low-level example exception: %s", path)
		}
	}
}

func authoringViolations(file *ast.File, fset *token.FileSet, info *types.Info) []error {
	var violations []error
	ast.Inspect(file, func(node ast.Node) bool {
		identifier, ok := node.(*ast.Ident)
		if !ok {
			return true
		}
		object := info.Uses[identifier]
		if object == nil || object.Pkg() == nil || object.Pkg().Path() != module+"html" {
			return true
		}
		blocked := false
		switch object.Name() {
		case "El", "Element", "ComponentFunc", "NewContext", "Render",
			"TrustStatic", "TrustedHTML", "Trusted":
			blocked = true
		}
		if method, ok := object.(*types.Func); ok {
			signature := method.Type().(*types.Signature)
			if receiver := signature.Recv(); receiver != nil {
				receiverType := receiver.Type()
				if pointer, ok := receiverType.(*types.Pointer); ok {
					receiverType = pointer.Elem()
				}
				if named, ok := receiverType.(*types.Named); ok && named.Obj().Name() == "Context" {
					switch method.Name() {
					case "Component", "Components", "Err", "Fail":
					default:
						blocked = true
					}
				}
			}
		}
		if blocked {
			violations = append(violations, fmt.Errorf(
				"%s: low-level html.%s in composition example; compose a semantic component or isolate an explicit demo",
				fset.Position(identifier.Pos()), object.Name(),
			))
		}
		return true
	})
	return violations
}

type importFunc func(string) (*types.Package, error)

func (fn importFunc) Import(path string) (*types.Package, error) { return fn(path) }

func TestAuthoringCheckResolvesAliasesAndMethodReferences(t *testing.T) {
	fset := token.NewFileSet()
	stub, err := parser.ParseFile(fset, "html.go", `package html
type Context struct{}
func (*Context) Static(string) {}
func (*Context) Text(string) {}
func (*Context) Writer() {}
func (*Context) Component() {}
func (*Context) Err() {}
type Element struct{}
type ComponentFunc func()
func El() {}
func A() {}
`, 0)
	if err != nil {
		t.Fatal(err)
	}
	htmlPackage, err := (&types.Config{}).Check(module+"html", fset, []*ast.File{stub}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, source string
		want         int
	}{
		{"composition", `import h "github.com/dmundt/go-mvu/html"
func view(c *h.Context) { c.Component(); c.Err(); h.A() }`, 0},
		{"aliased receiver", `import h "github.com/dmundt/go-mvu/html"
func view(writer *h.Context) { writer.Static("x"); writer.Text("x") }`, 2},
		{"method values", `import h "github.com/dmundt/go-mvu/html"
var emit = (*h.Context).Static
func view(c *h.Context) { _ = c.Writer }`, 2},
		{"dot import", `import . "github.com/dmundt/go-mvu/html"
var element = El`, 1},
		{"type aliases", `import h "github.com/dmundt/go-mvu/html"
type Node = h.Element
type Adapter = h.ComponentFunc`, 2},
		{"unrelated method", `type Local struct{}
func (Local) Static(string) {}
func view(c Local) { c.Static("allowed") }`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file, err := parser.ParseFile(fset, "view.go", "package view\n"+tc.source, 0)
			if err != nil {
				t.Fatal(err)
			}
			info := &types.Info{Uses: make(map[*ast.Ident]types.Object)}
			config := types.Config{Importer: importFunc(func(path string) (*types.Package, error) {
				if path != htmlPackage.Path() {
					return nil, fmt.Errorf("unexpected import %s", path)
				}
				return htmlPackage, nil
			})}
			if _, err := config.Check("view", fset, []*ast.File{file}, info); err != nil {
				t.Fatal(err)
			}
			if got := authoringViolations(file, fset, info); len(got) != tc.want {
				t.Fatalf("got %v, want %d violations", got, tc.want)
			}
		})
	}
}
