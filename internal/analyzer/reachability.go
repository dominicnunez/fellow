package analyzer

import (
	"go/token"
	"path/filepath"

	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

type functionPosition struct {
	file string
	line int
}

func applyRuntimeReachability(root string, module *moduleState, opts Options) {
	pkgs := loadRuntimePackages(module, opts)
	if len(pkgs) == 0 {
		return
	}

	prog, ssaPkgs := ssautil.AllPackages(pkgs, ssa.InstantiateGenerics)
	prog.Build()

	roots := runtimeRoots(ssaPkgs)
	if len(roots) == 0 {
		return
	}

	result := rta.Analyze(roots, false)
	if result == nil {
		return
	}

	reachable := reachableFunctionPositions(root, result.Reachable)
	markRuntimeReachability(module, reachable)
}

func loadRuntimePackages(module *moduleState, opts Options) []*packages.Package {
	if len(module.packages) == 0 {
		return nil
	}

	pkgs, err := packages.Load(&packages.Config{
		Dir:        module.absDir,
		Tests:      opts.IncludeTests,
		BuildFlags: buildFlags(opts.BuildTags),
		Mode:       packages.LoadAllSyntax,
	}, loadPatternAll)
	if err != nil || packagesHaveErrors(pkgs) {
		return nil
	}

	return pkgs
}

func packagesHaveErrors(pkgs []*packages.Package) bool {
	for _, pkg := range pkgs {
		if pkg != nil && len(pkg.Errors) > 0 {
			return true
		}
	}

	return false
}

func runtimeRoots(pkgs []*ssa.Package) []*ssa.Function {
	roots := make([]*ssa.Function, 0)
	for _, pkg := range pkgs {
		if pkg == nil || pkg.Pkg == nil || pkg.Pkg.Name() != mainPackageName {
			continue
		}
		if initFn := pkg.Func(initFunctionName); initFn != nil {
			roots = append(roots, initFn)
		}
		if mainFn := pkg.Func(mainFunctionName); mainFn != nil {
			roots = append(roots, mainFn)
		}
	}

	return roots
}

func reachableFunctionPositions(root string, reachable map[*ssa.Function]struct{ AddrTaken bool }) map[functionPosition]struct{} {
	positions := make(map[functionPosition]struct{}, len(reachable))
	for fn := range reachable {
		position, ok := sourceFunctionPosition(root, fn)
		if !ok {
			continue
		}
		positions[position] = struct{}{}
	}

	return positions
}

func sourceFunctionPosition(root string, fn *ssa.Function) (functionPosition, bool) {
	if fn == nil || !fn.Pos().IsValid() || fn.Prog == nil || fn.Prog.Fset == nil {
		return functionPosition{}, false
	}
	pos := fn.Prog.Fset.PositionFor(fn.Pos(), false)
	if !pos.IsValid() || pos.Filename == "" {
		return functionPosition{}, false
	}

	return functionPosition{file: relPath(root, cleanFilename(pos)), line: pos.Line}, true
}

func cleanFilename(pos token.Position) string {
	if abs, err := filepath.Abs(pos.Filename); err == nil {
		return abs
	}
	return pos.Filename
}

func markRuntimeReachability(module *moduleState, reachable map[functionPosition]struct{}) {
	for pkgIndex := range module.packages {
		for declIndex := range module.packages[pkgIndex].declarations {
			decl := &module.packages[pkgIndex].declarations[declIndex]
			if decl.Kind != declarationFunction && decl.Kind != declarationMethod {
				continue
			}
			decl.RuntimeChecked = true
			_, decl.RuntimeReachable = reachable[functionPosition{file: decl.File, line: decl.Line}]
		}
	}
}
