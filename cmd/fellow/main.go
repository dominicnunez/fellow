package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"fellow/internal/analyzer"
	"fellow/internal/settings"
)

const (
	appName = "fellow"
	version = "0.1.0"

	exitOK     = 0
	exitIssues = 1
	exitError  = 2

	formatHuman             = "human"
	formatJSON              = "json"
	formatSARIF             = "sarif"
	formatCodeClimate       = "codeclimate"
	formatGitLabCodeQuality = "gitlab-codequality"
	formatAnnotations       = "annotations"
)

type cliOptions struct {
	command          string
	root             string
	configPath       string
	baselinePath     string
	saveBaselinePath string
	coveragePath     string
	baseRef          string
	workspaceCSV     string
	tagsCSV          string
	outputFormat     string
	maxCyclomatic    int
	maxCognitive     int
	production       bool
	allRequires      bool
	ignoreGenerated  bool
	summaryOnly      bool
	failOnIssues     bool
	ci               bool
	showVersion      bool
}

type humanFindingWriter func(io.Writer, analyzer.Finding)

var humanFindingWriters = map[string]humanFindingWriter{
	analyzer.FindingUnusedDependency:   writeUnusedDependencyFinding,
	analyzer.FindingUnlistedDependency: writeUnlistedDependencyFinding,
	analyzer.FindingTestOnlyDependency: writeTestOnlyDependencyFinding,
	analyzer.FindingUnusedPackage:      writeUnusedPackageFinding,
	analyzer.FindingUnusedFile:         writeUnusedFileFinding,
	analyzer.FindingUnusedFunction:     writeUnusedFunctionFinding,
	analyzer.FindingUnusedMethod:       writeUnusedMethodFinding,
	analyzer.FindingUnusedStruct:       writeUnusedStructFinding,
	analyzer.FindingUnusedInterface:    writeUnusedInterfaceFinding,
	analyzer.FindingUnusedType:         writeUnusedTypeFinding,
	analyzer.FindingUnusedVar:          writeUnusedVarFinding,
	analyzer.FindingUnusedConst:        writeUnusedConstFinding,
	analyzer.FindingUnusedField:        writeUnusedFieldFinding,
	analyzer.FindingComplexity:         writeComplexityFinding,
	analyzer.FindingDuplicateCode:      writeDuplicateCodeFinding,
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	opts, cfg, code, ok := parseRunOptions(args, stdout, stderr)
	if !ok {
		return code
	}

	report, err := analyzer.Analyze(analyzerOptions(opts, cfg))
	if err != nil {
		fmt.Fprintf(stderr, "analyze: %v\n", err)
		return exitError
	}
	if code := postProcessReport(opts, report, stderr); code != exitOK {
		return code
	}
	if code := writeOutput(stdout, stderr, report, opts); code != exitOK {
		return code
	}

	if (opts.failOnIssues || opts.ci) && report.Summary.Findings > 0 {
		return exitIssues
	}

	return exitOK
}

func parseRunOptions(args []string, stdout io.Writer, stderr io.Writer) (cliOptions, settings.Config, int, bool) {
	command, args := splitCommand(args)
	opts := defaultCLIOptions(command)
	fs := newFlagSet(&opts, stderr)
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return opts, settings.Config{}, exitOK, false
		}
		return opts, settings.Config{}, exitError, false
	}
	if opts.showVersion {
		fmt.Fprintf(stdout, "%s %s\n", appName, version)
		return opts, settings.Config{}, exitOK, false
	}
	if fs.NArg() > 1 {
		fmt.Fprintf(stderr, "expected at most one root argument, got %d\n", fs.NArg())
		return opts, settings.Config{}, exitError, false
	}

	seenFlags := visitedFlags(fs)
	if fs.NArg() == 1 {
		opts.root = fs.Arg(0)
		seenFlags["root"] = true
	}
	cfg, loaded, err := loadConfig(opts.root, opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "config: %v\n", err)
		return opts, settings.Config{}, exitError, false
	}
	if loaded {
		applyConfig(&opts, cfg, seenFlags)
	}
	if !supportedFormat(opts.outputFormat) {
		fmt.Fprintf(stderr, "unsupported format %q\n", opts.outputFormat)
		return opts, settings.Config{}, exitError, false
	}

	return opts, cfg, exitOK, true
}

