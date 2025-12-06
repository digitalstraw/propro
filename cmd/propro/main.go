package main

import (
	"os"

	"github.com/digitalstraw/propro/pkg/analyzer"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	analyzer.CliInit(os.Args[1:])
	singlechecker.Main(analyzer.NewAnalyzer(map[string]any{}))
}
