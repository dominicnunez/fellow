package analyzer

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	packageKeySeparator = "\x00"
	testPackageSuffix   = "_test"

	blankImportName = "_"
	dotImportName   = "."
	mainPackageName = "main"

	mainFunctionName      = "main"
	initFunctionName      = "init"
	testFunctionPrefix    = "Test"
	benchmarkFunctionPref = "Benchmark"
	fuzzFunctionPrefix    = "Fuzz"
	exampleFunctionPrefix = "Example"

	declarationFunction  = "function"
	declarationMethod    = "method"
	declarationStruct    = "struct"
	declarationInterface = "interface"
	declarationType      = "type"
	declarationVar       = "var"
	declarationConst     = "const"
	declarationField     = "field"

	majorVersionPrefix = "v"
	noLine             = 0
	firstLine          = 1

	decodeCallName        = "Decode"
	unmarshalCallName     = "Unmarshal"
	unmarshalJSONCallName = "UnmarshalJSON"
	scanCallName          = "Scan"
	structScanCallName    = "StructScan"

	suppressFileToken     = "fellow-ignore-file"
	suppressNextLineToken = "fellow-ignore-next-line"
	suppressAllRules      = "*"
)

type packageState struct {
	importPath         string
	realImportPath     string
	name               string
	dir                string
	relDir             string
	isExternalTest     bool
	files              []*sourceFile
	imports            []ImportUse
	internalImports    map[string][]ImportUse
	declarations       []declaration
	usedNames          map[string][]codeUse
	selectorNames      map[string][]codeUse
	fieldKeys          map[string][]codeUse
	usedPackageSymbols map[string]map[string][]codeUse
	reflectiveStructs  map[string][]codeUse
	dotImports         []string
	interfaceMethods   map[string]struct{}
}

type sourceFile struct {
	absPath             string
	relPath             string
	file                *ast.File
	fset                *token.FileSet
	test                bool
	generated           bool
	tool                bool
	suppressFile        bool
	suppressNextLine    map[int]map[string]struct{}
	hasSideEffectImport bool
	hasSpecialFunction  bool
	hasTopLevelValue    bool
	hasDeclarations     bool
}

type declaration struct {
	Kind             string
	Name             string
	Receiver         string
	Struct           string
	FieldType        string
	File             string
	Line             int
	EndLine          int
	HasTag           bool
	Special          bool
	SideEffect       bool
	Typed            bool
	TypedUsed        bool
	RuntimeChecked   bool
	RuntimeReachable bool
}

type codeUse struct {
	File string
	Line int
}

type typedVariable struct {
	TypeName string
	Line     int
}

type moduleUsage struct {
	selectorUses      map[string][]codeUse
	fieldKeyUses      map[string][]codeUse
	packageSymbolUses map[string]map[string][]codeUse
	dotImports        map[string]struct{}
	interfaceMethods  map[string]struct{}
}

func scanModulePackages(root string, module moduleState, moduleDirs map[string]struct{}, opts Options) ([]packageState, []ImportUse, error) {
	packagesByKey := make(map[string]*packageState)

	err := filepath.WalkDir(module.absDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			if path != module.absDir {
				if shouldSkipDir(d.Name()) {
					return filepath.SkipDir
				}
				if _, ok := moduleDirs[path]; ok {
					return filepath.SkipDir
				}
			}
			return nil
		}

		if !strings.HasSuffix(d.Name(), goFileSuffix) {
			return nil
		}
		if !opts.IncludeTests && strings.HasSuffix(d.Name(), testFileSuffix) {
			return nil
		}
		if !matchesBuildContext(path, opts.BuildTags) && !isToolSourceFile(path) {
			return nil
		}

		source, err := parseSourceFile(root, path, opts)
		if err != nil {
			return err
		}
		if source == nil {
			return nil
		}

		pkg := packageForFile(packagesByKey, module, source)
		pkg.files = append(pkg.files, source)

		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	packages := packageMapValues(packagesByKey)
	packageNames := packageNamesByImportPath(packages)
	for i := range packages {
		initializePackageUsage(&packages[i])
		for _, source := range packages[i].files {
			collectFileFacts(&packages[i], source, packageNames)
		}
	}

	packageByImport := normalPackageIndexes(packages)
	imports := make([]ImportUse, 0)
	for i := range packages {
		packages[i].internalImports = make(map[string][]ImportUse)
		for _, use := range packages[i].imports {
			imports = append(imports, use)
			if _, ok := packageByImport[use.ImportPath]; ok {
				packages[i].internalImports[use.ImportPath] = append(packages[i].internalImports[use.ImportPath], use)
			}
		}
	}
	sortImportUses(imports)

	return packages, imports, nil
}