func defaultCLIOptions(command string) cliOptions {
	return cliOptions{
		command:      command,
		root:         ".",
		baseRef:      "HEAD~1",
		outputFormat: formatHuman,
	}
}

func newFlagSet(opts *cliOptions, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(appName, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.root, "root", opts.root, "project root to scan")
	fs.StringVar(&opts.root, "r", opts.root, "project root to scan")
	fs.StringVar(&opts.configPath, "config", "", "config file path")
	fs.StringVar(&opts.configPath, "c", "", "config file path")
	fs.StringVar(&opts.baselinePath, "baseline", "", "suppress findings recorded in a baseline file")
	fs.StringVar(&opts.saveBaselinePath, "save-baseline", "", "write current findings to a baseline file")
	fs.StringVar(&opts.coveragePath, "coverage", "", "Go coverage profile to annotate findings")
	fs.StringVar(&opts.baseRef, "base", opts.baseRef, "base ref for audit mode")
	fs.StringVar(&opts.baseRef, "changed-since", opts.baseRef, "base ref for audit mode")
	fs.StringVar(&opts.workspaceCSV, "workspace", "", "comma-separated module path or directory filters")
	fs.StringVar(&opts.tagsCSV, "tags", "", "comma-separated Go build tags")
	fs.StringVar(&opts.outputFormat, "format", opts.outputFormat, "output format: human, json, sarif, codeclimate, gitlab-codequality, or annotations")
	fs.StringVar(&opts.outputFormat, "f", opts.outputFormat, "output format: human, json, sarif, codeclimate, gitlab-codequality, or annotations")
	fs.IntVar(&opts.maxCyclomatic, "max-cyclomatic", 0, "maximum cyclomatic complexity before reporting")
	fs.IntVar(&opts.maxCognitive, "max-cognitive", 0, "maximum cognitive complexity before reporting")
	fs.BoolVar(&opts.production, "production", false, "exclude *_test.go files")
	fs.BoolVar(&opts.allRequires, "all-requires", false, "also check indirect requirements for unused status")
	fs.BoolVar(&opts.ignoreGenerated, "ignore-generated", false, "skip generated Go files")
	fs.BoolVar(&opts.summaryOnly, "summary", false, "only print summary counts in human output")
	fs.BoolVar(&opts.failOnIssues, "fail-on-issues", false, "exit 1 when findings exist")
	fs.BoolVar(&opts.ci, "ci", false, "enable CI behavior: fail on findings")
	fs.BoolVar(&opts.showVersion, "version", false, "print version and exit")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: %s [dead-code|audit] [flags] [root]\n\n", appName)
		fmt.Fprintln(stderr, "Analyze Go modules for dead code and dependency drift.")
		fmt.Fprintln(stderr)
		fs.PrintDefaults()
	}

	return fs
}

func analyzerOptions(opts cliOptions, cfg settings.Config) analyzer.Options {
	return analyzer.Options{
		Root:              opts.root,
		IncludeTests:      !opts.production,
		IncludeGenerated:  !opts.ignoreGenerated,
		CheckIndirect:     opts.allRequires,
		Rules:             cfg.Rules,
		IgnorePatterns:    cfg.IgnorePatterns,
		IgnoreFindings:    analyzerFindingMatchers(cfg.IgnoreFindings),
		WorkspacePatterns: splitCSV(opts.workspaceCSV),
		BuildTags:         splitCSV(opts.tagsCSV),
		MaxCyclomatic:     opts.maxCyclomatic,
		MaxCognitive:      opts.maxCognitive,
	}
}

func analyzerFindingMatchers(matchers []settings.FindingMatcher) []analyzer.FindingMatcher {
	if len(matchers) == 0 {
		return nil
	}
	out := make([]analyzer.FindingMatcher, 0, len(matchers))
	for _, matcher := range matchers {
		out = append(out, analyzer.FindingMatcher{
			Type:        matcher.Type,
			File:        matcher.File,
			Package:     matcher.Package,
			Module:      matcher.Module,
			ImportPath:  matcher.ImportPath,
			Symbol:      matcher.Symbol,
			Receiver:    matcher.Receiver,
			Struct:      matcher.Struct,
			Fingerprint: matcher.Fingerprint,
		})
	}

	return out
}

