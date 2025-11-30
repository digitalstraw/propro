package analyzer

import (
	"flag"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

var (
	EntityFile string
	Structs    []string
	SkipTests  bool
)

// ProtectedStructsMap stores the whitelist of struct names
var (
	ProtectedStructsMap = map[string]bool{}
	protectAllStructs   = false
	flagSet             flag.FlagSet
)

func init() {
	flagSet.StringVar(&EntityFile, "entityListFile", "", "Path to the Go source file defining the list of protected structs")

	strstr := ""
	flagSet.StringVar(&strstr, "structs", "", "Comma-separated list of struct names to protect (alternative to entityListFile)")

	if strstr != "" {
		for _, structName := range strings.Split(strstr, ",") {
			structName = strings.TrimSpace(structName)
			Structs = append(Structs, structName)
		}
	}

	flagSet.BoolVar(&SkipTests, "skipTests", false, "Skip analysis of test files")
}

func NewAnalyzer(cfg map[string]any) *analysis.Analyzer {
	if entityListFile, ok := cfg["entityListFile"].(string); ok && entityListFile != "" {
		EntityFile = entityListFile
	}

	if structs, ok := cfg["structs"].([]string); ok && structs != nil {
		for _, structName := range structs {
			structName = strings.TrimSpace(structName)
			Structs = append(Structs, structName)
		}
	}
	if skipTests, ok := cfg["skipTests"].(bool); ok {
		SkipTests = skipTests
	}

	return &analysis.Analyzer{
		Name: "propro",
		Doc:  "detects assignments to exported fields of protected structs outside of methods",
		Requires: []*analysis.Analyzer{
			inspect.Analyzer,
		},
		Flags: flagSet,
		Run:   run,
	}
}

func createProtectedStructsMap() {
	if EntityFile != "" {
		ProtectedStructsMap = LoadEntityList(EntityFile)
	}

	for _, structName := range Structs {
		structName = strings.TrimSpace(structName)
		ProtectedStructsMap[structName] = true
	}

	if len(ProtectedStructsMap) == 0 {
		protectAllStructs = true
	}
}

func run(pass *analysis.Pass) (interface{}, error) {
	createProtectedStructsMap()

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.AssignStmt)(nil),
		(*ast.SelectorExpr)(nil),
	}

	insp.Preorder(nodeFilter, func(n ast.Node) {
		switch node := n.(type) {

		case *ast.AssignStmt:
			for _, lhs := range node.Lhs {
				selector, ok := lhs.(*ast.SelectorExpr)
				if !ok {
					continue
				}

				if pass.TypesInfo == nil {
					continue
				}

				typ := pass.TypesInfo.TypeOf(selector.X)
				if typ == nil {
					continue
				}

				named, ok := typ.(*types.Named)
				if !ok {
					ptr, ok := typ.(*types.Pointer)
					if !ok {
						continue
					}
					named, ok = ptr.Elem().(*types.Named)
					if !ok {
						continue
					}
				}

				structName := named.Obj().Name()
				if !protectAllStructs && !ProtectedStructsMap[structName] {
					continue
				}

				field := selector.Sel.Name
				if !ast.IsExported(field) {
					continue
				}

				// skip assignments inside struct methods
				if enclosing := findEnclosingFunc(pass, node.Pos()); enclosing != nil && enclosing.Recv != nil {
					for _, recv := range enclosing.Recv.List {
						recvType := pass.TypesInfo.TypeOf(recv.Type)
						if isSameStructType(recvType, structName) {
							return
						}
					}
				}

				// report forbidden assignment
				pass.Reportf(lhs.Pos(),
					"assignment to exported field %s.%s is forbidden outside its methods",
					structName, field)
			}
		}
	})
	return nil, nil
}

// --- Helpers ---

// Find the FuncDecl enclosing a given position
func findEnclosingFunc(pass *analysis.Pass, pos token.Pos) *ast.FuncDecl {
	for _, file := range pass.Files {
		var enclosing *ast.FuncDecl
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				return true
			}
			f, ok := n.(*ast.FuncDecl)
			if !ok {
				return true
			}
			if f.Pos() <= pos && pos <= f.End() {
				enclosing = f
			}
			return true
		})
		if enclosing != nil {
			return enclosing
		}
	}
	return nil
}

// Check if a types.Type represents the same struct (unwrap pointer if needed)
func isSameStructType(typ types.Type, structName string) bool {
	for {
		ptr, ok := typ.(*types.Pointer)
		if !ok {
			break
		}
		typ = ptr.Elem()
	}

	if named, ok := typ.(*types.Named); ok {
		return named.Obj().Name() == structName
	}
	return false
}

func LoadEntityList(filePath string) map[string]bool {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, nil, parser.AllErrors)
	if err != nil {
		return nil
	}

	entities := map[string]bool{}

	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}

		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			if vs.Names[0].Name != "EntityList" {
				continue
			}

			for _, val := range vs.Values {
				sliceLit, ok := val.(*ast.CompositeLit)
				if !ok {
					continue
				}

				for _, elt := range sliceLit.Elts {
					var typName string

					switch e := elt.(type) {
					case *ast.UnaryExpr: // &Entity{}
						cl, ok := e.X.(*ast.CompositeLit)
						if !ok || cl.Type == nil {
							continue
						}
						typName = extractTypeName(cl.Type)
					case *ast.CompositeLit: // Entity{}
						if e.Type == nil {
							continue
						}
						typName = extractTypeName(e.Type)
					}

					if typName != "" {
						entities[typName] = true
					}
				}
			}
		}
	}

	return entities
}

func extractTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return t.Sel.Name
	case *ast.StarExpr:
		switch x := t.X.(type) {
		case *ast.Ident:
			return x.Name
		case *ast.SelectorExpr:
			return x.Sel.Name
		}
	}
	return ""
}
