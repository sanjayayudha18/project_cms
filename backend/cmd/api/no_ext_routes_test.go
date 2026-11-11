package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Property 13 (ext-user-api): the /api/ext routes live only in backend-extapi.
// Scans non-test Go files of backend/ and backend-exq/ for the path prefix.
func TestNoExtRoutesInBackendOrExq(t *testing.T) {
	const forbidden = "/api/" + "ext"
	roots := []string{filepath.Join("..", ".."), filepath.Join("..", "..", "..", "backend-exq")}

	for _, root := range roots {
		if _, err := os.Stat(root); err != nil {
			t.Fatalf("scan root %s: %v", root, err)
		}
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if n := d.Name(); n == "vendor" || n == "node_modules" || n == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(string(b), forbidden) {
				t.Errorf("%s contains %q; ext routes belong only in backend-extapi", path, forbidden)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
}
