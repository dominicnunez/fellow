package analyzer

import (
	"go/ast"
	"go/token"
)

func healthFindings(module moduleState, opts Options) []Finding {
	if opts.MaxCyclomatic <= 0 && opts.MaxCognitive <= 0 {
		return nil
	}

	var findings []Finding
	for _, pkg := range module.packages {
		for _, source := range pkg.files {
			findings = append(findings, healthFindingsForSource(pkg, source, opts.MaxCyclomatic, opts.MaxCognitive)...)
		}
	}

	return findings
}

func healthFindingsForSource(pkg packageState, source *sourceFile, maxCyclomatic int, maxCognitive int) []Finding {
	findings := make([]Finding, 0)
	for _, decl := range source.file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		cyclomatic, cognitive := functionComplexity(fn)
		if !complexityExceeded(cyclomatic, cognitive, maxCyclomatic, maxCognitive) {
			continue
		}
		findings = append(findings, complexityFinding(pkg, source, fn, cyclomatic, cognitive))
	}

	return findings
}

func complexityExceeded(cyclomatic int, cognitive int, maxCyclomatic int, maxCognitive int) bool {
	return maxCyclomatic > 0 && cyclomatic > maxCyclomatic || maxCognitive > 0 && cognitive > maxCognitive
}

func complexityFinding(pkg packageState, source *sourceFile, fn *ast.FuncDecl, cyclomatic int, cognitive int) Finding {
	finding := Finding{
		Type:        FindingComplexity,
		Package:     pkg.importPath,
		ImportPath:  pkg.importPath,
		Symbol:      fn.Name.Name,
		File:        source.relPath,
		Line:        source.fset.Position(fn.Name.Pos()).Line,
		Fingerprint: "",
		Metrics: Metrics{
			Cyclomatic: cyclomatic,
			Cognitive:  cognitive,
		},
	}
	if fn.Recv != nil {
		finding.Receiver = receiverTypeName(fn.Recv)
	}

	return finding
}

func functionComplexity(fn *ast.FuncDecl) (int, int) {
	cyclomatic := 1
	var walk func(ast.Node, int)
	walk = func(node ast.Node, nesting int) {
		switch n := node.(type) {
		case nil:
			return
		case *ast.IfStmt:
			cyclomatic++
			walk(n.Init, nesting)
			walk(n.Cond, nesting)
			walk(n.Body, nesting+1)
			walk(n.Else, nesting+1)
			return
		case *ast.ForStmt:
			cyclomatic++
			walk(n.Init, nesting)
			walk(n.Cond, nesting)
			walk(n.Post, nesting)
			walk(n.Body, nesting+1)
			return
		case *ast.RangeStmt:
			cyclomatic++
			walk(n.X, nesting)
			walk(n.Body, nesting+1)
			return
		case *ast.CaseClause:
			cyclomatic++
		case *ast.CommClause:
			cyclomatic++
		case *ast.BinaryExpr:
			if n.Op.String() == "&&" || n.Op.String() == "||" {
				cyclomatic++
			}
		}

		ast.Inspect(node, func(child ast.Node) bool {
			if child == nil || child == node {
				return true
			}
			walk(child, nesting)
			return false
		})
	}
	walk(fn.Body, 0)

	return cyclomatic, cognitiveComplexity(fn)
}

type cognitiveComplexityVisitor struct {
	name       *ast.Ident
	complexity int
	nesting    int
	elseIfs    map[*ast.IfStmt]struct{}
	calculated map[*ast.BinaryExpr]struct{}
}

func cognitiveComplexity(fn *ast.FuncDecl) int {
	visitor := &cognitiveComplexityVisitor{
		name:       fn.Name,
		elseIfs:    make(map[*ast.IfStmt]struct{}),
		calculated: make(map[*ast.BinaryExpr]struct{}),
	}
	ast.Walk(visitor, fn.Body)

	return visitor.complexity
}

func (v *cognitiveComplexityVisitor) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case nil:
		return nil
	case *ast.IfStmt:
		v.visitIfStmt(n)
	case *ast.ForStmt:
		v.visitForStmt(n)
	case *ast.RangeStmt:
		v.visitRangeStmt(n)
	case *ast.SwitchStmt:
		v.visitSwitchStmt(n)
	case *ast.TypeSwitchStmt:
		v.visitTypeSwitchStmt(n)
	case *ast.SelectStmt:
		v.visitSelectStmt(n)
	case *ast.FuncLit:
		v.visitFuncLit(n)
	case *ast.BinaryExpr:
		v.visitBinaryExpr(n)
		return v
	case *ast.BranchStmt:
		v.visitBranchStmt(n)
		return v
	case *ast.CallExpr:
		v.visitCallExpr(n)
		return v
	default:
		return v
	}

	return nil
}

