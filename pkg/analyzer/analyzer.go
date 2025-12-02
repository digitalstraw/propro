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

	ProtectedStructsMap = map[string]bool{}
	protectAllStructs   bool
	seen                map[string]bool

	flagSet flag.FlagSet
)

func init() {
	flagSet.StringVar(&EntityFile, "entityListFile", "", "Path to file listing protected structs")
	flagSet.BoolVar(&SkipTests, "skipTests", false, "Skip test files")
	flagSet.String("structs", "", "Comma-separated list of protected structs")
}

func NewAnalyzer(cfg map[string]any) *analysis.Analyzer {
	if v, ok := cfg["entityListFile"].(string); ok && v != "" {
		EntityFile = v
	}
	if v, ok := cfg["structs"].([]string); ok && len(v) > 0 {
		Structs = append(Structs, v...)
	}
	if v, ok := cfg["skipTests"].(bool); ok {
		SkipTests = v
	}

	return &analysis.Analyzer{
		Name:     "propro",
		Doc:      "Detects writes to exported fields of protected structs outside methods",
		Requires: []*analysis.Analyzer{inspect.Analyzer},
		Flags:    flagSet,
		Run:      run,
	}
}

func run(pass *analysis.Pass) (interface{}, error) {
	seen = make(map[string]bool)
	buildProtectedStructMap()

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	insp.Preorder([]ast.Node{
		(*ast.AssignStmt)(nil),
		(*ast.IncDecStmt)(nil),
		(*ast.StarExpr)(nil),
	}, func(n ast.Node) {
		sel := extractSelectorFromNode(n)
		if sel != nil {
			handleSelectorMutation(pass, sel)
		}
	})

	return nil, nil
}

func buildProtectedStructMap() {
	if EntityFile != "" {
		for k := range loadEntityList(EntityFile) {
			ProtectedStructsMap[k] = true
		}
	}

	for _, s := range Structs {
		s = strings.TrimSpace(s)
		if s != "" {
			ProtectedStructsMap[s] = true
		}
	}

	if len(ProtectedStructsMap) == 0 {
		protectAllStructs = true
	}
}

func extractSelectorFromNode(n ast.Node) *ast.SelectorExpr {
	switch x := n.(type) {
	case *ast.AssignStmt:
		for _, lhs := range x.Lhs {
			if s := unwrapSelectorExpr(lhs); s != nil {
				return s
			}
		}
	case *ast.IncDecStmt:
		return unwrapSelectorExpr(x.X)
	case *ast.StarExpr:
		return unwrapSelectorExpr(x)
	}
	return nil
}

func unwrapSelectorExpr(expr ast.Expr) *ast.SelectorExpr {
	switch e := expr.(type) {
	case *ast.SelectorExpr:
		return e
	case *ast.ParenExpr:
		return unwrapSelectorExpr(e.X)
	case *ast.StarExpr:
		return unwrapSelectorExpr(e.X)
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			return unwrapSelectorExpr(e.X)
		}
	}
	return nil
}

func handleSelectorMutation(pass *analysis.Pass, sel *ast.SelectorExpr) {
	if pass.TypesInfo == nil {
		return
	}

	fieldName := sel.Sel.Name
	if !ast.IsExported(fieldName) {
		return
	}

	typ := pass.TypesInfo.TypeOf(sel.X)
	if typ == nil {
		return
	}

	base := deref(typ)
	named, ok := base.(*types.Named)
	if !ok {
		return
	}
	structName := named.Obj().Name()

	if !protectAllStructs && !ProtectedStructsMap[structName] {
		return
	}

	if isEmbeddedField(pass, sel, structName, fieldName) {
		return
	}

	if insideStructMethod(pass, sel.Pos(), structName) {
		return
	}

	reportIssue(pass, sel.Pos(), structName, fieldName)
}

func deref(t types.Type) types.Type {
	for {
		p, ok := t.(*types.Pointer)
		if !ok {
			return t
		}
		t = p.Elem()
	}
}

func insideStructMethod(pass *analysis.Pass, pos token.Pos, structName string) bool {
	fn := findEnclosingFunc(pass, pos)
	if fn == nil || fn.Recv == nil {
		return false
	}

	for _, recv := range fn.Recv.List {
		if isSameStructType(pass.TypesInfo.TypeOf(recv.Type), structName) {
			return true
		}
	}
	return false
}

func findEnclosingFunc(pass *analysis.Pass, pos token.Pos) *ast.FuncDecl {
	for _, file := range pass.Files {
		var found *ast.FuncDecl
		ast.Inspect(file, func(n ast.Node) bool {
			if f, ok := n.(*ast.FuncDecl); ok {
				if f.Pos() <= pos && pos <= f.End() {
					found = f
				}
			}
			return true
		})
		if found != nil {
			return found
		}
	}
	return nil
}

func isSameStructType(t types.Type, structName string) bool {
	return namedTypeName(deref(t)) == structName
}

func namedTypeName(t types.Type) string {
	if n, ok := t.(*types.Named); ok {
		return n.Obj().Name()
	}
	return ""
}

func isEmbeddedField(pass *analysis.Pass, sel *ast.SelectorExpr, structName, fieldName string) bool {
	t := deref(pass.TypesInfo.TypeOf(sel.X))
	n, ok := t.(*types.Named)
	if !ok {
		return false
	}

	s, ok := n.Underlying().(*types.Struct)
	if !ok {
		return false
	}

	for i := 0; i < s.NumFields(); i++ {
		f := s.Field(i)
		if f.Embedded() && f.Name() == fieldName {
			return true
		}
	}
	return false
}

func reportIssue(pass *analysis.Pass, pos token.Pos, structName, fieldName string) {
	key := fmt.Sprintf("%s.%s.%d", structName, fieldName, pos)
	if seen[key] {
		return
	}
	seen[key] = true

	pass.Reportf(
		pos,
		"assignment to exported field %s.%s is forbidden outside its methods",
		structName,
		fieldName,
	)
}

func loadEntityList(filePath string) map[string]bool {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, nil, parser.AllErrors)
	if err != nil {
		return map[string]bool{}
	}

	out := map[string]bool{}

	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}

		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || vs.Names[0].Name != "EntityList" {
				continue
			}

			for _, v := range vs.Values {
				cl, ok := v.(*ast.CompositeLit)
				if !ok {
					continue
				}
				for _, elt := range cl.Elts {
					if name := extractTypeName(elt); name != "" {
						out[name] = true
					}
				}
			}
		}
	}

	return out
}

func extractTypeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.CompositeLit:
		return extractTypeName(e.Type)
	case *ast.UnaryExpr: // &Type{}
		return extractTypeName(e.X)
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return e.Sel.Name
	}
	return ""
}
