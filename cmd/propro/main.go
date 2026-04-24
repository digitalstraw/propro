package main

import (
	"github.com/digitalstraw/propro/v3/pkg/analyzer"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(analyzer.NewAnalyzer(map[string]any{}))
}
