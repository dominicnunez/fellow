package analyzer

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
)

const (
	SchemaVersion = "1"

	FindingUnusedDependency   = "unused-dependency"
	FindingUnlistedDependency = "unlisted-dependency"
	FindingTestOnlyDependency = "test-only-dependency"
	FindingUnusedPackage      = "unused-package"
	FindingUnusedFile         = "unused-file"
	FindingUnusedFunction     = "unused-function"
	FindingUnusedMethod       = "unused-method"
	FindingUnusedStruct       = "unused-struct"
	FindingUnusedField        = "unused-field"

	goModFileName  = "go.mod"
	goFileSuffix   = ".go"
	testFileSuffix = "_test.go"
	cgoImportPath  = "C"

	generatedHeaderScanLimit = 2048
	generatedMarker          = "Code generated"
	generatedDoNotEditMarker = "DO NOT EDIT"
	ruleOffSeverity          = "off"
)

type Options struct {
	Root             string
	IncludeTests     bool
	IncludeGenerated bool
	CheckIndirect    bool
	Rules            map[string]string
	IgnorePatterns   []string
}

type Report struct {
	SchemaVersion string         `json:"schema_version"`
	Root          string         `json:"root"`
	Modules       []ModuleReport `json:"modules"`
	Summary       Summary        `json:"summary"`
}

type Summary struct {
	Modules              int `json:"modules"`
	Findings             int `json:"findings"`
	UnusedDependencies   int `json:"unused_dependencies"`
	UnlistedDependencies int `json:"unlisted_dependencies"`
	TestOnlyDependencies int `json:"test_only_dependencies"`
	UnusedPackages       int `json:"unused_packages"`
	UnusedFiles          int `json:"unused_files"`
	UnusedFunctions      int `json:"unused_functions"`
	UnusedMethods        int `json:"unused_methods"`
	UnusedStructs        int `json:"unused_structs"`
	UnusedFields         int `json:"unused_fields"`
}

type ModuleReport struct {
	ModulePath string    `json:"module_path"`
	Dir        string    `json:"dir"`
	GoMod      string    `json:"go_mod"`
	Findings   []Finding `json:"findings"`
}

type Finding struct {
	Type          string      `json:"type"`
	Package       string      `json:"package,omitempty"`
	Module        string      `json:"module,omitempty"`
	Version       string      `json:"version,omitempty"`
	ImportPath    string      `json:"import_path,omitempty"`
	Symbol        string      `json:"symbol,omitempty"`
	Receiver      string      `json:"receiver,omitempty"`
	Struct        string      `json:"struct,omitempty"`
	File          string      `json:"file"`
	Line          int         `json:"line"`
	Indirect      bool        `json:"indirect,omitempty"`
	UsedIn        []ImportUse `json:"used_in,omitempty"`
	UsedInModules []string    `json:"used_in_modules,omitempty"`
}

