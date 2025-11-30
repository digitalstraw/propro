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
	EntityFile = filepath.Join(testdata, "src/config/entities.go")

	analysistest.Run(t, testdata, NewAnalyzer(), "protectselected")
}

func TestWithStructsParameter(t *testing.T) {
	testdata := setUp()
	Structs = []string{"Entity", "SubEntity"}

	analysistest.Run(t, testdata, NewAnalyzer(), "protectselected")
}

func TestWithEntityFileAndStructs(t *testing.T) {
	testdata := setUp()
	EntityFile = filepath.Join(testdata, "src/config2/entities.go")
	Structs = []string{"SubEntity"}

	analysistest.Run(t, testdata, NewAnalyzer(), "protectselected")
}

func TestWithNoParameters_allStructsAreProtected(t *testing.T) {
	testdata := setUp()

	analysistest.Run(t, testdata, NewAnalyzer(), "protectall")
}
