package analyzer

import (
	"flag"
	"fmt"
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
	seen       map[string]bool
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
		ProtectedStructsMap = loadEntityList(EntityFile)
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
	seen = make(map[string]bool)

	createProtectedStructsMap()

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// Catch allways a field can be mutated
	nodeFilter := []ast.Node{
		(*ast.AssignStmt)(nil),   // =, +=, -=, etc.
		(*ast.IncDecStmt)(nil),   // ++, --
		(*ast.StarExpr)(nil),     // *( &x.f ) = ...
		(*ast.SelectorExpr)(nil), // context
	}

	insp.Preorder(nodeFilter, func(n ast.Node) {
		switch node := n.(type) {

		// ---------------------------------------
		// Case 1: direct or compound assignment
		// ---------------------------------------
		case *ast.AssignStmt:
			for _, lhs := range node.Lhs {
				sel := unwrapSelectorExpr(lhs)
				if sel != nil {
					handleSelectorMutation(pass, sel)
				}
			}

		// ---------------------------------------
		// Case 2: e.Field++ or e.Field-- etc.
		// ---------------------------------------
		case *ast.IncDecStmt:
			sel := unwrapSelectorExpr(node.X)
			if sel != nil {
				handleSelectorMutation(pass, sel)
			}

		// ---------------------------------------
		// Case 3: *( &e.Field ) = ... or *(&e.Field)++ / --
		// ---------------------------------------
		case *ast.StarExpr:
			sel := unwrapSelectorExpr(node)
			if sel != nil {
				handleSelectorMutation(pass, sel)
			}
		}
	})

	return nil, nil
}

// --- Helpers ---

func loadEntityList(filePath string) map[string]bool {
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

func isEmbeddedField(pass *analysis.Pass, selector *ast.SelectorExpr, structName, fieldName string) bool {
	// Detect if fieldName is an embedded field of structName

	typ := pass.TypesInfo.TypeOf(selector.X)
	if typ == nil {
		return false
	}

	// unwrap pointer
	if p, ok := typ.(*types.Pointer); ok {
		typ = p.Elem()
	}

	named, ok := typ.(*types.Named)
	if !ok {
		return false
	}

	st, ok := named.Underlying().(*types.Struct)
	if !ok {
		return false
	}

	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)

		// embedded field must have no name OR f.Name() == type name
		if f.Embedded() && f.Name() == fieldName {
			return true
		}
	}

	return false
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

func handleSelectorMutation(pass *analysis.Pass, selector *ast.SelectorExpr) {
	if pass.TypesInfo == nil || selector == nil {
		return
	}

	fieldName := selector.Sel.Name
	if !ast.IsExported(fieldName) {
		return
	}

	typ := pass.TypesInfo.TypeOf(selector.X)
	if typ == nil {
		return
	}

	// unwrap pointer
	for {
		ptr, ok := typ.(*types.Pointer)
		if !ok {
			break
		}
		typ = ptr.Elem()
	}

	named, ok := typ.(*types.Named)
	if !ok {
		return
	}

	structName := named.Obj().Name()

	// skip non-protected structs
	if !protectAllStructs && !ProtectedStructsMap[structName] {
		return
	}

	// skip embedded field itself
	if isEmbeddedField(pass, selector, structName, fieldName) {
		return
	}

	// skip assignments inside struct methods
	if enclosing := findEnclosingFunc(pass, selector.Pos()); enclosing != nil && enclosing.Recv != nil {
		for _, recv := range enclosing.Recv.List {
			recvType := pass.TypesInfo.TypeOf(recv.Type)
			if isSameStructType(recvType, structName) {
				return
			}
		}
	}

	// REPORT forbidden mutation
	reportIssue(pass, selector.Pos(), structName, fieldName)
}

func reportIssue(pass *analysis.Pass, pos token.Pos, structName, fieldName string) {
	key := structName + "." + fieldName + "." + fmt.Sprint(pos)
	if seen[key] {
		return
	}
	pass.Reportf(pos, "assignment to exported field %s.%s is forbidden outside its methods", structName, fieldName)
	seen[key] = true
}

// unwrapSelectorExpr recursively unwraps *(&...selector...) and returns the SelectorExpr if present
func unwrapSelectorExpr(expr ast.Expr) *ast.SelectorExpr {
	switch e := expr.(type) {
	case *ast.ParenExpr:
		return unwrapSelectorExpr(e.X)
	case *ast.SelectorExpr:
		return e
	case *ast.StarExpr:
		return unwrapSelectorExpr(e.X)
	case *ast.UnaryExpr:
		// & operator
		if e.Op == token.AND {
			return unwrapSelectorExpr(e.X)
		} else {
			return nil
		}
	default:
		return nil
	}
}
