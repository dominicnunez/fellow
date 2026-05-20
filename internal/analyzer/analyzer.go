package analyzer

import (
	"crypto/sha256"
	"encoding/hex"
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
	FindingUnusedInterface    = "unused-interface"
	FindingUnusedType         = "unused-type"
	FindingUnusedVar          = "unused-var"
	FindingUnusedConst        = "unused-const"
	FindingUnusedField        = "unused-field"
	FindingComplexity         = "complexity"
	FindingDuplicateCode      = "duplicate-code"
	FindingTidyDrift          = "tidy-drift"
	FindingLocalReplace       = "local-replace"
	FindingToolDependency     = "tool-dependency"

	goModFileName  = "go.mod"
	goFileSuffix   = ".go"
	testFileSuffix = "_test.go"
	cgoImportPath  = "C"

	generatedHeaderScanLimit = 2048
	generatedMarker          = "Code generated"
	generatedDoNotEditMarker = "DO NOT EDIT"
	ruleOffSeverity          = "off"
	fingerprintLength        = 16
)

type Options struct {
	Root               string
	IncludeTests       bool
	IncludeGenerated   bool
	CheckIndirect      bool
	Rules              map[string]string
	IgnorePatterns     []string
	IgnoreFindings     []FindingMatcher
	WorkspacePatterns  []string
	BuildTags          []string
	MaxCyclomatic      int
	MaxCognitive       int
	CheckModuleHygiene bool
}

type FindingMatcher struct {
	Type        string
	File        string
	Package     string
	Module      string
	ImportPath  string
	Symbol      string
	Receiver    string
	Struct      string
	Fingerprint string
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
	SuppressedFindings   int `json:"suppressed_findings,omitempty"`
	SuppressedByBaseline int `json:"suppressed_by_baseline,omitempty"`
	SkippedModules       int `json:"skipped_modules,omitempty"`
	CoveredFindings      int `json:"covered_findings,omitempty"`
	UncoveredFindings    int `json:"uncovered_findings,omitempty"`
	UnusedDependencies   int `json:"unused_dependencies"`
	UnlistedDependencies int `json:"unlisted_dependencies"`
	TestOnlyDependencies int `json:"test_only_dependencies"`
	UnusedPackages       int `json:"unused_packages"`
	UnusedFiles          int `json:"unused_files"`
	UnusedFunctions      int `json:"unused_functions"`
	UnusedMethods        int `json:"unused_methods"`
	UnusedStructs        int `json:"unused_structs"`
	UnusedInterfaces     int `json:"unused_interfaces"`
	UnusedTypes          int `json:"unused_types"`
	UnusedVars           int `json:"unused_vars"`
	UnusedConsts         int `json:"unused_consts"`
	UnusedFields         int `json:"unused_fields"`
	ComplexityFindings   int `json:"complexity_findings"`
	DuplicateGroups      int `json:"duplicate_groups"`
	DuplicatedLines      int `json:"duplicated_lines"`
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
	Fingerprint   string      `json:"fingerprint"`
	Metrics       Metrics     `json:"metrics,omitempty"`
	Coverage      *Coverage   `json:"coverage,omitempty"`
	Locations     []Location  `json:"locations,omitempty"`
	Lines         int         `json:"lines,omitempty"`
	Indirect      bool        `json:"indirect,omitempty"`
	UsedIn        []ImportUse `json:"used_in,omitempty"`
	UsedInModules []string    `json:"used_in_modules,omitempty"`
}

type Metrics struct {
	Cyclomatic int `json:"cyclomatic,omitempty"`
	Cognitive  int `json:"cognitive,omitempty"`
}

type Coverage struct {
	Covered bool `json:"covered"`
	Count   int  `json:"count,omitempty"`
}

