package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
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

	formatHuman = "human"
	formatJSON  = "json"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	args = normalizeCommand(args)

	var root string
	var configPath string
	var baselinePath string
	var saveBaselinePath string
	var outputFormat string
	var maxCyclomatic int
	var maxCognitive int
	var production bool
	var allRequires bool
	var ignoreGenerated bool
	var summaryOnly bool
	var failOnIssues bool
	var ci bool
	var showVersion bool

	fs := flag.NewFlagSet(appName, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&root, "root", ".", "project root to scan")
	fs.StringVar(&root, "r", ".", "project root to scan")
	fs.StringVar(&configPath, "config", "", "config file path")
	fs.StringVar(&configPath, "c", "", "config file path")
	fs.StringVar(&baselinePath, "baseline", "", "suppress findings recorded in a baseline file")
	fs.StringVar(&saveBaselinePath, "save-baseline", "", "write current findings to a baseline file")
	fs.StringVar(&outputFormat, "format", formatHuman, "output format: human or json")
	fs.StringVar(&outputFormat, "f", formatHuman, "output format: human or json")
	fs.IntVar(&maxCyclomatic, "max-cyclomatic", 0, "maximum cyclomatic complexity before reporting")
	fs.IntVar(&maxCognitive, "max-cognitive", 0, "maximum cognitive complexity before reporting")
	fs.BoolVar(&production, "production", false, "exclude *_test.go files")
	fs.BoolVar(&allRequires, "all-requires", false, "also check indirect requirements for unused status")
	fs.BoolVar(&ignoreGenerated, "ignore-generated", false, "skip generated Go files")
	fs.BoolVar(&summaryOnly, "summary", false, "only print summary counts in human output")
	fs.BoolVar(&failOnIssues, "fail-on-issues", false, "exit 1 when findings exist")
	fs.BoolVar(&ci, "ci", false, "enable CI behavior: fail on findings")
	fs.BoolVar(&showVersion, "version", false, "print version and exit")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: %s [dead-code] [flags] [root]\n\n", appName)
		fmt.Fprintln(stderr, "Analyze Go modules for dead code and dependency drift.")
		fmt.Fprintln(stderr)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return exitOK
		}
		return exitError
	}

	if showVersion {
		fmt.Fprintf(stdout, "%s %s\n", appName, version)
		return exitOK
	}

	if fs.NArg() > 1 {
		fmt.Fprintf(stderr, "expected at most one root argument, got %d\n", fs.NArg())
		return exitError
	}
	seenFlags := visitedFlags(fs)
	if fs.NArg() == 1 {
		root = fs.Arg(0)
		seenFlags["root"] = true
	}

	cfg, loaded, err := loadConfig(root, configPath)
	if err != nil {
		fmt.Fprintf(stderr, "config: %v\n", err)
		return exitError
	}
	if loaded {
		applyConfig(&root, &outputFormat, &maxCyclomatic, &maxCognitive, &production, &allRequires, &ignoreGenerated, &summaryOnly, &failOnIssues, cfg, seenFlags)
	}

	if outputFormat != formatHuman && outputFormat != formatJSON {
		fmt.Fprintf(stderr, "unsupported format %q\n", outputFormat)
		return exitError
	}

	report, err := analyzer.Analyze(analyzer.Options{
		Root:             root,
		IncludeTests:     !production,
		IncludeGenerated: !ignoreGenerated,
		CheckIndirect:    allRequires,
		Rules:            cfg.Rules,
		IgnorePatterns:   cfg.IgnorePatterns,
		MaxCyclomatic:    maxCyclomatic,
		MaxCognitive:     maxCognitive,
	})
	if err != nil {
		fmt.Fprintf(stderr, "analyze: %v\n", err)
		return exitError
	}
	if saveBaselinePath != "" {
		if err := analyzer.SaveBaseline(saveBaselinePath, report); err != nil {
			fmt.Fprintf(stderr, "save baseline: %v\n", err)
			return exitError
		}
	}
	if baselinePath != "" {
		if err := analyzer.ApplyBaseline(baselinePath, report); err != nil {
			fmt.Fprintf(stderr, "baseline: %v\n", err)
			return exitError
		}
	}

	switch outputFormat {
	case formatJSON:
		if err := writeJSON(stdout, report); err != nil {
			fmt.Fprintf(stderr, "write json: %v\n", err)
			return exitError
		}
	case formatHuman:
		writeHuman(stdout, report, summaryOnly)
	}

	if (failOnIssues || ci) && report.Summary.Findings > 0 {
		return exitIssues
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

func applyConfig(root *string, outputFormat *string, maxCyclomatic *int, maxCognitive *int, production *bool, allRequires *bool, ignoreGenerated *bool, summaryOnly *bool, failOnIssues *bool, cfg settings.Config, seen map[string]bool) {
	if cfg.Root != "" && !seen["root"] && !seen["r"] {
		*root = cfg.Root
	}
	if cfg.Format != "" && !seen["format"] && !seen["f"] {
		*outputFormat = cfg.Format
	}
	if cfg.Health.MaxCyclomatic != 0 && !seen["max-cyclomatic"] {
		*maxCyclomatic = cfg.Health.MaxCyclomatic
	}
	if cfg.Health.MaxCognitive != 0 && !seen["max-cognitive"] {
		*maxCognitive = cfg.Health.MaxCognitive
	}
	if cfg.Production != nil && !seen["production"] {
		*production = *cfg.Production
	}
	if cfg.AllRequires != nil && !seen["all-requires"] {
		*allRequires = *cfg.AllRequires
	}
	if cfg.IgnoreGenerated != nil && !seen["ignore-generated"] {
		*ignoreGenerated = *cfg.IgnoreGenerated
	}
	if cfg.Summary != nil && !seen["summary"] {
		*summaryOnly = *cfg.Summary
	}
	if cfg.FailOnIssues != nil && !seen["fail-on-issues"] {
		*failOnIssues = *cfg.FailOnIssues
	}
}

func normalizeCommand(args []string) []string {
	if len(args) == 0 {
		return args
	}

	switch args[0] {
	case "dead-code", "check":
		return args[1:]
	case "help":
		return []string{"-h"}
	default:
		return args
	}
}

func writeJSON(w io.Writer, report *analyzer.Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
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
	switch finding.Type {
	case analyzer.FindingUnusedDependency:
		fmt.Fprintf(w, "  unused dependency %s %s at %s:%d", finding.Module, finding.Version, finding.File, finding.Line)
		if len(finding.UsedInModules) > 0 {
			fmt.Fprintf(w, " (used in %s)", strings.Join(finding.UsedInModules, ", "))
		}
		fmt.Fprintln(w)
	case analyzer.FindingUnlistedDependency:
		fmt.Fprintf(w, "  unlisted dependency %s imported at %s:%d\n", finding.ImportPath, finding.File, finding.Line)
	case analyzer.FindingTestOnlyDependency:
		fmt.Fprintf(w, "  test-only dependency %s %s at %s:%d\n", finding.Module, finding.Version, finding.File, finding.Line)
	case analyzer.FindingUnusedPackage:
		fmt.Fprintf(w, "  unused package %s at %s:%d\n", finding.ImportPath, finding.File, finding.Line)
	case analyzer.FindingUnusedFile:
		fmt.Fprintf(w, "  unused file %s:%d\n", finding.File, finding.Line)
	case analyzer.FindingUnusedFunction:
		fmt.Fprintf(w, "  unused function %s.%s at %s:%d\n", finding.Package, finding.Symbol, finding.File, finding.Line)
	case analyzer.FindingUnusedMethod:
		fmt.Fprintf(w, "  unused method %s.%s in %s at %s:%d\n", finding.Receiver, finding.Symbol, finding.Package, finding.File, finding.Line)
	case analyzer.FindingUnusedStruct:
		fmt.Fprintf(w, "  unused struct %s.%s at %s:%d\n", finding.Package, finding.Symbol, finding.File, finding.Line)
	case analyzer.FindingUnusedInterface:
		fmt.Fprintf(w, "  unused interface %s.%s at %s:%d\n", finding.Package, finding.Symbol, finding.File, finding.Line)
	case analyzer.FindingUnusedType:
		fmt.Fprintf(w, "  unused type %s.%s at %s:%d\n", finding.Package, finding.Symbol, finding.File, finding.Line)
	case analyzer.FindingUnusedVar:
		fmt.Fprintf(w, "  unused var %s.%s at %s:%d\n", finding.Package, finding.Symbol, finding.File, finding.Line)
	case analyzer.FindingUnusedConst:
		fmt.Fprintf(w, "  unused const %s.%s at %s:%d\n", finding.Package, finding.Symbol, finding.File, finding.Line)
	case analyzer.FindingUnusedField:
		fmt.Fprintf(w, "  unused field %s.%s in %s at %s:%d\n", finding.Struct, finding.Symbol, finding.Package, finding.File, finding.Line)
	case analyzer.FindingComplexity:
		fmt.Fprintf(w, "  complex function %s.%s at %s:%d (cyclomatic %d, cognitive %d)\n", finding.Package, finding.Symbol, finding.File, finding.Line, finding.Metrics.Cyclomatic, finding.Metrics.Cognitive)
	case analyzer.FindingDuplicateCode:
		fmt.Fprintf(w, "  duplicate code %s at %s:%d (%d duplicated lines)\n", finding.Symbol, finding.File, finding.Line, finding.Lines)
	default:
		fmt.Fprintf(w, "  %s at %s:%d\n", finding.Type, finding.File, finding.Line)
	}
}
