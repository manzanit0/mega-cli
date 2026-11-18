package remote

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// usesMega reports whether a type expression references the go-mega package.
func usesMega(n ast.Node) bool {
	found := false
	ast.Inspect(n, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "mega" {
				found = true
			}
		}
		return !found
	})
	return found
}

// exportedRecv reports whether a method receiver is an exported type.
func exportedRecv(f *ast.FuncDecl) bool {
	if f.Recv == nil || len(f.Recv.List) == 0 {
		return true
	}
	t := f.Recv.List[0].Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	id, ok := t.(*ast.Ident)
	return ok && id.IsExported()
}

// TestNoGoMegaInPublicAPI checks that no exported function, method, field
// or typed variable of this package mentions a go-mega type, so callers
// never need to import go-mega.
func TestNoGoMegaInPublicAPI(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, d := range file.Decls {
				checkDecl(t, fset, d)
			}
		}
	}
}

// checkDecl reports exported declarations that use go-mega types.
func checkDecl(t *testing.T, fset *token.FileSet, d ast.Decl) {
	t.Helper()
	switch d := d.(type) {
	case *ast.FuncDecl:
		if d.Name.IsExported() && exportedRecv(d) && usesMega(d.Type) {
			t.Errorf("%s: %s exposes a go-mega type", fset.Position(d.Pos()), d.Name)
		}
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			checkSpec(t, fset, spec)
		}
	}
}

// checkSpec reports exported types and variables that use go-mega types.
func checkSpec(t *testing.T, fset *token.FileSet, spec ast.Spec) {
	t.Helper()
	switch s := spec.(type) {
	case *ast.TypeSpec:
		if !s.Name.IsExported() {
			return
		}
		st, ok := s.Type.(*ast.StructType)
		if !ok {
			if usesMega(s.Type) {
				t.Errorf("%s: type %s exposes a go-mega type", fset.Position(s.Pos()), s.Name)
			}
			return
		}
		for _, f := range st.Fields.List {
			for _, n := range f.Names {
				if n.IsExported() && usesMega(f.Type) {
					t.Errorf("%s: field %s.%s exposes a go-mega type",
						fset.Position(f.Pos()), s.Name, n)
				}
			}
		}
	case *ast.ValueSpec:
		for _, n := range s.Names {
			if n.IsExported() && s.Type != nil && usesMega(s.Type) {
				t.Errorf("%s: %s exposes a go-mega type", fset.Position(s.Pos()), n)
			}
		}
	}
}

// TestOnlyRemoteImportsGoMega checks go-mega stays behind this package.
func TestOnlyRemoteImportsGoMega(t *testing.T) {
	out, err := exec.Command("go", "list", "-f", "{{.ImportPath}} {{.Imports}}",
		"github.com/manzanit0/mega-cli/pkg/...").Output()
	if err != nil {
		t.Skipf("go list unavailable: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		pkg, imports, _ := strings.Cut(line, " ")
		if strings.HasSuffix(pkg, "/pkg/remote") {
			continue
		}
		if strings.Contains(imports, "go-mega") {
			t.Errorf("%s imports go-mega", pkg)
		}
	}
}