func matchesBuildContext(path string, buildTags []string) bool {
	ctx := build.Default
	ctx.BuildTags = buildTags
	matched, err := ctx.MatchFile(filepath.Dir(path), filepath.Base(path))
	return err == nil && matched
}

func isToolSourceFile(path string) bool {
	return filepath.Base(path) == "tools.go"
}

func parseSourceFile(root string, path string, opts Options) (*sourceFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	generated := isGeneratedSource(data)
	if generated && !opts.IncludeGenerated {
		return nil, nil
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, data, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	suppressFile, suppressNextLine := parseCommentSuppressions(file, fset)

	return &sourceFile{
		absPath:          path,
		relPath:          relPath(root, path),
		file:             file,
		fset:             fset,
		test:             strings.HasSuffix(filepath.Base(path), testFileSuffix),
		generated:        generated,
		tool:             isToolSourceFile(path),
		suppressFile:     suppressFile,
		suppressNextLine: suppressNextLine,
	}, nil
}

func parseCommentSuppressions(file *ast.File, fset *token.FileSet) (bool, map[int]map[string]struct{}) {
	suppressions := make(map[int]map[string]struct{})
	suppressFile := false
	for _, group := range file.Comments {
		for _, comment := range group.List {
			text := comment.Text
			if strings.Contains(text, suppressFileToken) {
				suppressFile = true
			}
			idx := strings.Index(text, suppressNextLineToken)
			if idx < 0 {
				continue
			}

			line := fset.Position(comment.Slash).Line
			rules := parseSuppressionRules(text[idx+len(suppressNextLineToken):])
			suppressions[line+1] = rules
		}
	}

	return suppressFile, suppressions
}

func parseSuppressionRules(value string) map[string]struct{} {
	rules := make(map[string]struct{})
	for _, field := range strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t'
	}) {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		rules[field] = struct{}{}
	}
	if len(rules) == 0 {
		rules[suppressAllRules] = struct{}{}
	}

	return rules
}

func packageForFile(packagesByKey map[string]*packageState, module moduleState, source *sourceFile) *packageState {
	dir := filepath.Dir(source.absPath)
	name := source.file.Name.Name
	realImportPath := packageImportPath(module, dir)
	isExternalTest := strings.HasSuffix(name, testPackageSuffix)
	importPath := realImportPath
	if isExternalTest {
		importPath += testPackageSuffix
	}

	key := dir + packageKeySeparator + name
	pkg := packagesByKey[key]
	if pkg != nil {
		return pkg
	}

	pkg = &packageState{
		importPath:     importPath,
		realImportPath: realImportPath,
		name:           name,
		dir:            dir,
		relDir:         relPath(module.absDir, dir),
		isExternalTest: isExternalTest,
	}
	packagesByKey[key] = pkg

	return pkg
}

func packageImportPath(module moduleState, dir string) string {
	rel := relPath(module.absDir, dir)
	if rel == "." {
		return module.report.ModulePath
	}
	return module.report.ModulePath + "/" + rel
}

func packageMapValues(packagesByKey map[string]*packageState) []packageState {
	packages := make([]packageState, 0, len(packagesByKey))
	for _, pkg := range packagesByKey {
		sort.Slice(pkg.files, func(i, j int) bool {
			return pkg.files[i].relPath < pkg.files[j].relPath
		})
		packages = append(packages, *pkg)
	}
	sort.Slice(packages, func(i, j int) bool {
		if packages[i].importPath == packages[j].importPath {
			return packages[i].name < packages[j].name
		}
		return packages[i].importPath < packages[j].importPath
	})

	return packages
}

func packageNamesByImportPath(packages []packageState) map[string]string {
	names := make(map[string]string, len(packages))
	for _, pkg := range packages {
		if pkg.isExternalTest {
			continue
		}
		names[pkg.importPath] = pkg.name
	}

	return names
}

func normalPackageIndexes(packages []packageState) map[string]int {
	indexes := make(map[string]int, len(packages))
	for i, pkg := range packages {
		if pkg.isExternalTest {
			continue
		}
		indexes[pkg.importPath] = i
	}

	return indexes
}