type Location struct {
	File      string `json:"file"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

type ImportUse struct {
	ImportPath string `json:"import_path"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	Test       bool   `json:"test,omitempty"`
	Generated  bool   `json:"generated,omitempty"`
	Tool       bool   `json:"tool,omitempty"`
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
	modules, skippedModules := filterModules(modules, opts.WorkspacePatterns)
	if len(modules) == 0 {
		return nil, fmt.Errorf("no selected go.mod files found under %s", absRoot)
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
		applyRuntimeReachability(absRoot, &modules[i], opts)
	}

	suppressedByComment := 0
	for i := range modules {
		findings, suppressed, err := analyzeModule(absRoot, modules, i, opts)
		if err != nil {
			return nil, err
		}
		modules[i].report.Findings = findings
		suppressedByComment += suppressed
	}

	report := &Report{
		SchemaVersion: SchemaVersion,
		Root:          filepath.ToSlash(absRoot),
		Modules:       make([]ModuleReport, 0, len(modules)),
	}
	for _, module := range modules {
		report.Modules = append(report.Modules, module.report)
	}
	finalizeReport(report)
	report.Summary.SuppressedFindings = suppressedByComment
	report.Summary.SkippedModules = skippedModules

	return report, nil
}

func filterModules(modules []moduleState, patterns []string) ([]moduleState, int) {
	if len(patterns) == 0 {
		return modules, 0
	}

	selected := modules[:0]
	skipped := 0
	for _, module := range modules {
		if moduleMatchesPatterns(module, patterns) {
			selected = append(selected, module)
			continue
		}
		skipped++
	}

	return selected, skipped
}

func moduleMatchesPatterns(module moduleState, patterns []string) bool {
	for _, pattern := range patterns {
		if matchPathPattern(pattern, module.report.ModulePath) || matchPathPattern(pattern, module.report.Dir) {
			return true
		}
	}

	return false
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

func analyzeModule(root string, modules []moduleState, moduleIndex int, opts Options) ([]Finding, int, error) {
	module := modules[moduleIndex]
	usedByRequire := make(map[string][]ImportUse, len(module.requires))
	unlistedByImport := make(map[string][]ImportUse)

	for _, use := range module.imports {
		if !isExternalImport(use.ImportPath) || matchesModulePath(use.ImportPath, module.report.ModulePath) {
			continue
		}

		req, ok := longestMatchingRequire(use.ImportPath, module.requires)
		if !ok {
			if use.Tool {
				continue
			}
			unlistedByImport[use.ImportPath] = append(unlistedByImport[use.ImportPath], use)
			continue
		}
		usedByRequire[req.Module] = append(usedByRequire[req.Module], use)
	}
	for _, use := range toolDependencyUses(root, module) {
		req, ok := longestMatchingRequire(use.ImportPath, module.requires)
		if ok {
			usedByRequire[req.Module] = append(usedByRequire[req.Module], use)
		}
	}

	findings := make([]Finding, 0)
	findings = append(findings, unlistedDependencyFindings(unlistedByImport)...)
	if opts.CheckModuleHygiene {
		hygieneFindings, err := moduleHygieneFindings(root, modules, module)
		if err != nil {
			return nil, 0, err
		}
		findings = append(findings, hygieneFindings...)
	}

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
	findings = append(findings, healthFindings(module, opts)...)
	findings = append(findings, duplicateFindings(module)...)
	assignFingerprints(findings)
	findings, suppressedByComment := applyCommentSuppressions(findings, module)
	findings = filterFindings(findings, opts)

	sortFindings(findings)

	return findings, suppressedByComment, nil
}

func finalizeReport(report *Report) {
	for i := range report.Modules {
		assignFingerprints(report.Modules[i].Findings)
	}
	report.Summary = summarize(report.Modules)
}

func RefreshReport(report *Report) {
	previous := report.Summary
	finalizeReport(report)
	report.Summary.SuppressedFindings = previous.SuppressedFindings
	report.Summary.SuppressedByBaseline = previous.SuppressedByBaseline
	report.Summary.SkippedModules = previous.SkippedModules
}

func assignFingerprints(findings []Finding) {
	for i := range findings {
		findings[i].Fingerprint = findingFingerprint(findings[i])
	}
}

func applyCommentSuppressions(findings []Finding, module moduleState) ([]Finding, int) {
	suppressions := moduleSuppressions(module)
	if len(suppressions) == 0 {
		return findings, 0
	}

	filtered := findings[:0]
	suppressed := 0
	for _, finding := range findings {
		fileSuppression, ok := suppressions[finding.File]
		if ok && fileSuppression.suppresses(finding) {
			suppressed++
			continue
		}
		filtered = append(filtered, finding)
	}

	return filtered, suppressed
}

type fileSuppression struct {
	file bool
	next map[int]map[string]struct{}
}

func (s fileSuppression) suppresses(finding Finding) bool {
	if s.file {
		return true
	}
	rules := s.next[finding.Line]
	if len(rules) == 0 {
		return false
	}
	if _, ok := rules[suppressAllRules]; ok {
		return true
	}
	_, ok := rules[finding.Type]
	return ok
}

func moduleSuppressions(module moduleState) map[string]fileSuppression {
	suppressions := make(map[string]fileSuppression)
	for _, pkg := range module.packages {
		for _, source := range pkg.files {
			if !source.suppressFile && len(source.suppressNextLine) == 0 {
				continue
			}
			suppressions[source.relPath] = fileSuppression{
				file: source.suppressFile,
				next: source.suppressNextLine,
			}
		}
	}

	return suppressions
}

func findingFingerprint(f Finding) string {
	parts := []string{
		f.Type,
		f.Package,
		f.Module,
		f.ImportPath,
		f.Symbol,
		f.Receiver,
		f.Struct,
		f.File,
		fmt.Sprint(f.Line),
		fmt.Sprint(f.Lines),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])[:fingerprintLength]
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
	if len(opts.Rules) == 0 && len(opts.IgnorePatterns) == 0 && len(opts.IgnoreFindings) == 0 {
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
		if ignoredByFindingMatcher(finding, opts.IgnoreFindings) {
			continue
		}
		filtered = append(filtered, finding)
	}

	return filtered
}

