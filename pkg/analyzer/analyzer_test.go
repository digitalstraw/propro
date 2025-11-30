package analyzer

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func setUp() string {
	ProtectedStructsMap = make(map[string]bool)
	EntityFile = ""
	Structs = []string{}
	SkipTests = false

	path, _ := os.Getwd()
	testdata := filepath.Join(filepath.Dir(filepath.Dir(path)), "testdata")

	return testdata
}

func TestWithEntityFileParameter(t *testing.T) {
	testdata := setUp()

	cfg := map[string]any{
		"entityListFile": filepath.Join(testdata, "src/config/entities.go"),
	}

	analysistest.Run(t, testdata, NewAnalyzer(cfg), "protectselected")
}

func TestWithStructsParameter(t *testing.T) {
	testdata := setUp()
	cfg := map[string]any{
		"structs": []string{"Entity", "SubEntity"},
	}

	analysistest.Run(t, testdata, NewAnalyzer(cfg), "protectselected")
}

func TestWithEntityFileAndStructs(t *testing.T) {
	testdata := setUp()
	cfg := map[string]any{
		"entityListFile": filepath.Join(testdata, "src/config/entities.go"),
		"structs":        []string{"Entity", "SubEntity"},
	}

	analysistest.Run(t, testdata, NewAnalyzer(cfg), "protectselected")
}

func TestWithNoParameters_allStructsAreProtected(t *testing.T) {
	testdata := setUp()

	analysistest.Run(t, testdata, NewAnalyzer(map[string]any{}), "protectall")
}