func (v *cognitiveComplexityVisitor) visitIfStmt(stmt *ast.IfStmt) {
	if _, ok := v.elseIfs[stmt]; ok {
		v.increment()
	} else {
		v.incrementNested()
	}
	if stmt.Init != nil {
		ast.Walk(v, stmt.Init)
	}
	ast.Walk(v, stmt.Cond)
	v.walkNested(stmt.Body)

	switch elseStmt := stmt.Else.(type) {
	case nil:
		return
	case *ast.BlockStmt:
		v.increment()
		ast.Walk(v, elseStmt)
	case *ast.IfStmt:
		v.elseIfs[elseStmt] = struct{}{}
		ast.Walk(v, elseStmt)
	default:
		ast.Walk(v, elseStmt)
	}
}

func (v *cognitiveComplexityVisitor) visitForStmt(stmt *ast.ForStmt) {
	v.incrementNested()
	if stmt.Init != nil {
		ast.Walk(v, stmt.Init)
	}
	if stmt.Cond != nil {
		ast.Walk(v, stmt.Cond)
	}
	if stmt.Post != nil {
		ast.Walk(v, stmt.Post)
	}
	v.walkNested(stmt.Body)
}

func (v *cognitiveComplexityVisitor) visitRangeStmt(stmt *ast.RangeStmt) {
	v.incrementNested()
	ast.Walk(v, stmt.X)
	v.walkNested(stmt.Body)
}

func (v *cognitiveComplexityVisitor) visitSwitchStmt(stmt *ast.SwitchStmt) {
	v.incrementNested()
	if stmt.Init != nil {
		ast.Walk(v, stmt.Init)
	}
	if stmt.Tag != nil {
		ast.Walk(v, stmt.Tag)
	}
	v.walkNested(stmt.Body)
}

func (v *cognitiveComplexityVisitor) visitTypeSwitchStmt(stmt *ast.TypeSwitchStmt) {
	v.incrementNested()
	if stmt.Init != nil {
		ast.Walk(v, stmt.Init)
	}
	if stmt.Assign != nil {
		ast.Walk(v, stmt.Assign)
	}
	v.walkNested(stmt.Body)
}

func (v *cognitiveComplexityVisitor) visitSelectStmt(stmt *ast.SelectStmt) {
	v.incrementNested()
	v.walkNested(stmt.Body)
}

func (v *cognitiveComplexityVisitor) visitFuncLit(lit *ast.FuncLit) {
	ast.Walk(v, lit.Type)
	v.walkNested(lit.Body)
}

func (v *cognitiveComplexityVisitor) visitBinaryExpr(expr *ast.BinaryExpr) {
	if !isLogicalOperator(expr.Op) {
		return
	}
	if _, ok := v.calculated[expr]; ok {
		return
	}

	var last token.Token
	for _, op := range v.collectLogicalOperators(expr) {
		if op == last {
			continue
		}
		v.increment()
		last = op
	}
}

func (v *cognitiveComplexityVisitor) visitBranchStmt(stmt *ast.BranchStmt) {
	if stmt.Label != nil {
		v.increment()
	}
}

func (v *cognitiveComplexityVisitor) visitCallExpr(expr *ast.CallExpr) {
	ident, ok := expr.Fun.(*ast.Ident)
	if !ok || v.name == nil || ident.Obj == nil {
		return
	}
	if ident.Obj == v.name.Obj && ident.Name == v.name.Name {
		v.increment()
	}
}

func (v *cognitiveComplexityVisitor) walkNested(node ast.Node) {
	v.nesting++
	ast.Walk(v, node)
	v.nesting--
}

func (v *cognitiveComplexityVisitor) increment() {
	v.complexity++
}

func (v *cognitiveComplexityVisitor) incrementNested() {
	v.complexity += 1 + v.nesting
}

func (v *cognitiveComplexityVisitor) collectLogicalOperators(expr ast.Expr) []token.Token {
	binary, ok := expr.(*ast.BinaryExpr)
	if !ok {
		return nil
	}
	v.calculated[binary] = struct{}{}

	operators := v.collectLogicalOperators(binary.X)
	if isLogicalOperator(binary.Op) {
		operators = append(operators, binary.Op)
	}
	operators = append(operators, v.collectLogicalOperators(binary.Y)...)

	return operators
}

func isLogicalOperator(op token.Token) bool {
	return op == token.LAND || op == token.LOR
}