func postProcessReport(opts cliOptions, report *analyzer.Report, stderr io.Writer) int {
	if opts.saveBaselinePath != "" {
		if err := analyzer.SaveBaseline(opts.saveBaselinePath, report); err != nil {
			fmt.Fprintf(stderr, "save baseline: %v\n", err)
			return exitError
		}
	}
	if opts.baselinePath != "" {
		if err := analyzer.ApplyBaseline(opts.baselinePath, report); err != nil {
			fmt.Fprintf(stderr, "baseline: %v\n", err)
			return exitError
		}
	}
	if opts.coveragePath != "" {
		if err := analyzer.ApplyCoverage(opts.coveragePath, report); err != nil {
			fmt.Fprintf(stderr, "coverage: %v\n", err)
			return exitError
		}
	}
	if opts.command == "audit" {
		changed, err := changedFiles(opts.root, opts.baseRef)
		if err != nil {
			fmt.Fprintf(stderr, "audit: %v\n", err)
			return exitError
		}
		filterReportFiles(report, changed)
	}

	return exitOK
}

func writeOutput(stdout io.Writer, stderr io.Writer, report *analyzer.Report, opts cliOptions) int {
	switch opts.outputFormat {
	case formatJSON:
		if err := writeJSON(stdout, report); err != nil {
			fmt.Fprintf(stderr, "write json: %v\n", err)
			return exitError
		}
	case formatHuman:
		writeHuman(stdout, report, opts.summaryOnly)
	case formatSARIF:
		if err := writeSARIF(stdout, report); err != nil {
			fmt.Fprintf(stderr, "write sarif: %v\n", err)
			return exitError
		}
	case formatCodeClimate, formatGitLabCodeQuality:
		if err := writeCodeClimate(stdout, report); err != nil {
			fmt.Fprintf(stderr, "write code quality: %v\n", err)
			return exitError
		}
	case formatAnnotations:
		writeAnnotations(stdout, report)
	}

	return exitOK
}

func visitedFlags(fs *flag.FlagSet) map[string]bool {
	seen := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		seen[f.Name] = true
	})

	return seen
}

func loadConfig(root string, configPath string) (settings.Config, bool, error) {
	if configPath != "" {
		return settings.Load(configPath)
	}

	return settings.Load(settings.DefaultPath(root))
}

func applyConfig(opts *cliOptions, cfg settings.Config, seen map[string]bool) {
	applyStringConfig(&opts.root, cfg.Root, !seen["root"] && !seen["r"])
	applyStringConfig(&opts.outputFormat, cfg.Format, !seen["format"] && !seen["f"])
	applyIntConfig(&opts.maxCyclomatic, cfg.Health.MaxCyclomatic, !seen["max-cyclomatic"])
	applyIntConfig(&opts.maxCognitive, cfg.Health.MaxCognitive, !seen["max-cognitive"])
	applyBoolConfig(&opts.production, cfg.Production, !seen["production"])
	applyBoolConfig(&opts.allRequires, cfg.AllRequires, !seen["all-requires"])
	applyBoolConfig(&opts.ignoreGenerated, cfg.IgnoreGenerated, !seen["ignore-generated"])
	applyBoolConfig(&opts.summaryOnly, cfg.Summary, !seen["summary"])
	applyBoolConfig(&opts.failOnIssues, cfg.FailOnIssues, !seen["fail-on-issues"])
	applyStringSliceConfig(&opts.workspaceCSV, cfg.Workspace)
	applyStringSliceConfig(&opts.tagsCSV, cfg.BuildTags)
}

func applyStringConfig(target *string, value string, allowed bool) {
	if value != "" && allowed {
		*target = value
	}
}

func applyIntConfig(target *int, value int, allowed bool) {
	if value != 0 && allowed {
		*target = value
	}
}

func applyBoolConfig(target *bool, value *bool, allowed bool) {
	if value != nil && allowed {
		*target = *value
	}
}

func applyStringSliceConfig(target *string, values []string) {
	if *target == "" && len(values) > 0 {
		*target = strings.Join(values, ",")
	}
}

func supportedFormat(format string) bool {
	switch format {
	case formatHuman, formatJSON, formatSARIF, formatCodeClimate, formatGitLabCodeQuality, formatAnnotations:
		return true
	default:
		return false
	}
}

func splitCommand(args []string) (string, []string) {
	if len(args) == 0 {
		return "check", args
	}

	switch args[0] {
	case "dead-code", "check", "audit":
		return args[0], args[1:]
	case "help":
		return "help", []string{"-h"}
	default:
		return "check", args
	}
}

func splitCSV(value string) []string {
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}

	return items
}

