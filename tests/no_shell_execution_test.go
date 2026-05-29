package tests

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestProductionGoCodeDoesNotExposeShellOrDirectPythonExecution(t *testing.T) {
	root := repositoryRoot(t)
	productionRoots := []string{
		filepath.Join(root, "service"),
		filepath.Join(root, "internal"),
	}

	for _, productionRoot := range productionRoots {
		err := filepath.WalkDir(productionRoot, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}

			assertNoShellOrPythonExecutionPath(t, root, path)
			return nil
		})
		if err != nil {
			t.Fatalf("scan production Go files: %v", err)
		}
	}
}

func assertNoShellOrPythonExecutionPath(t *testing.T, root, path string) {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	rel := relativePath(t, root, path)
	for _, imp := range file.Imports {
		importPath, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			t.Fatalf("parse import in %s: %v", rel, err)
		}
		if importPath == "os/exec" {
			t.Fatalf("%s imports os/exec; production services must not directly execute shell or python commands", rel)
		}
	}

	forbiddenLiterals := map[string]struct{}{
		"/bin/bash": {},
		"/bin/sh":   {},
		"bash":      {},
		"python":    {},
		"python3":   {},
		"sh":        {},
	}
	ast.Inspect(file, func(node ast.Node) bool {
		lit, ok := node.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		if _, forbidden := forbiddenLiterals[value]; forbidden {
			t.Fatalf("%s contains forbidden command literal %q", rel, value)
		}
		return true
	})
}

func repositoryRoot(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatal("could not find repository root with go.mod")
		}
		wd = parent
	}
}

func relativePath(t *testing.T, root, path string) string {
	t.Helper()

	rel, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatalf("make relative path for %s: %v", path, err)
	}
	return rel
}
