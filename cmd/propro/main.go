package main

import (
	"github.com/digitalstraw/propro/v2/pkg/analyzer"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(analyzer.NewAnalyzer(map[string]any{}))
}
