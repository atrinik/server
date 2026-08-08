package architecture

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestForbiddenDependencyEdges(t *testing.T) {
	t.Parallel()
	repository, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	forbiddenEverywhere := []string{"atrinik/classic", "atrinik/client", "atrinik/editor", "atrinik/renderer", "atrinik/content-toolkit", "python", "cpython"}
	forbiddenCore := []string{"database/sql", "modernc.org/sqlite", "google.golang.org/protobuf", "github.com/quic-go/quic-go", "/internal/observability"}
	err = filepath.WalkDir(repository, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "build" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "architecture_test.go") {
			return nil
		}
		parsed, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}
		for _, imported := range parsed.Imports {
			name, unquoteErr := strconv.Unquote(imported.Path.Value)
			if unquoteErr != nil {
				return unquoteErr
			}
			for _, denied := range forbiddenEverywhere {
				if strings.Contains(strings.ToLower(name), denied) {
					t.Errorf("%s imports forbidden dependency %q", path, name)
				}
			}
			if strings.Contains(path, string(filepath.Separator)+"internal"+string(filepath.Separator)+"kernel"+string(filepath.Separator)) || strings.Contains(path, string(filepath.Separator)+"internal"+string(filepath.Separator)+"domain"+string(filepath.Separator)) {
				if slices.ContainsFunc(forbiddenCore, func(denied string) bool { return strings.Contains(name, denied) }) {
					t.Errorf("core package %s imports adapter dependency %q", path, name)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
