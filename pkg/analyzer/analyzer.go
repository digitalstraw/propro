package analyzer

import (
	"errors"
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

const (
	metaName          = "propro"
	metaDoc           = "Detects writes to exported fields of protected structs outside methods"
	metaURL           = "github.com/digitalstraw/propro"
	entityListVarName = "EntityList"

	// These must be identical to golangci-lint repo config keys.
	entityListFileArg = "entityListFile"
	structsArg        = "structs"
)

var (
	EntityFile string
	Structs    []string

	ProtectedStructsMap map[string]bool
	protectAllStructs   bool
	seen                map[string]bool

	ErrNotInspectAnalyzer = errors.New("inspect analyzer result is not *inspector.Inspector")

	flagSet flag.FlagSet
)

func CliInit(args []string) {
	structs := ""

	flagSet.StringVar(&EntityFile, entityListFileArg, "", "Path to file listing protected structs")
	flagSet.StringVar(&structs, structsArg, "", "Comma-separated list of protected structs")

	err := flagSet.Parse(args)
	if err != nil {
		return
	}

	EntityFile = strings.TrimSpace(EntityFile)

	parts := strings.Split(structs, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			Structs = append(Structs, part)
		}
	}
}

func NewAnalyzer(cfg map[string]any) *analysis.Analyzer {
	if v, ok := cfg[entityListFileArg].(string); ok && v != "" {
		EntityFile = v
	}
	if v, ok := cfg[structsArg].([]string); ok && len(v) > 0 {
		Structs = append(Structs, v...)
	}

	buildProtectedStructMap()

	return &analysis.Analyzer{
		Name:     metaName,
		Doc:      metaDoc,
		URL:      metaURL,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
		Flags:    flagSet,
		Run:      run,
	}
}

// buildProtectedStructMap populates ProtectedStructsMap and sets protectAllStructs when empty.
func buildProtectedStructMap() {
	if len(ProtectedStructsMap) > 0 || protectAllStructs {
		// Concurrency expected: already built
		return
	}

	ProtectedStructsMap = make(map[string]bool)

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

func run(pass *analysis.Pass) (any, error) {
	seen = make(map[string]bool)
	aliasMap := map[types.Object]*ast.SelectorExpr{}

	insp, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, ErrNotInspectAnalyzer
	}

	inNodes := []ast.Node{
		(*ast.AssignStmt)(nil),
		(*ast.IncDecStmt)(nil),
		(*ast.CallExpr)(nil),
	}

	insp.Preorder(inNodes, func(n ast.Node) {
		switch node := n.(type) {
		case *ast.AssignStmt:
			handleAssignStmt(pass, node, aliasMap)
		case *ast.IncDecStmt:
			handleIncDecStmt(pass, node, aliasMap)
		case *ast.CallExpr:
			handleCallExpr(pass, node, aliasMap)
		}
	})

	return nil, nil
}

// handleAssignStmt processes assignments and checks mutations.
func handleAssignStmt(pass *analysis.Pass, node *ast.AssignStmt, aliasMap map[types.Object]*ast.SelectorExpr) {
	trackAlias(pass, node, aliasMap)
	for _, lhs := range node.Lhs {
		if sel := resolveMutationTarget(pass, lhs, aliasMap); sel != nil {
			handleSelectorMutation(pass, sel)
		}
	}
}

// handleIncDecStmt handles ++/-- operations.
func handleIncDecStmt(pass *analysis.Pass, node *ast.IncDecStmt, aliasMap map[types.Object]*ast.SelectorExpr) {
	if sel := resolveMutationTarget(pass, node.X, aliasMap); sel != nil {
		handleSelectorMutation(pass, sel)
	}
}

// handleCallExpr tracks argument aliases in function calls.
func handleCallExpr(pass *analysis.Pass, node *ast.CallExpr, aliasMap map[types.Object]*ast.SelectorExpr) {
	fnType := pass.TypesInfo.TypeOf(node.Fun)
	sig, ok := fnType.(*types.Signature)
	if !ok {
		return
	}
	for idx, arg := range node.Args {
		if sel := unwrapSelectorExpr(arg); sel != nil && idx < sig.Params().Len() {
			param := sig.Params().At(idx)
			aliasMap[param] = sel
		}
	}
}

// trackAlias captures simple aliasing like: x := &e.Field.
func trackAlias(pass *analysis.Pass, node *ast.AssignStmt, aliasMap map[types.Object]*ast.SelectorExpr) {
	if len(node.Lhs) != 1 || len(node.Rhs) != 1 {
		return
	}
	lhsIdent, ok := node.Lhs[0].(*ast.Ident)
	if !ok {
		return
	}
	unary, ok := node.Rhs[0].(*ast.UnaryExpr)
	if !ok || unary.Op != token.AND {
		return
	}
	sel := unwrapSelectorExpr(unary.X)
	if sel == nil {
		return
	}
	if obj := pass.TypesInfo.ObjectOf(lhsIdent); obj != nil {
		aliasMap[obj] = sel
	}
}