func initializePackageUsage(pkg *packageState) {
	pkg.usedNames = make(map[string][]codeUse)
	pkg.selectorNames = make(map[string][]codeUse)
	pkg.fieldKeys = make(map[string][]codeUse)
	pkg.usedPackageSymbols = make(map[string]map[string][]codeUse)
	pkg.reflectiveStructs = make(map[string][]codeUse)
	pkg.interfaceMethods = make(map[string]struct{})
}

func collectFileFacts(pkg *packageState, source *sourceFile, packageNames map[string]string) {
	aliases := collectImports(pkg, source, packageNames)
	ignoredPositions := make(map[token.Pos]struct{})
	collectDeclarations(pkg, source, ignoredPositions)
	collectUses(pkg, source, aliases, ignoredPositions)
}

func collectImports(pkg *packageState, source *sourceFile, packageNames map[string]string) map[string]string {
	aliases := make(map[string]string)

	for _, spec := range source.file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}

		line := source.fset.Position(spec.Path.Pos()).Line
		use := ImportUse{
			ImportPath: importPath,
			File:       source.relPath,
			Line:       line,
			Test:       source.test,
			Generated:  source.generated,
			Tool:       source.tool,
		}
		pkg.imports = append(pkg.imports, use)

		alias := importAlias(spec, importPath, packageNames)
		switch alias {
		case blankImportName:
			source.hasSideEffectImport = true
		case dotImportName:
			pkg.dotImports = append(pkg.dotImports, importPath)
		case "":
			continue
		default:
			aliases[alias] = importPath
		}
	}

	return aliases
}

func importAlias(spec *ast.ImportSpec, importPath string, packageNames map[string]string) string {
	if spec.Name != nil {
		return spec.Name.Name
	}
	if name := packageNames[importPath]; name != "" {
		return name
	}

	return defaultImportName(importPath)
}

func defaultImportName(importPath string) string {
	parts := strings.Split(importPath, "/")
	if len(parts) == 0 {
		return ""
	}

	last := parts[len(parts)-1]
	if isMajorVersionSegment(last) && len(parts) > 1 {
		return parts[len(parts)-2]
	}

	return last
}

func isMajorVersionSegment(segment string) bool {
	if !strings.HasPrefix(segment, majorVersionPrefix) || len(segment) == len(majorVersionPrefix) {
		return false
	}
	for _, r := range segment[len(majorVersionPrefix):] {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}

func collectDeclarations(pkg *packageState, source *sourceFile, ignoredPositions map[token.Pos]struct{}) {
	for _, decl := range source.file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			collectFuncDeclaration(pkg, source, d, ignoredPositions)
		case *ast.GenDecl:
			collectGenDeclaration(pkg, source, d, ignoredPositions)
		}
	}
}

func collectFuncDeclaration(pkg *packageState, source *sourceFile, decl *ast.FuncDecl, ignoredPositions map[token.Pos]struct{}) {
	ignoredPositions[decl.Name.Pos()] = struct{}{}
	startLine, endLine := lineRange(source.fset, decl)
	special := isSpecialFunction(pkg, source, decl.Name.Name)
	if special {
		source.hasSpecialFunction = true
	}

	entry := declaration{
		Kind:    declarationFunction,
		Name:    decl.Name.Name,
		File:    source.relPath,
		Line:    startLine,
		EndLine: endLine,
		Special: special,
	}
	if decl.Recv != nil {
		entry.Kind = declarationMethod
		entry.Receiver = receiverTypeName(decl.Recv)
		collectIdentPositions(decl.Recv, ignoredPositions)
	}

	source.hasDeclarations = true
	pkg.declarations = append(pkg.declarations, entry)
}

func collectGenDeclaration(pkg *packageState, source *sourceFile, decl *ast.GenDecl, ignoredPositions map[token.Pos]struct{}) {
	switch decl.Tok {
	case token.TYPE:
		for _, spec := range decl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			collectTypeDeclaration(pkg, source, typeSpec, ignoredPositions)
		}
	case token.VAR, token.CONST:
		for _, spec := range decl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			collectValueDeclarations(pkg, source, decl.Tok, valueSpec, ignoredPositions)
		}
	}
}

func collectValueDeclarations(pkg *packageState, source *sourceFile, tok token.Token, spec *ast.ValueSpec, ignoredPositions map[token.Pos]struct{}) {
	kind := declarationVar
	if tok == token.CONST {
		kind = declarationConst
	}
	sideEffect := tok == token.VAR && valueSpecHasSideEffect(spec)
	if sideEffect {
		source.hasTopLevelValue = true
	}

	for _, name := range spec.Names {
		ignoredPositions[name.Pos()] = struct{}{}
		startLine, endLine := lineRange(source.fset, name)
		pkg.declarations = append(pkg.declarations, declaration{
			Kind:       kind,
			Name:       name.Name,
			File:       source.relPath,
			Line:       startLine,
			EndLine:    endLine,
			SideEffect: sideEffect,
		})
	}
}