func ignoredByFindingMatcher(finding Finding, matchers []FindingMatcher) bool {
	for _, matcher := range matchers {
		if matcher.matches(finding) {
			return true
		}
	}

	return false
}

func (m FindingMatcher) matches(finding Finding) bool {
	return matchesString(m.Type, finding.Type) &&
		matchesPath(m.File, finding.File) &&
		matchesString(m.Package, finding.Package) &&
		matchesString(m.Module, finding.Module) &&
		matchesString(m.ImportPath, finding.ImportPath) &&
		matchesString(m.Symbol, finding.Symbol) &&
		matchesString(m.Receiver, finding.Receiver) &&
		matchesString(m.Struct, finding.Struct) &&
		matchesString(m.Fingerprint, finding.Fingerprint)
}

func matchesString(pattern string, value string) bool {
	return pattern == "" || pattern == value
}

func matchesPath(pattern string, value string) bool {
	return pattern == "" || matchPathPattern(pattern, value)
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
			summary.addFinding(finding)
		}
	}

	return summary
}

func (s *Summary) addFinding(finding Finding) {
	s.Findings++
	s.addCoverage(finding.Coverage)
	s.addFindingType(finding)
}

func (s *Summary) addCoverage(coverage *Coverage) {
	if coverage == nil {
		return
	}
	if coverage.Covered {
		s.CoveredFindings++
		return
	}
	s.UncoveredFindings++
}

func (s *Summary) addFindingType(finding Finding) {
	switch finding.Type {
	case FindingUnusedDependency:
		s.UnusedDependencies++
	case FindingUnlistedDependency:
		s.UnlistedDependencies++
	case FindingTestOnlyDependency:
		s.TestOnlyDependencies++
	case FindingUnusedPackage:
		s.UnusedPackages++
	case FindingUnusedFile:
		s.UnusedFiles++
	case FindingUnusedFunction:
		s.UnusedFunctions++
	case FindingUnusedMethod:
		s.UnusedMethods++
	case FindingUnusedStruct:
		s.UnusedStructs++
	case FindingUnusedInterface:
		s.UnusedInterfaces++
	case FindingUnusedType:
		s.UnusedTypes++
	case FindingUnusedVar:
		s.UnusedVars++
	case FindingUnusedConst:
		s.UnusedConsts++
	case FindingUnusedField:
		s.UnusedFields++
	case FindingComplexity:
		s.ComplexityFindings++
	case FindingDuplicateCode:
		s.DuplicateGroups++
		s.DuplicatedLines += finding.Lines
	}
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
	case ".git", ".gallow", "node_modules", "vendor":
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
