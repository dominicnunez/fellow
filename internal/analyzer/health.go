package analyzer

import "go/ast"

func healthFindings(module moduleState, opts Options) []Finding {
	maxCyclomatic := opts.MaxCyclomatic
	if maxCyclomatic == 0 {
		maxCyclomatic = defaultMaxCyclomatic
	}
	maxCognitive := opts.MaxCognitive
	if maxCognitive == 0 {
		maxCognitive = defaultMaxCognitive
	}

	var findings []Finding
	for _, pkg := range module.packages {
		for _, source := range pkg.files {
			for _, decl := range source.file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}

				cyclomatic, cognitive := functionComplexity(fn)
				if cyclomatic <= maxCyclomatic && cognitive <= maxCognitive {
					continue
				}

				line := source.fset.Position(fn.Name.Pos()).Line
				finding := Finding{
					Type:        FindingComplexity,
					Package:     pkg.importPath,
					ImportPath:  pkg.importPath,
					Symbol:      fn.Name.Name,
					File:        source.relPath,
					Line:        line,
					Fingerprint: "",
					Metrics: Metrics{
						Cyclomatic: cyclomatic,
						Cognitive:  cognitive,
					},
				}
				if fn.Recv != nil {
					finding.Receiver = receiverTypeName(fn.Recv)
				}
				findings = append(findings, finding)
			}
		}
	}

	return findings
}

func functionComplexity(fn *ast.FuncDecl) (int, int) {
	cyclomatic := 1
	cognitive := 0
	var walk func(ast.Node, int)
	walk = func(node ast.Node, nesting int) {
		switch n := node.(type) {
		case nil:
			return
		case *ast.IfStmt:
			cyclomatic++
			cognitive += 1 + nesting
			walk(n.Init, nesting)
			walk(n.Cond, nesting)
			walk(n.Body, nesting+1)
			walk(n.Else, nesting+1)
			return
		case *ast.ForStmt:
			cyclomatic++
			cognitive += 1 + nesting
			walk(n.Init, nesting)
			walk(n.Cond, nesting)
			walk(n.Post, nesting)
			walk(n.Body, nesting+1)
			return
		case *ast.RangeStmt:
			cyclomatic++
			cognitive += 1 + nesting
			walk(n.X, nesting)
			walk(n.Body, nesting+1)
			return
		case *ast.CaseClause:
			cyclomatic++
			cognitive += 1 + nesting
		case *ast.CommClause:
			cyclomatic++
			cognitive += 1 + nesting
		case *ast.BinaryExpr:
			if n.Op.String() == "&&" || n.Op.String() == "||" {
				cyclomatic++
				cognitive++
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

	return cyclomatic, cognitive
}
