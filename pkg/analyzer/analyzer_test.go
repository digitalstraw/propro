package analyzer

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func setUp() string {
	ProtectedStructsMap = make(map[string]bool)
	EntityFile = ""
	Structs = []string{}

	path, _ := os.Getwd()
	testdata := filepath.Join(filepath.Dir(filepath.Dir(path)), "testdata")

	return testdata
}

func TestWithEntityFileParameter(t *testing.T) {
	testdata := setUp()

	cfg := map[string]any{
		entityListFileArg: filepath.Join(testdata, "src/config/entities.go"),
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
		entityListFileArg: filepath.Join(testdata, "src/config/entities.go"),
		structsArg:        []string{"Entity", "SubEntity"},
	}

	analysistest.Run(t, testdata, NewAnalyzer(cfg), "protectselected")
}

func TestWithEntityFileAndStructsComposed(t *testing.T) {
	testdata := setUp()
	cfg := map[string]any{
		// contains UnProtectedEntity to test that only specified structs are protected
		entityListFileArg: filepath.Join(testdata, "src/config2/entities.go"), // Entity
		structsArg:        []string{"SubEntity"},
	}

	// Test twice to simulate concurrent runs which reuse already set up configuration.
	analysistest.Run(t, testdata, NewAnalyzer(cfg), "protectselected")
	analysistest.Run(t, testdata, NewAnalyzer(cfg), "protectselected")
}

func TestWithEntityFileWhichDoesNotCompile(t *testing.T) {
	testdata := setUp()
	cfg := map[string]any{
		entityListFileArg: filepath.Join(testdata, "src/config3/entities.go.txt"),
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
	fs.String(entityListFileArg, "", "path to file containing list of entities")
	fs.String(structsArg, "", "comma separated list of structs to protect")

	_ = fs.Set(structsArg, "   Entity   ,   Entity2")
	_ = fs.Set(entityListFileArg, "      /path/to/file.go    ")
	flagSet = *fs

	tryInitFromCLI()

	if len(Structs) != 2 || Structs[0] != "Entity" || Structs[1] != "Entity2" {
		t.Errorf("tryInitFromCLI did not set Structs correctly, got: %v", Structs)
	}
	if EntityFile != "/path/to/file.go" {
		t.Errorf("tryInitFromCLI did not set EntityFile correctly, got: %s", EntityFile)
	}
}
