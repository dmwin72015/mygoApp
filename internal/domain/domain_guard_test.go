package domain

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDomainHasZeroExternalDependencies enforces Clean Architecture rule:
// the domain package MUST NOT depend on any external packages, frameworks,
// UI libraries (mygo/gin), or database drivers (pgx/sqlite).
func TestDomainHasZeroExternalDependencies(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("failed to glob files: %v", err)
	}

	fset := token.NewFileSet()
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}

		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("failed to read %s: %v", file, err)
		}

		node, err := parser.ParseFile(fset, file, src, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("failed to parse %s: %v", file, err)
		}

		for _, imp := range node.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			// Standard library packages do not contain dot in domain (e.g., github.com/...)
			if strings.Contains(path, ".") || strings.HasPrefix(path, "mygo-app/") {
				t.Errorf("violation in %s: domain must NOT import non-stdlib package %q", file, path)
			}
		}
	}
}