func valueSpecHasSideEffect(spec *ast.ValueSpec) bool {
	for _, value := range spec.Values {
		if exprHasCall(value) {
			return true
		}
	}

	return false
}

func exprHasCall(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		if _, ok := node.(*ast.CallExpr); ok {
			found = true
			return false
		}

		return true
	})

	return found
}

func collectTypeDeclaration(pkg *packageState, source *sourceFile, spec *ast.TypeSpec, ignoredPositions map[token.Pos]struct{}) {
	ignoredPositions[spec.Name.Pos()] = struct{}{}
	startLine, endLine := lineRange(source.fset, spec)
	source.hasDeclarations = true

	switch typ := spec.Type.(type) {
	case *ast.StructType:
		pkg.declarations = append(pkg.declarations, declaration{
			Kind:    declarationStruct,
			Name:    spec.Name.Name,
			File:    source.relPath,
			Line:    startLine,
			EndLine: endLine,
		})
		collectStructFields(pkg, source, spec.Name.Name, typ, ignoredPositions)
	case *ast.InterfaceType:
		pkg.declarations = append(pkg.declarations, declaration{
			Kind:    declarationInterface,
			Name:    spec.Name.Name,
			File:    source.relPath,
			Line:    startLine,
			EndLine: endLine,
		})
		collectInterfaceMethods(pkg, typ, ignoredPositions)
	default:
		pkg.declarations = append(pkg.declarations, declaration{
			Kind:    declarationType,
			Name:    spec.Name.Name,
			File:    source.relPath,
			Line:    startLine,
			EndLine: endLine,
		})
	}
}

func collectStructFields(pkg *packageState, source *sourceFile, structName string, typ *ast.StructType, ignoredPositions map[token.Pos]struct{}) {
	for _, field := range typ.Fields.List {
		for _, name := range field.Names {
			ignoredPositions[name.Pos()] = struct{}{}
			startLine, endLine := lineRange(source.fset, field)
			pkg.declarations = append(pkg.declarations, declaration{
				Kind:      declarationField,
				Name:      name.Name,
				Struct:    structName,
				FieldType: exprTypeName(field.Type),
				File:      source.relPath,
				Line:      startLine,
				EndLine:   endLine,
				HasTag:    field.Tag != nil,
			})
		}
	}
}

func collectInterfaceMethods(pkg *packageState, typ *ast.InterfaceType, ignoredPositions map[token.Pos]struct{}) {
	for _, field := range typ.Methods.List {
		for _, name := range field.Names {
			ignoredPositions[name.Pos()] = struct{}{}
			pkg.interfaceMethods[name.Name] = struct{}{}
		}
	}
}

func collectUses(pkg *packageState, source *sourceFile, aliases map[string]string, ignoredPositions map[token.Pos]struct{}) {
	typedVars := collectTypedVariables(source)

	ast.Inspect(source.file, func(node ast.Node) bool {
		switch n := node.(type) {
		case nil:
			return true
		case *ast.ImportSpec:
			return false
		case *ast.CallExpr:
			if !isReflectiveCall(n.Fun) {
				return true
			}
			for _, arg := range n.Args {
				if typeName := reflectiveArgTypeName(source, arg, typedVars); typeName != "" {
					pkg.reflectiveStructs[typeName] = append(pkg.reflectiveStructs[typeName], useAt(source, arg.Pos()))
				}
			}
		case *ast.SelectorExpr:
			use := useAt(source, n.Sel.Pos())
			pkg.selectorNames[n.Sel.Name] = append(pkg.selectorNames[n.Sel.Name], use)
			if ident, ok := n.X.(*ast.Ident); ok {
				if importPath := aliases[ident.Name]; importPath != "" {
					addPackageSymbolUse(pkg.usedPackageSymbols, importPath, n.Sel.Name, use)
				}
			}
		case *ast.KeyValueExpr:
			if ident, ok := n.Key.(*ast.Ident); ok {
				pkg.fieldKeys[ident.Name] = append(pkg.fieldKeys[ident.Name], useAt(source, ident.Pos()))
			}
		case *ast.Ident:
			if n.Name == blankImportName {
				return true
			}
			if _, ignored := ignoredPositions[n.Pos()]; ignored {
				return true
			}
			pkg.usedNames[n.Name] = append(pkg.usedNames[n.Name], useAt(source, n.Pos()))
		}

		return true
	})
}

