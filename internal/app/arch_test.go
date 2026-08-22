package app

// Architecture boundary test for the hexagonal vertical-slice migration.
//
// Rules encoded here (see specs/003-hexagonal-vertical-slice/contracts/boundaries.md):
//  1. Feature packages under internal/ must not import each other.
//  2. Features must not import legacy shared paths (internal/config,
//     internal/db, internal/model, internal/password, internal/session,
//     internal/web).
//  3. Each data-owning feature must contain the expected slice files; home is
//     exempt from postgres_repository.go because it owns no persistence.
//  4. service.go and http_handler.go must not import database/sql or pgx;
//     postgres_repository.go is the only slice file allowed to.
//  5. Shared packages must not import feature packages.

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const modulePrefix = "github.com/leoarkiteto/stratum/"

var legacyShared = []string{
	"internal/config",
	"internal/db",
	"internal/model",
	"internal/password",
	"internal/session",
	"internal/web",
}

var featureNames = []string{"auth", "home"}

var expectedSliceFiles = map[string][]string{
	"auth": {"domain.go", "ports.go", "service.go", "http_handler.go", "postgres_repository.go", "module.go"},
	"home": {"domain.go", "ports.go", "service.go", "http_handler.go", "module.go"},
}

func TestArchitectureBoundaries(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	// Tests run with CWD = internal/app.
	root := strings.TrimSuffix(cwd, string(filepath.Separator)+"internal"+string(filepath.Separator)+"app")
	if root == cwd {
		t.Fatalf("unexpected working dir %q (want .../internal/app)", cwd)
	}

	checkFeatureFiles(t, root)
	checkFeatureImports(t, root)
	checkSharedImports(t, root)
}

func checkFeatureFiles(t *testing.T, root string) {
	t.Helper()
	for _, feature := range featureNames {
		dir := filepath.Join(root, "internal", feature)
		for _, f := range expectedSliceFiles[feature] {
			if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
				t.Errorf("feature %s: missing required slice file %s", feature, f)
			}
		}
	}
}

func checkFeatureImports(t *testing.T, root string) {
	t.Helper()
	for _, feature := range featureNames {
		dir := filepath.Join(root, "internal", feature)
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			imports := parseImports(t, path)
			for _, imp := range imports {
				// Feature must not import another feature.
				for _, other := range featureNames {
					if other == feature {
						continue
					}
					if imp == modulePrefix+"internal/"+other {
						t.Errorf("%s: imports another feature %s", rel(root, path), other)
					}
				}
				// Feature must not import legacy shared paths.
				for _, legacy := range legacyShared {
					if imp == modulePrefix+legacy {
						t.Errorf("%s: imports legacy shared path %s", rel(root, path), legacy)
					}
				}
			}

			// SQL isolation: only postgres_repository.go may import database/sql or pgx.
			base := filepath.Base(path)
			sqlImport := func(imp string) bool {
				return imp == "database/sql" || strings.HasPrefix(imp, "github.com/jackc/pgx")
			}
			if sqlImportAny(imports, sqlImport) {
				if base != "postgres_repository.go" {
					t.Errorf("%s: imports database/sql or pgx (only postgres_repository.go may)", rel(root, path))
				}
			} else if base == "postgres_repository.go" {
				t.Errorf("%s: postgres_repository.go must import database/sql or pgx", rel(root, path))
			}
			return nil
		})
		if err != nil {
			t.Errorf("walk %s: %v", dir, err)
		}
	}
}

func checkSharedImports(t *testing.T, root string) {
	t.Helper()
	sharedDir := filepath.Join(root, "internal", "shared")
	if _, err := os.Stat(sharedDir); err != nil {
		return // shared/ does not exist yet (migration in progress)
	}
	err := filepath.WalkDir(sharedDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		for _, imp := range parseImports(t, path) {
			for _, feature := range featureNames {
				if imp == modulePrefix+"internal/"+feature || strings.HasPrefix(imp, modulePrefix+"internal/"+feature+"/") {
					t.Errorf("%s: shared package must not import feature %s", rel(root, path), feature)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Errorf("walk %s: %v", sharedDir, err)
	}
}

func parseImports(t *testing.T, path string) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		t.Errorf("parse %s: %v", path, err)
		return nil
	}
	var out []string
	for _, imp := range f.Imports {
		if imp.Path != nil {
			out = append(out, strings.Trim(imp.Path.Value, `"`))
		}
	}
	return out
}

func sqlImportAny(imports []string, matches func(string) bool) bool {
	for _, imp := range imports {
		if matches(imp) {
			return true
		}
	}
	return false
}

func rel(root, path string) string {
	r, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return r
}