func changedFiles(root string, baseRef string) (map[string]struct{}, error) {
	files := make(map[string]struct{})
	if err := addChangedFiles(files, root, baseRef); err != nil {
		return nil, err
	}
	if err := addChangedFiles(files, root, ""); err != nil {
		return nil, err
	}

	return files, nil
}

func addChangedFiles(files map[string]struct{}, root string, baseRef string) error {
	args := []string{"-C", root, "diff", "--name-only", "--relative"}
	if baseRef != "" {
		args = append(args, baseRef)
	}
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("git diff %s: %w", baseRef, err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files[line] = struct{}{}
		}
	}

	return nil
}

func filterReportFiles(report *analyzer.Report, files map[string]struct{}) {
	for moduleIndex := range report.Modules {
		findings := report.Modules[moduleIndex].Findings
		active := findings[:0]
		for _, finding := range findings {
			if findingTouchesFile(finding, files) {
				active = append(active, finding)
			}
		}
		report.Modules[moduleIndex].Findings = active
	}
	analyzer.RefreshReport(report)
}

func findingTouchesFile(finding analyzer.Finding, files map[string]struct{}) bool {
	if _, ok := files[finding.File]; ok {
		return true
	}
	for _, location := range finding.Locations {
		if _, ok := files[location.File]; ok {
			return true
		}
	}

	return false
}