func collectTypedVariables(source *sourceFile) map[string][]typedVariable {
	typedVars := make(map[string][]typedVariable)

	ast.Inspect(source.file, func(node ast.Node) bool {
		switch n := node.(type) {
		case nil:
			return true
		case *ast.ValueSpec:
			collectValueSpecTypedVariables(source, n, typedVars)
		case *ast.AssignStmt:
			collectAssignmentTypedVariables(source, n, typedVars)
		}

		return true
	})

	return typedVars
}

func collectValueSpecTypedVariables(source *sourceFile, spec *ast.ValueSpec, typedVars map[string][]typedVariable) {
	typeName := exprTypeName(spec.Type)
	if typeName == "" {
		return
	}
	for _, name := range spec.Names {
		addTypedVariable(source, typedVars, name.Name, typeName, name.Pos())
	}
}

func collectAssignmentTypedVariables(source *sourceFile, stmt *ast.AssignStmt, typedVars map[string][]typedVariable) {
	for i, rhs := range stmt.Rhs {
		if i >= len(stmt.Lhs) {
			continue
		}
		ident, ok := stmt.Lhs[i].(*ast.Ident)
		if !ok {
			continue
		}
		literal, ok := rhs.(*ast.CompositeLit)
		if !ok {
			continue
		}
		if typeName := exprTypeName(literal.Type); typeName != "" {
			addTypedVariable(source, typedVars, ident.Name, typeName, ident.Pos())
		}
	}
}

func addTypedVariable(source *sourceFile, typedVars map[string][]typedVariable, name string, typeName string, pos token.Pos) {
	typedVars[name] = append(typedVars[name], typedVariable{
		TypeName: typeName,
		Line:     source.fset.Position(pos).Line,
	})
}

func isReflectiveCall(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.Ident:
		return isReflectiveCallName(e.Name)
	case *ast.SelectorExpr:
		return isReflectiveCallName(e.Sel.Name)
	default:
		return false
	}
}

func isReflectiveCallName(name string) bool {
	switch name {
	case decodeCallName, unmarshalCallName, unmarshalJSONCallName, scanCallName, structScanCallName:
		return true
	default:
		return false
	}
}

func reflectiveArgTypeName(source *sourceFile, expr ast.Expr, typedVars map[string][]typedVariable) string {
	unary, ok := expr.(*ast.UnaryExpr)
	if !ok || unary.Op != token.AND {
		return ""
	}

	switch e := unary.X.(type) {
	case *ast.Ident:
		return latestTypedVariable(typedVars[e.Name], source.fset.Position(e.Pos()).Line)
	case *ast.CompositeLit:
		return exprTypeName(e.Type)
	default:
		return ""
	}
}

func latestTypedVariable(vars []typedVariable, useLine int) string {
	latestLine := noLine
	latestType := ""
	for _, variable := range vars {
		if variable.Line > useLine || variable.Line < latestLine {
			continue
		}
		latestLine = variable.Line
		latestType = variable.TypeName
	}

	return latestType
}

func addPackageSymbolUse(uses map[string]map[string][]codeUse, importPath string, symbol string, use codeUse) {
	if uses[importPath] == nil {
		uses[importPath] = make(map[string][]codeUse)
	}
	uses[importPath][symbol] = append(uses[importPath][symbol], use)
}

func useAt(source *sourceFile, pos token.Pos) codeUse {
	return codeUse{
		File: source.relPath,
		Line: source.fset.Position(pos).Line,
	}
}

func lineRange(fset *token.FileSet, node ast.Node) (int, int) {
	return fset.Position(node.Pos()).Line, fset.Position(node.End()).Line
}

func isSpecialFunction(pkg *packageState, source *sourceFile, name string) bool {
	if name == initFunctionName {
		return true
	}
	if pkg.name == mainPackageName && name == mainFunctionName {
		return true
	}
	if source.test && isTestEntryPoint(name) {
		return true
	}

	return false
}

func isTestEntryPoint(name string) bool {
	return strings.HasPrefix(name, testFunctionPrefix) ||
		strings.HasPrefix(name, benchmarkFunctionPref) ||
		strings.HasPrefix(name, fuzzFunctionPrefix) ||
		strings.HasPrefix(name, exampleFunctionPrefix)
}

func receiverTypeName(fields *ast.FieldList) string {
	if fields == nil || len(fields.List) == 0 {
		return ""
	}

	return exprTypeName(fields.List[0].Type)
}

func exprTypeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return exprTypeName(e.X)
	case *ast.ParenExpr:
		return exprTypeName(e.X)
	case *ast.IndexExpr:
		return exprTypeName(e.X)
	case *ast.IndexListExpr:
		return exprTypeName(e.X)
	case *ast.SelectorExpr:
		return e.Sel.Name
	default:
		return ""
	}
}

func collectIdentPositions(node ast.Node, positions map[token.Pos]struct{}) {
	ast.Inspect(node, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if ok {
			positions[ident.Pos()] = struct{}{}
		}

		return true
	})
}

func deadCodeFindings(module moduleState) []Finding {
	if len(module.packages) == 0 {
		return nil
	}

	packageByImport := normalPackageIndexes(module.packages)
	reachable := reachablePackages(module, packageByImport)
	usage := collectModuleUsage(module.packages, reachable)
	reflectiveStructs := reflectiveStructNamesByPackage(module.packages)
	deadStructs := deadStructNames(module.packages, reachable, usage, reflectiveStructs)
	fileHasLiveDeclaration := make(map[string]bool)
	findings := make([]Finding, 0)

	findings = append(findings, unusedDeclarationFindings(module.packages, reachable, usage, reflectiveStructs, deadStructs, fileHasLiveDeclaration)...)
	findings = append(findings, unusedFileFindings(module.packages, reachable, fileHasLiveDeclaration)...)

	return findings
}

func unusedDeclarationFindings(packages []packageState, reachable map[int]bool, usage moduleUsage, reflectiveStructs map[int]map[string]struct{}, deadStructs map[string]bool, fileHasLiveDeclaration map[string]bool) []Finding {
	findings := make([]Finding, 0)
	for i, pkg := range packages {
		if pkg.isExternalTest {
			continue
		}
		if !reachable[i] {
			findings = append(findings, unusedPackageFinding(pkg))
			continue
		}

		findings = append(findings, unusedDeclarationsInPackage(pkg, i, usage, reflectiveStructs[i], deadStructs, fileHasLiveDeclaration)...)
	}

	return findings
}

func unusedDeclarationsInPackage(pkg packageState, packageIndex int, usage moduleUsage, reflectiveStructs map[string]struct{}, deadStructs map[string]bool, fileHasLiveDeclaration map[string]bool) []Finding {
	findings := make([]Finding, 0)
	for _, decl := range pkg.declarations {
		if !shouldCheckDeclaration(decl) {
			continue
		}
		if declarationBlockedByDeadStruct(packageIndex, decl, deadStructs) {
			continue
		}
		if declarationUsed(pkg, decl, usage, reflectiveStructs) {
			fileHasLiveDeclaration[decl.File] = true
			continue
		}
		findings = append(findings, declarationFinding(pkg, decl))
	}

	return findings
}

func shouldCheckDeclaration(decl declaration) bool {
	return decl.Kind != declarationField || !decl.HasTag
}

func unusedFileFindings(packages []packageState, reachable map[int]bool, fileHasLiveDeclaration map[string]bool) []Finding {
	findings := make([]Finding, 0)
	for i, pkg := range packages {
		if pkg.isExternalTest || !reachable[i] {
			continue
		}
		for _, source := range pkg.files {
			if source.hasSpecialFunction || source.hasSideEffectImport || source.hasTopLevelValue || !source.hasDeclarations {
				continue
			}
			if fileHasLiveDeclaration[source.relPath] {
				continue
			}
			findings = append(findings, Finding{
				Type:       FindingUnusedFile,
				Package:    pkg.importPath,
				ImportPath: pkg.importPath,
				File:       source.relPath,
				Line:       firstLine,
			})
		}
	}

	return findings
}

func declarationBlockedByDeadStruct(packageIndex int, decl declaration, deadStructs map[string]bool) bool {
	switch decl.Kind {
	case declarationMethod:
		return deadStructs[packageSymbolKey(packageIndex, decl.Receiver)]
	case declarationField:
		return deadStructs[packageSymbolKey(packageIndex, decl.Struct)]
	default:
		return false
	}
}

func reachablePackages(module moduleState, packageByImport map[string]int) map[int]bool {
	importers := packageImportCounts(module.packages, packageByImport)
	roots := rootPackageIndexes(module, importers)
	return walkReachablePackages(module.packages, packageByImport, roots)
}