type ImportUse struct {
	ImportPath string `json:"import_path"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	Test       bool   `json:"test,omitempty"`
	Generated  bool   `json:"generated,omitempty"`
}

type Require struct {
	Module   string `json:"module"`
	Version  string `json:"version"`
	Indirect bool   `json:"indirect"`
	File     string `json:"file"`
	Line     int    `json:"line"`
}

type moduleState struct {
	report   ModuleReport
	absDir   string
	requires []Require
	imports  []ImportUse
	packages []packageState
}

func Analyze(opts Options) (*Report, error) {
	root := opts.Root
	if root == "" {
		root = "."
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve root: %w", err)
	}

	modules, err := discoverModules(absRoot)
	if err != nil {
		return nil, err
	}
	if len(modules) == 0 {
		return nil, fmt.Errorf("no go.mod files found under %s", absRoot)
	}

	moduleDirs := make(map[string]struct{}, len(modules))
	for _, module := range modules {
		moduleDirs[module.absDir] = struct{}{}
	}

	for i := range modules {
		packages, imports, err := scanModulePackages(absRoot, modules[i], moduleDirs, opts)
		if err != nil {
			return nil, fmt.Errorf("scan %s: %w", modules[i].report.Dir, err)
		}
		modules[i].packages = packages
		modules[i].imports = imports
		applyTypedInfo(absRoot, &modules[i], opts)
	}

	for i := range modules {
		modules[i].report.Findings = analyzeModule(absRoot, modules, i, opts)
	}

	report := &Report{
		SchemaVersion: SchemaVersion,
		Root:          filepath.ToSlash(absRoot),
		Modules:       make([]ModuleReport, 0, len(modules)),
	}
	for _, module := range modules {
		report.Modules = append(report.Modules, module.report)
	}
	report.Summary = summarize(report.Modules)

	return report, nil
}

func discoverModules(root string) ([]moduleState, error) {
	var modules []moduleState

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() && path != root && shouldSkipDir(d.Name()) {
			return filepath.SkipDir
		}

		if d.IsDir() || d.Name() != goModFileName {
			return nil
		}

		module, err := parseModule(root, path)
		if err != nil {
			return err
		}
		modules = append(modules, module)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover modules: %w", err)
	}

	sort.Slice(modules, func(i, j int) bool {
		return modules[i].absDir < modules[j].absDir
	})

	return modules, nil
}

func parseModule(root string, goModPath string) (moduleState, error) {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return moduleState{}, fmt.Errorf("read %s: %w", goModPath, err)
	}

	parsed, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return moduleState{}, fmt.Errorf("parse %s: %w", goModPath, err)
	}
	if parsed.Module == nil {
		return moduleState{}, fmt.Errorf("%s has no module directive", goModPath)
	}

	dir := filepath.Dir(goModPath)
	module := moduleState{
		absDir: dir,
		report: ModuleReport{
			ModulePath: parsed.Module.Mod.Path,
			Dir:        relPath(root, dir),
			GoMod:      relPath(root, goModPath),
		},
		requires: make([]Require, 0, len(parsed.Require)),
	}

	for _, req := range parsed.Require {
		line := 0
		if req.Syntax != nil {
			line = req.Syntax.Start.Line
		}
		module.requires = append(module.requires, Require{
			Module:   req.Mod.Path,
			Version:  req.Mod.Version,
			Indirect: req.Indirect,
			File:     relPath(root, goModPath),
			Line:     line,
		})
	}

	sort.Slice(module.requires, func(i, j int) bool {
		if module.requires[i].Line == module.requires[j].Line {
			return module.requires[i].Module < module.requires[j].Module
		}
		return module.requires[i].Line < module.requires[j].Line
	})

	return module, nil
}

func analyzeModule(root string, modules []moduleState, moduleIndex int, opts Options) []Finding {
	module := modules[moduleIndex]
	usedByRequire := make(map[string][]ImportUse, len(module.requires))
	unlistedByImport := make(map[string][]ImportUse)

	for _, use := range module.imports {
		if !isExternalImport(use.ImportPath) || matchesModulePath(use.ImportPath, module.report.ModulePath) {
			continue
		}

		req, ok := longestMatchingRequire(use.ImportPath, module.requires)
		if !ok {
			unlistedByImport[use.ImportPath] = append(unlistedByImport[use.ImportPath], use)
			continue
		}
		usedByRequire[req.Module] = append(usedByRequire[req.Module], use)
	}

	findings := make([]Finding, 0)
	findings = append(findings, unlistedDependencyFindings(unlistedByImport)...)

	for _, req := range module.requires {
		if req.Indirect && !opts.CheckIndirect {
			continue
		}

		uses := usedByRequire[req.Module]
		if len(uses) == 0 {
			findings = append(findings, Finding{
				Type:          FindingUnusedDependency,
				Module:        req.Module,
				Version:       req.Version,
				File:          req.File,
				Line:          req.Line,
				Indirect:      req.Indirect,
				UsedInModules: usedInOtherModules(root, modules, moduleIndex, req.Module),
			})
			continue
		}

		if usesOnlyTests(uses) {
			findings = append(findings, Finding{
				Type:     FindingTestOnlyDependency,
				Module:   req.Module,
				Version:  req.Version,
				File:     req.File,
				Line:     req.Line,
				Indirect: req.Indirect,
				UsedIn:   uses,
			})
		}
	}
	findings = append(findings, deadCodeFindings(module)...)
	findings = filterFindings(findings, opts)

	sortFindings(findings)

	return findings
}

func unlistedDependencyFindings(unlistedByImport map[string][]ImportUse) []Finding {
	imports := make([]string, 0, len(unlistedByImport))
	for importPath := range unlistedByImport {
		imports = append(imports, importPath)
	}
	sort.Strings(imports)

	findings := make([]Finding, 0, len(imports))
	for _, importPath := range imports {
		uses := unlistedByImport[importPath]
		sortImportUses(uses)
		firstUse := uses[0]
		findings = append(findings, Finding{
			Type:       FindingUnlistedDependency,
			ImportPath: importPath,
			File:       firstUse.File,
			Line:       firstUse.Line,
			UsedIn:     uses,
		})
	}

	return findings
}

func filterFindings(findings []Finding, opts Options) []Finding {
	if len(opts.Rules) == 0 && len(opts.IgnorePatterns) == 0 {
		return findings
	}

	filtered := findings[:0]
	for _, finding := range findings {
		if opts.Rules[finding.Type] == ruleOffSeverity {
			continue
		}
		if ignoredByPattern(finding.File, opts.IgnorePatterns) {
			continue
		}
		filtered = append(filtered, finding)
	}

	return filtered
}

func ignoredByPattern(file string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchPathPattern(pattern, file) {
			return true
		}
	}

	return false
}

func matchPathPattern(pattern string, file string) bool {
	pattern = normalizePatternPath(pattern)
	file = normalizePatternPath(file)
	if pattern == file {
		return true
	}
	if ok, _ := path.Match(pattern, file); ok {
		return true
	}
	if !strings.Contains(pattern, "/") {
		if ok, _ := path.Match(pattern, path.Base(file)); ok {
			return true
		}
	}

	matched, err := regexp.MatchString(globRegex(pattern), file)
	return err == nil && matched
}

func normalizePatternPath(value string) string {
	value = filepath.ToSlash(value)
	value = strings.TrimPrefix(value, "./")
	return value
}

func globRegex(pattern string) string {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		ch := pattern[i]
		switch ch {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				b.WriteString(".*")
				i++
				continue
			}
			b.WriteString("[^/]*")
		case '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(ch)))
		}
	}
	b.WriteString("$")

	return b.String()
}

func usedInOtherModules(root string, modules []moduleState, moduleIndex int, modulePath string) []string {
	usedIn := make([]string, 0)
	for i, module := range modules {
		if i == moduleIndex {
			continue
		}

		for _, use := range module.imports {
			if matchesModulePath(use.ImportPath, modulePath) {
				usedIn = append(usedIn, relPath(root, module.absDir))
				break
			}
		}
	}

	sort.Strings(usedIn)
	return usedIn
}

func summarize(modules []ModuleReport) Summary {
	summary := Summary{Modules: len(modules)}

	for _, module := range modules {
		for _, finding := range module.Findings {
			summary.Findings++
			switch finding.Type {
			case FindingUnusedDependency:
				summary.UnusedDependencies++
			case FindingUnlistedDependency:
				summary.UnlistedDependencies++
			case FindingTestOnlyDependency:
				summary.TestOnlyDependencies++
			case FindingUnusedPackage:
				summary.UnusedPackages++
			case FindingUnusedFile:
				summary.UnusedFiles++
			case FindingUnusedFunction:
				summary.UnusedFunctions++
			case FindingUnusedMethod:
				summary.UnusedMethods++
			case FindingUnusedStruct:
				summary.UnusedStructs++
			case FindingUnusedField:
				summary.UnusedFields++
			}
		}
	}

	return summary
}

func longestMatchingRequire(importPath string, requires []Require) (Require, bool) {
	var match Require
	matched := false

	for _, req := range requires {
		if !matchesModulePath(importPath, req.Module) {
			continue
		}
		if !matched || len(req.Module) > len(match.Module) {
			match = req
			matched = true
		}
	}

	return match, matched
}

func matchesModulePath(importPath string, modulePath string) bool {
	return importPath == modulePath || strings.HasPrefix(importPath, modulePath+"/")
}

func isExternalImport(importPath string) bool {
	if importPath == "" || importPath == cgoImportPath {
		return false
	}

	firstSegment, _, _ := strings.Cut(importPath, "/")
	return strings.Contains(firstSegment, ".")
}

func usesOnlyTests(uses []ImportUse) bool {
	if len(uses) == 0 {
		return false
	}

	for _, use := range uses {
		if !use.Test {
			return false
		}
	}

	return true
}

func shouldSkipDir(name string) bool {
	switch name {
	case ".git", ".fellow", "node_modules", "vendor":
		return true
	default:
		return false
	}
}

func isGeneratedSource(src []byte) bool {
	limit := generatedHeaderScanLimit
	if len(src) < limit {
		limit = len(src)
	}

	header := string(src[:limit])
	return strings.Contains(header, generatedMarker) && strings.Contains(header, generatedDoNotEditMarker)
}

func relPath(root string, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	if rel == "." {
		return rel
	}
	return filepath.ToSlash(rel)
}

func sortFindings(findings []Finding) {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].File == findings[j].File {
			if findings[i].Line == findings[j].Line {
				return findings[i].Type < findings[j].Type
			}
			return findings[i].Line < findings[j].Line
		}
		return findings[i].File < findings[j].File
	})
}

func sortImportUses(uses []ImportUse) {
	sort.Slice(uses, func(i, j int) bool {
		if uses[i].File == uses[j].File {
			return uses[i].Line < uses[j].Line
		}
		return uses[i].File < uses[j].File
	})
}