func writeJSON(w io.Writer, report *analyzer.Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func writeSARIF(w io.Writer, report *analyzer.Report) error {
	type sarifLocation struct {
		PhysicalLocation struct {
			ArtifactLocation struct {
				URI string `json:"uri"`
			} `json:"artifactLocation"`
			Region struct {
				StartLine int `json:"startLine"`
			} `json:"region"`
		} `json:"physicalLocation"`
	}
	type sarifResult struct {
		RuleID    string            `json:"ruleId"`
		Level     string            `json:"level"`
		Message   map[string]string `json:"message"`
		Locations []sarifLocation   `json:"locations"`
	}
	type sarifRun struct {
		Tool struct {
			Driver struct {
				Name string `json:"name"`
			} `json:"driver"`
		} `json:"tool"`
		Results []sarifResult `json:"results"`
	}
	output := struct {
		Version string     `json:"version"`
		Schema  string     `json:"$schema"`
		Runs    []sarifRun `json:"runs"`
	}{
		Version: "2.1.0",
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Runs:    []sarifRun{{}},
	}
	output.Runs[0].Tool.Driver.Name = appName
	output.Runs[0].Results = []sarifResult{}
	for _, finding := range allFindings(report) {
		var loc sarifLocation
		loc.PhysicalLocation.ArtifactLocation.URI = finding.File
		loc.PhysicalLocation.Region.StartLine = finding.Line
		output.Runs[0].Results = append(output.Runs[0].Results, sarifResult{
			RuleID:    finding.Type,
			Level:     "warning",
			Message:   map[string]string{"text": findingMessage(finding)},
			Locations: []sarifLocation{loc},
		})
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func writeCodeClimate(w io.Writer, report *analyzer.Report) error {
	type location struct {
		Path  string `json:"path"`
		Lines struct {
			Begin int `json:"begin"`
		} `json:"lines"`
	}
	type issue struct {
		Type        string   `json:"type"`
		CheckName   string   `json:"check_name"`
		Description string   `json:"description"`
		Categories  []string `json:"categories"`
		Fingerprint string   `json:"fingerprint"`
		Severity    string   `json:"severity"`
		Location    location `json:"location"`
	}
	issues := make([]issue, 0)
	for _, finding := range allFindings(report) {
		item := issue{
			Type:        "issue",
			CheckName:   finding.Type,
			Description: findingMessage(finding),
			Categories:  []string{"Bug Risk"},
			Fingerprint: finding.Fingerprint,
			Severity:    "minor",
		}
		item.Location.Path = finding.File
		item.Location.Lines.Begin = finding.Line
		issues = append(issues, item)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(issues)
}

func writeAnnotations(w io.Writer, report *analyzer.Report) {
	for _, finding := range allFindings(report) {
		fmt.Fprintf(w, "::warning file=%s,line=%d::%s\n", escapeAnnotationProperty(finding.File), finding.Line, escapeAnnotation(findingMessage(finding)))
	}
}

func allFindings(report *analyzer.Report) []analyzer.Finding {
	var findings []analyzer.Finding
	for _, module := range report.Modules {
		findings = append(findings, module.Findings...)
	}

	return findings
}

func findingMessage(finding analyzer.Finding) string {
	if finding.Symbol != "" {
		return fmt.Sprintf("%s: %s", finding.Type, finding.Symbol)
	}
	if finding.Module != "" {
		return fmt.Sprintf("%s: %s", finding.Type, finding.Module)
	}
	if finding.ImportPath != "" {
		return fmt.Sprintf("%s: %s", finding.Type, finding.ImportPath)
	}

	return finding.Type
}

func escapeAnnotation(value string) string {
	value = strings.ReplaceAll(value, "%", "%25")
	value = strings.ReplaceAll(value, "\r", "%0D")
	value = strings.ReplaceAll(value, "\n", "%0A")
	return value
}

func escapeAnnotationProperty(value string) string {
	value = escapeAnnotation(value)
	value = strings.ReplaceAll(value, ":", "%3A")
	value = strings.ReplaceAll(value, ",", "%2C")
	return value
}

func writeHuman(w io.Writer, report *analyzer.Report, summaryOnly bool) {
	if !summaryOnly {
		writeHumanFindings(w, report)
	}

	fmt.Fprintln(w, "Summary")
	fmt.Fprintf(w, "  modules: %d\n", report.Summary.Modules)
	fmt.Fprintf(w, "  findings: %d\n", report.Summary.Findings)
	if report.Summary.SuppressedFindings > 0 {
		fmt.Fprintf(w, "  suppressed findings: %d\n", report.Summary.SuppressedFindings)
	}
	if report.Summary.SuppressedByBaseline > 0 {
		fmt.Fprintf(w, "  suppressed by baseline: %d\n", report.Summary.SuppressedByBaseline)
	}
	if report.Summary.SkippedModules > 0 {
		fmt.Fprintf(w, "  skipped modules: %d\n", report.Summary.SkippedModules)
	}
	if report.Summary.CoveredFindings > 0 || report.Summary.UncoveredFindings > 0 {
		fmt.Fprintf(w, "  covered findings: %d\n", report.Summary.CoveredFindings)
		fmt.Fprintf(w, "  uncovered findings: %d\n", report.Summary.UncoveredFindings)
	}
	fmt.Fprintf(w, "  unused dependencies: %d\n", report.Summary.UnusedDependencies)
	fmt.Fprintf(w, "  unlisted dependencies: %d\n", report.Summary.UnlistedDependencies)
	fmt.Fprintf(w, "  test-only dependencies: %d\n", report.Summary.TestOnlyDependencies)
	fmt.Fprintf(w, "  unused packages: %d\n", report.Summary.UnusedPackages)
	fmt.Fprintf(w, "  unused files: %d\n", report.Summary.UnusedFiles)
	fmt.Fprintf(w, "  unused functions: %d\n", report.Summary.UnusedFunctions)
	fmt.Fprintf(w, "  unused methods: %d\n", report.Summary.UnusedMethods)
	fmt.Fprintf(w, "  unused structs: %d\n", report.Summary.UnusedStructs)
	fmt.Fprintf(w, "  unused interfaces: %d\n", report.Summary.UnusedInterfaces)
	fmt.Fprintf(w, "  unused types: %d\n", report.Summary.UnusedTypes)
	fmt.Fprintf(w, "  unused vars: %d\n", report.Summary.UnusedVars)
	fmt.Fprintf(w, "  unused consts: %d\n", report.Summary.UnusedConsts)
	fmt.Fprintf(w, "  unused fields: %d\n", report.Summary.UnusedFields)
	fmt.Fprintf(w, "  complexity findings: %d\n", report.Summary.ComplexityFindings)
	fmt.Fprintf(w, "  duplicate groups: %d\n", report.Summary.DuplicateGroups)
	fmt.Fprintf(w, "  duplicated lines: %d\n", report.Summary.DuplicatedLines)
}

func writeHumanFindings(w io.Writer, report *analyzer.Report) {
	if report.Summary.Findings == 0 {
		fmt.Fprintln(w, "No Go dead-code or dependency findings.")
		fmt.Fprintln(w)
		return
	}

	for _, module := range report.Modules {
		if len(module.Findings) == 0 {
			continue
		}

		fmt.Fprintf(w, "%s (%s)\n", module.ModulePath, module.Dir)
		for _, finding := range module.Findings {
			writeHumanFinding(w, finding)
		}
		fmt.Fprintln(w)
	}
}

func writeHumanFinding(w io.Writer, finding analyzer.Finding) {
	writer, ok := humanFindingWriters[finding.Type]
	if !ok {
		writeDefaultFinding(w, finding)
		return
	}
	writer(w, finding)
}

func writeUnusedDependencyFinding(w io.Writer, finding analyzer.Finding) {
	fmt.Fprintf(w, "  unused dependency %s %s at %s:%d", finding.Module, finding.Version, finding.File, finding.Line)
	if len(finding.UsedInModules) > 0 {
		fmt.Fprintf(w, " (used in %s)", strings.Join(finding.UsedInModules, ", "))
	}
	fmt.Fprintln(w)
}

func writeUnlistedDependencyFinding(w io.Writer, finding analyzer.Finding) {
	fmt.Fprintf(w, "  unlisted dependency %s imported at %s:%d\n", finding.ImportPath, finding.File, finding.Line)
}

func writeTestOnlyDependencyFinding(w io.Writer, finding analyzer.Finding) {
	fmt.Fprintf(w, "  test-only dependency %s %s at %s:%d\n", finding.Module, finding.Version, finding.File, finding.Line)
}

func writeUnusedPackageFinding(w io.Writer, finding analyzer.Finding) {
	fmt.Fprintf(w, "  unused package %s at %s:%d\n", finding.ImportPath, finding.File, finding.Line)
}

func writeUnusedFileFinding(w io.Writer, finding analyzer.Finding) {
	fmt.Fprintf(w, "  unused file %s:%d\n", finding.File, finding.Line)
}

func writeUnusedFunctionFinding(w io.Writer, finding analyzer.Finding) {
	fmt.Fprintf(w, "  unused function %s.%s at %s:%d\n", finding.Package, finding.Symbol, finding.File, finding.Line)
}

func writeUnusedMethodFinding(w io.Writer, finding analyzer.Finding) {
	fmt.Fprintf(w, "  unused method %s.%s in %s at %s:%d\n", finding.Receiver, finding.Symbol, finding.Package, finding.File, finding.Line)
}

func writeUnusedStructFinding(w io.Writer, finding analyzer.Finding) {
	fmt.Fprintf(w, "  unused struct %s.%s at %s:%d\n", finding.Package, finding.Symbol, finding.File, finding.Line)
}

func writeUnusedInterfaceFinding(w io.Writer, finding analyzer.Finding) {
	fmt.Fprintf(w, "  unused interface %s.%s at %s:%d\n", finding.Package, finding.Symbol, finding.File, finding.Line)
}

func writeUnusedTypeFinding(w io.Writer, finding analyzer.Finding) {
	fmt.Fprintf(w, "  unused type %s.%s at %s:%d\n", finding.Package, finding.Symbol, finding.File, finding.Line)
}

func writeUnusedVarFinding(w io.Writer, finding analyzer.Finding) {
	fmt.Fprintf(w, "  unused var %s.%s at %s:%d\n", finding.Package, finding.Symbol, finding.File, finding.Line)
}

func writeUnusedConstFinding(w io.Writer, finding analyzer.Finding) {
	fmt.Fprintf(w, "  unused const %s.%s at %s:%d\n", finding.Package, finding.Symbol, finding.File, finding.Line)
}

func writeUnusedFieldFinding(w io.Writer, finding analyzer.Finding) {
	fmt.Fprintf(w, "  unused field %s.%s in %s at %s:%d\n", finding.Struct, finding.Symbol, finding.Package, finding.File, finding.Line)
}

func writeComplexityFinding(w io.Writer, finding analyzer.Finding) {
	fmt.Fprintf(w, "  complex function %s.%s at %s:%d (cyclomatic %d, cognitive %d)\n", finding.Package, finding.Symbol, finding.File, finding.Line, finding.Metrics.Cyclomatic, finding.Metrics.Cognitive)
}

func writeDuplicateCodeFinding(w io.Writer, finding analyzer.Finding) {
	fmt.Fprintf(w, "  duplicate code %s at %s:%d (%d duplicated lines)\n", finding.Symbol, finding.File, finding.Line, finding.Lines)
}

func writeDefaultFinding(w io.Writer, finding analyzer.Finding) {
	fmt.Fprintf(w, "  %s at %s:%d\n", finding.Type, finding.File, finding.Line)
}