func packageImportCounts(packages []packageState, packageByImport map[string]int) map[int]int {
	importers := make(map[int]int, len(packages))
	for _, pkg := range packages {
		for importPath := range pkg.internalImports {
			if importedIndex, ok := packageByImport[importPath]; ok {
				importers[importedIndex]++
			}
		}
	}
	return importers
}

func rootPackageIndexes(module moduleState, importers map[int]int) map[int]struct{} {
	roots := make(map[int]struct{})
	for i, pkg := range module.packages {
		if pkg.isExternalTest || pkg.name == mainPackageName || pkg.realImportPath == module.report.ModulePath {
			roots[i] = struct{}{}
		}
	}
	if len(roots) == 0 {
		for i, pkg := range module.packages {
			if pkg.isExternalTest {
				continue
			}
			if importers[i] == 0 {
				roots[i] = struct{}{}
			}
		}
	}
	return roots
}

func walkReachablePackages(packages []packageState, packageByImport map[string]int, roots map[int]struct{}) map[int]bool {
	reachable := make(map[int]bool, len(packages))
	queue := make([]int, 0, len(roots))
	for root := range roots {
		reachable[root] = true
		queue = append(queue, root)
	}

	for len(queue) > 0 {
		pkgIndex := queue[0]
		queue = queue[1:]
		for importPath := range packages[pkgIndex].internalImports {
			importedIndex, ok := packageByImport[importPath]
			if !ok || reachable[importedIndex] {
				continue
			}
			reachable[importedIndex] = true
			queue = append(queue, importedIndex)
		}
	}

	return reachable
}

func collectModuleUsage(packages []packageState, reachable map[int]bool) moduleUsage {
	usage := moduleUsage{
		selectorUses:      make(map[string][]codeUse),
		fieldKeyUses:      make(map[string][]codeUse),
		packageSymbolUses: make(map[string]map[string][]codeUse),
		dotImports:        make(map[string]struct{}),
		interfaceMethods:  make(map[string]struct{}),
	}

	for i, pkg := range packages {
		if !reachable[i] {
			continue
		}
		for name, uses := range pkg.selectorNames {
			usage.selectorUses[name] = append(usage.selectorUses[name], uses...)
		}
		for name, uses := range pkg.fieldKeys {
			usage.fieldKeyUses[name] = append(usage.fieldKeyUses[name], uses...)
		}
		for importPath, symbols := range pkg.usedPackageSymbols {
			if usage.packageSymbolUses[importPath] == nil {
				usage.packageSymbolUses[importPath] = make(map[string][]codeUse)
			}
			for symbol, uses := range symbols {
				usage.packageSymbolUses[importPath][symbol] = append(usage.packageSymbolUses[importPath][symbol], uses...)
			}
		}
		for _, importPath := range pkg.dotImports {
			usage.dotImports[importPath] = struct{}{}
		}
		for name := range pkg.interfaceMethods {
			usage.interfaceMethods[name] = struct{}{}
		}
	}

	return usage
}

func reflectiveStructNamesByPackage(packages []packageState) map[int]map[string]struct{} {
	all := make(map[int]map[string]struct{}, len(packages))
	for i, pkg := range packages {
		all[i] = reflectiveStructNames(pkg)
	}

	return all
}

func reflectiveStructNames(pkg packageState) map[string]struct{} {
	reflective := make(map[string]struct{}, len(pkg.reflectiveStructs))
	for name := range pkg.reflectiveStructs {
		reflective[name] = struct{}{}
	}

	changed := true
	for changed {
		changed = false
		for _, decl := range pkg.declarations {
			if decl.Kind != declarationField || decl.FieldType == "" {
				continue
			}
			if _, ok := reflective[decl.Struct]; !ok {
				continue
			}
			if _, ok := reflective[decl.FieldType]; ok {
				continue
			}
			reflective[decl.FieldType] = struct{}{}
			changed = true
		}
	}

	return reflective
}

func deadStructNames(packages []packageState, reachable map[int]bool, usage moduleUsage, reflectiveStructs map[int]map[string]struct{}) map[string]bool {
	dead := make(map[string]bool)
	for i, pkg := range packages {
		if pkg.isExternalTest || !reachable[i] {
			continue
		}
		for _, decl := range pkg.declarations {
			if decl.Kind != declarationStruct {
				continue
			}
			if !declarationUsed(pkg, decl, usage, reflectiveStructs[i]) {
				dead[packageSymbolKey(i, decl.Name)] = true
			}
		}
	}

	return dead
}

func packageSymbolKey(packageIndex int, symbol string) string {
	return strconv.Itoa(packageIndex) + packageKeySeparator + symbol
}