// resolveMutationTarget centralizes all ways an expression can represent a mutation target.
func resolveMutationTarget(pass *analysis.Pass, expr ast.Expr, aliasMap map[types.Object]*ast.SelectorExpr) *ast.SelectorExpr {
	// Direct selector: e.Field or parentheses/star/unary wrapping
	if sel := unwrapSelectorExpr(expr); sel != nil {
		return sel
	}

	// star of ident: *x or ident itself (for IncDec with x)
	switch e := expr.(type) {
	case *ast.StarExpr:
		if id, ok := e.X.(*ast.Ident); ok {
			if obj := pass.TypesInfo.ObjectOf(id); obj != nil {
				return aliasMap[obj]
			}
		}
	case *ast.Ident:
		if obj := pass.TypesInfo.ObjectOf(e); obj != nil {
			return aliasMap[obj]
		}
	}
	return nil
}

// handleSelectorMutation validates selector and reports if it's a forbidden mutation.
func handleSelectorMutation(pass *analysis.Pass, sel *ast.SelectorExpr) {
	structName, fieldName, protectionViolated := guardProtectedFieldMutation(pass, sel)
	if !protectionViolated {
		return
	}
	reportIssue(pass, sel.Pos(), structName, fieldName)
}

// guardProtectedFieldMutation does the heavy checks: exported, protected struct, embedded, method.
func guardProtectedFieldMutation(pass *analysis.Pass, sel *ast.SelectorExpr) (structName, fieldName string, protectionViolated bool) {
	if pass.TypesInfo == nil || sel == nil {
		return "", "", false
	}

	fieldName = sel.Sel.Name
	if !ast.IsExported(fieldName) {
		return "", "", false
	}

	typ := pass.TypesInfo.TypeOf(sel.X)
	if typ == nil {
		return "", "", false
	}

	base := deref(typ)
	named, ok := base.(*types.Named)
	if !ok {
		return "", "", false
	}
	structName = named.Obj().Name()

	if !protectAllStructs && !ProtectedStructsMap[structName] {
		return "", "", false
	}

	if isEmbeddedField(pass, sel, fieldName) {
		return "", "", false
	}

	if insideStructMethod(pass, sel.Pos(), structName) {
		return "", "", false
	}

	return structName, fieldName, true
}

// deref peels pointer types to get the base type.
func deref(t types.Type) types.Type {
	for {
		if p, ok := t.(*types.Pointer); ok {
			t = p.Elem()
			continue
		}
		return t
	}
}

// insideStructMethod checks if the position is inside a method of the given struct.
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

// findEnclosingFunc finds the function declaration enclosing the given position.
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

// isSameStructType checks if the type matches the named struct.
func isSameStructType(t types.Type, structName string) bool {
	return namedTypeName(deref(t)) == structName
}

// namedTypeName returns the name of the named type, or empty string.
func namedTypeName(t types.Type) string {
	if n, ok := t.(*types.Named); ok {
		return n.Obj().Name()
	}
	return ""
}

// isEmbeddedField checks if the field is embedded in the struct.
func isEmbeddedField(pass *analysis.Pass, sel *ast.SelectorExpr, fieldName string) bool {
	t := deref(pass.TypesInfo.TypeOf(sel.X))
	n, ok := t.(*types.Named)
	if !ok {
		return false
	}

	s, ok := n.Underlying().(*types.Struct)
	if !ok {
		return false
	}

	for i := range s.NumFields() {
		f := s.Field(i)
		if f.Embedded() && f.Name() == fieldName {
			return true
		}
	}
	return false
}

// reportIssue reports the forbidden mutation if not already reported.
func reportIssue(pass *analysis.Pass, pos token.Pos, structName, fieldName string) {
	key := fmt.Sprintf("%s.%s.%d", structName, fieldName, pos)
	if seen[key] {
		return
	}
	seen[key] = true

	pass.Reportf(pos, "assignment to exported field %s.%s is forbidden outside its methods", structName, fieldName)
}

// loadEntityList reads a file containing a var EntityList = []Type{...}.
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
			if !ok || len(vs.Names) == 0 || vs.Names[0].Name != entityListVarName {
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

// extractTypeName extracts the type name from an expression.
func extractTypeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.CompositeLit:
		return extractTypeName(e.Type)
	case *ast.UnaryExpr:
		// &Type{}
		return extractTypeName(e.X)
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return e.Sel.Name
	}
	return ""
}

// unwrapSelectorExpr peels parentheses, pointer and address-of to find selector expressions.
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
