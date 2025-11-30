package main

import (
	"github.com/digitalstraw/propro/pkg/analyzer"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(analyzer.NewAnalyzer(map[string]any{}))
}