func declarationUsed(pkg packageState, decl declaration, usage moduleUsage, reflectiveStructs map[string]struct{}) bool {
	if decl.Special {
		return true
	}
	if decl.Kind == declarationFunction && decl.RuntimeChecked {
		return decl.RuntimeReachable
	}
	if decl.Kind == declarationMethod && decl.RuntimeChecked {
		if decl.RuntimeReachable {
			return true
		}
		_, implementsInterface := usage.interfaceMethods[decl.Name]
		return implementsInterface
	}
	if decl.Typed {
		return typedDeclarationUsed(pkg, decl, usage, reflectiveStructs)
	}

	switch decl.Kind {
	case declarationFunction, declarationStruct, declarationInterface, declarationType, declarationVar, declarationConst:
		return valueDeclarationUsed(pkg, decl, usage, reflectiveStructs)
	case declarationMethod:
		return methodDeclarationUsed(decl, usage)
	case declarationField:
		return fieldDeclarationUsed(decl, usage, reflectiveStructs)
	}

	return false
}

func valueDeclarationUsed(pkg packageState, decl declaration, usage moduleUsage, reflectiveStructs map[string]struct{}) bool {
	if decl.SideEffect {
		return true
	}
	if _, ok := reflectiveStructs[decl.Name]; ok {
		return true
	}
	if hasUseOutside(pkg.usedNames[decl.Name], decl) {
		return true
	}
	if hasUseOutside(usage.packageSymbolUses[pkg.realImportPath][decl.Name], decl) {
		return true
	}
	_, dotImported := usage.dotImports[pkg.realImportPath]
	return dotImported && ast.IsExported(decl.Name)
}

func methodDeclarationUsed(decl declaration, usage moduleUsage) bool {
	if _, ok := usage.interfaceMethods[decl.Name]; ok {
		return true
	}
	return hasUseOutside(usage.selectorUses[decl.Name], decl)
}

func fieldDeclarationUsed(decl declaration, usage moduleUsage, reflectiveStructs map[string]struct{}) bool {
	if _, ok := reflectiveStructs[decl.Struct]; ok {
		return true
	}
	if hasUseOutside(usage.selectorUses[decl.Name], decl) {
		return true
	}
	return hasUseOutside(usage.fieldKeyUses[decl.Name], decl)
}

func typedDeclarationUsed(pkg packageState, decl declaration, usage moduleUsage, reflectiveStructs map[string]struct{}) bool {
	if decl.SideEffect {
		return true
	}
	switch decl.Kind {
	case declarationFunction, declarationStruct, declarationInterface, declarationType, declarationVar, declarationConst:
		if _, ok := reflectiveStructs[decl.Name]; ok {
			return true
		}
		return decl.TypedUsed
	case declarationMethod:
		return decl.TypedUsed
	case declarationField:
		if _, ok := reflectiveStructs[decl.Struct]; ok {
			return true
		}
		return decl.TypedUsed
	default:
		return false
	}
}

func hasUseOutside(uses []codeUse, decl declaration) bool {
	for _, use := range uses {
		if use.File != decl.File || use.Line < decl.Line || use.Line > decl.EndLine {
			return true
		}
	}

	return false
}

func unusedPackageFinding(pkg packageState) Finding {
	file := pkg.relDir
	line := firstLine
	if len(pkg.files) > 0 {
		file = pkg.files[0].relPath
	}

	return Finding{
		Type:       FindingUnusedPackage,
		Package:    pkg.importPath,
		ImportPath: pkg.importPath,
		File:       file,
		Line:       line,
	}
}

func declarationFinding(pkg packageState, decl declaration) Finding {
	finding := Finding{
		Type:       findingTypeForDeclaration(decl),
		Package:    pkg.importPath,
		ImportPath: pkg.importPath,
		Symbol:     decl.Name,
		Receiver:   decl.Receiver,
		Struct:     decl.Struct,
		File:       decl.File,
		Line:       decl.Line,
	}

	return finding
}

func findingTypeForDeclaration(decl declaration) string {
	switch decl.Kind {
	case declarationFunction:
		return FindingUnusedFunction
	case declarationMethod:
		return FindingUnusedMethod
	case declarationStruct:
		return FindingUnusedStruct
	case declarationInterface:
		return FindingUnusedInterface
	case declarationType:
		return FindingUnusedType
	case declarationVar:
		return FindingUnusedVar
	case declarationConst:
		return FindingUnusedConst
	case declarationField:
		return FindingUnusedField
	default:
		return FindingUnusedFile
	}
}
