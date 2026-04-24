package analyzer

import (
	"flag"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func setUp() string {
	ProtectedStructsMap = make(map[string]bool)
	protectAllStructs = false
	EntityFiles = nil
	Structs = []string{}
	initOnce = sync.Once{}

	path, _ := os.Getwd()
	testdata := filepath.Join(filepath.Dir(filepath.Dir(path)), "testdata")

	return testdata
}

func TestWithEntityFileParameter(t *testing.T) {
	testdata := setUp()

	cfg := map[string]any{
		entityListFilesArg: []string{filepath.Join(testdata, "src/config/entities.go")},
	}

	analysistest.Run(t, testdata, NewAnalyzer(cfg), "protectselected")
}

func TestWithStructsParameter(t *testing.T) {
	testdata := setUp()
	cfg := map[string]any{
		// contains UnProtectedEntity to test that only specified structs are protected
		structsArg: []string{"Entity", "SubEntity"},
	}

	analysistest.Run(t, testdata, NewAnalyzer(cfg), "protectselected")
}

func TestWithEntityFileAndStructsWithOverlap(t *testing.T) {
	testdata := setUp()
	cfg := map[string]any{
		// contains UnProtectedEntity to test that only specified structs are protected
		entityListFilesArg: []string{filepath.Join(testdata, "src/config/entities.go")},
		structsArg:         []string{"Entity", "SubEntity"},
	}

	analysistest.Run(t, testdata, NewAnalyzer(cfg), "protectselected")
}

func TestWithEntityFileAndStructsComposed(t *testing.T) {
	testdata := setUp()
	cfg := map[string]any{
		// contains UnProtectedEntity to test that only specified structs are protected
		entityListFilesArg: []string{filepath.Join(testdata, "src/config2/entities.go")}, // Entity
		structsArg:         []string{"SubEntity"},
	}

	// Test twice to simulate concurrent runs which reuse already set up configuration.
	analysistest.Run(t, testdata, NewAnalyzer(cfg), "protectselected")
	analysistest.Run(t, testdata, NewAnalyzer(cfg), "protectselected")
}

func TestWithMultipleEntityFiles(t *testing.T) {
	testdata := setUp()
	cfg := map[string]any{
		entityListFilesArg: []string{
			filepath.Join(testdata, "src/config/entities.go"),  // Entity, SubEntity from protectall
			filepath.Join(testdata, "src/config2/entities.go"), // Entity from protectselected
		},
	}

	analysistest.Run(t, testdata, NewAnalyzer(cfg), "protectselected")
}

func TestWithNonExistentEntityFile(t *testing.T) {
	testdata := setUp()
	cfg := map[string]any{
		entityListFilesArg: []string{
			filepath.Join(testdata, "src/does_not_exist.go"),
			filepath.Join(testdata, "src/config/entities.go"),
		},
	}

	analysistest.Run(t, testdata, NewAnalyzer(cfg), "protectselected")
}

func TestWithEmptyAndWhitespaceEntityFilePaths_fallsBackToAllStructs(t *testing.T) {
	testdata := setUp()

	// An entity-list-files entry that is empty or whitespace must not be treated as "provided";
	// with no real paths and no structs, the linter falls back to protecting all structs.
	cfg := map[string]any{
		entityListFilesArg: []string{"", "   "},
	}

	analysistest.Run(t, testdata, NewAnalyzer(cfg), "protectall")
}

func TestTryInitFromCfg_WithAnySliceOfEntityFiles(t *testing.T) {
	testdata := setUp()

	// golangci-lint may decode YAML list values as []any rather than []string.
	cfg := map[string]any{
		entityListFilesArg: []any{
			filepath.Join(testdata, "src/config/entities.go"),
			"",
			123, // non-string entries are ignored
		},
	}

	analysistest.Run(t, testdata, NewAnalyzer(cfg), "protectselected")
}

func TestWithEntityFileWhichDoesNotCompile(t *testing.T) {
	testdata := setUp()
	cfg := map[string]any{
		entityListFilesArg: []string{filepath.Join(testdata, "src/config3/entities.go.txt")},
		structsArg: []string{
			"UnProtectedEntity", "Entity", "SubEntity", "Entity2", "SubEntity2", "SubSubEntity2",
			"Entity3", "SubEntity3", "SubSubEntity3", "Entity4", "SubEntity4", "SubSubEntity4",
			"RepositoryImpl", "SubEntityWithPtrComposition", "SubEntityWithComposition",
		},
	}

	analysistest.Run(t, testdata, NewAnalyzer(cfg), "protectall")
}

func TestWithNoParameters_allStructsAreProtected(t *testing.T) {
	testdata := setUp()

	// UnProtectedEntity WILL also be protected in this test
	analysistest.Run(t, testdata, NewAnalyzer(map[string]any{}), "protectall")
}

func TestTryInitFromCLI(t *testing.T) {
	_ = setUp()

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String(entityListFilesArg, "", "comma-separated list of paths to files containing list of entities")
	fs.String(structsArg, "", "comma separated list of structs to protect")

	_ = fs.Set(structsArg, "   Entity   ,   Entity2")
	_ = fs.Set(entityListFilesArg, "   /path/to/file1.go  ,   /path/to/file2.go   ")
	flagSet = *fs

	tryInitFromCLI()

	if len(Structs) != 2 || Structs[0] != "Entity" || Structs[1] != "Entity2" {
		t.Errorf("tryInitFromCLI did not set Structs correctly, got: %v", Structs)
	}
	if len(EntityFiles) != 2 || EntityFiles[0] != "/path/to/file1.go" || EntityFiles[1] != "/path/to/file2.go" {
		t.Errorf("tryInitFromCLI did not set EntityFiles correctly, got: %v", EntityFiles)
	}
}
