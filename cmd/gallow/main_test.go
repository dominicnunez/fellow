package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dominicnunez/gallow/internal/analyzer"
)

type sarifTestOutput struct {
	Runs []sarifTestRun `json:"runs"`
}

type sarifTestRun struct {
	AutomationDetails struct {
		ID string `json:"id"`
	} `json:"automationDetails"`
	Tool struct {
		Driver sarifTestDriver `json:"driver"`
	} `json:"tool"`
	Results []sarifTestResult `json:"results"`
}

type sarifTestDriver struct {
	Name            string          `json:"name"`
	InformationURI  string          `json:"informationUri"`
	SemanticVersion string          `json:"semanticVersion"`
	Rules           []sarifTestRule `json:"rules"`
}

type sarifTestRule struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	ShortDescription map[string]string `json:"shortDescription"`
	HelpURI          string            `json:"helpUri"`
}

type sarifTestResult struct {
	RuleID              string              `json:"ruleId"`
	RuleIndex           *int                `json:"ruleIndex"`
	Level               string              `json:"level"`
	Message             map[string]string   `json:"message"`
	Locations           []sarifTestLocation `json:"locations"`
	RelatedLocations    []sarifTestRelated  `json:"relatedLocations"`
	PartialFingerprints map[string]string   `json:"partialFingerprints"`
	Fingerprints        map[string]string   `json:"fingerprints"`
}

type sarifTestLocation struct {
	PhysicalLocation struct {
		ArtifactLocation struct {
			URI string `json:"uri"`
		} `json:"artifactLocation"`
		Region struct {
			StartLine int `json:"startLine"`
			EndLine   int `json:"endLine"`
		} `json:"region"`
	} `json:"physicalLocation"`
}

type sarifTestRelated struct {
	ID int `json:"id"`
}

func TestFilterReportFilesKeepsDuplicateLocations(t *testing.T) {
	report := &analyzer.Report{
		Modules: []analyzer.ModuleReport{{
			ModulePath: "example.com/app",
			Dir:        ".",
			Findings: []analyzer.Finding{
				{
					Type:  analyzer.FindingDuplicateCode,
					File:  "a.go",
					Line:  10,
					Lines: 6,
					Locations: []analyzer.Location{
						{File: "a.go", StartLine: 10, EndLine: 15},
						{File: "b.go", StartLine: 20, EndLine: 25},
					},
				},
				{Type: analyzer.FindingUnusedFunction, File: "c.go", Line: 3, Symbol: "Dead"},
			},
		}},
	}
	report.Summary.SkippedModules = 1
	report.Summary.SuppressedFindings = 2

	filterReportFiles(report, map[string]struct{}{"b.go": {}})

	if got := report.Summary.Findings; got != 1 {
		t.Fatalf("findings = %d; want 1", got)
	}
	if got := report.Summary.DuplicateGroups; got != 1 {
		t.Fatalf("duplicate groups = %d; want 1", got)
	}
	if got := report.Summary.SkippedModules; got != 1 {
		t.Fatalf("skipped modules = %d; want 1", got)
	}
	if got := report.Summary.SuppressedFindings; got != 2 {
		t.Fatalf("suppressed findings = %d; want 2", got)
	}
}

func TestWriteCIFormats(t *testing.T) {
	report := &analyzer.Report{
		Modules: []analyzer.ModuleReport{{
			ModulePath: "example.com/app",
			Dir:        ".",
			Findings: []analyzer.Finding{{
				Type:        analyzer.FindingUnusedFunction,
				File:        "main.go",
				Line:        4,
				Symbol:      "Dead",
				Fingerprint: "abc123",
			}},
		}},
	}

	var sarif bytes.Buffer
	if err := writeSARIF(&sarif, report); err != nil {
		t.Fatalf("writeSARIF() error = %v", err)
	}
	var sarifOutput struct {
		Runs []struct {
			Results []struct {
				RuleID string `json:"ruleId"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(sarif.Bytes(), &sarifOutput); err != nil {
		t.Fatalf("sarif unmarshal error = %v", err)
	}
	if got := sarifOutput.Runs[0].Results[0].RuleID; got != analyzer.FindingUnusedFunction {
		t.Fatalf("sarif ruleId = %q; want %q", got, analyzer.FindingUnusedFunction)
	}

	var codeQuality bytes.Buffer
	if err := writeCodeClimate(&codeQuality, report); err != nil {
		t.Fatalf("writeCodeClimate() error = %v", err)
	}
	var issues []struct {
		CheckName string `json:"check_name"`
		Location  struct {
			Path string `json:"path"`
		} `json:"location"`
	}
	if err := json.Unmarshal(codeQuality.Bytes(), &issues); err != nil {
		t.Fatalf("code quality unmarshal error = %v", err)
	}
	if len(issues) != 1 || issues[0].CheckName != analyzer.FindingUnusedFunction || issues[0].Location.Path != "main.go" {
		t.Fatalf("code quality issues = %#v; want unused-function for main.go", issues)
	}

	var annotations bytes.Buffer
	if err := writeAnnotations(&annotations, report); err != nil {
		t.Fatalf("writeAnnotations() error = %v", err)
	}
	if got := annotations.String(); !strings.Contains(got, "::warning file=main.go,line=4::unused-function: Dead") {
		t.Fatalf("annotations = %q; want warning for unused function", got)
	}
}

func TestWriteSARIFIncludesCodeScanningMetadata(t *testing.T) {
	report := &analyzer.Report{
		Modules: []analyzer.ModuleReport{{
			ModulePath: "example.com/app",
			Dir:        ".",
			Findings: []analyzer.Finding{
				{
					Type:        analyzer.FindingUnusedFunction,
					File:        "main.go",
					Line:        4,
					Symbol:      "Dead",
					Fingerprint: "unused123",
				},
				{
					Type:        analyzer.FindingDuplicateCode,
					File:        "a.go",
					Line:        10,
					Lines:       12,
					Symbol:      "clone-abc123",
					Fingerprint: "duplicate123",
					Locations: []analyzer.Location{
						{File: "a.go", StartLine: 10, EndLine: 21},
						{File: "b.go", StartLine: 30, EndLine: 41},
					},
				},
			},
		}},
	}

	var sarif bytes.Buffer
	if err := writeSARIF(&sarif, report); err != nil {
		t.Fatalf("writeSARIF() error = %v", err)
	}

	output := decodeSARIFOutput(t, sarif.Bytes())
	if len(output.Runs) != 1 {
		t.Fatalf("runs = %d; want 1", len(output.Runs))
	}
	run := output.Runs[0]
	ruleIndexes := assertSARIFRunMetadata(t, run)
	assertSARIFResults(t, run.Results, ruleIndexes)
}

func decodeSARIFOutput(t *testing.T, data []byte) sarifTestOutput {
	t.Helper()

	var output sarifTestOutput
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatalf("sarif unmarshal error = %v", err)
	}
	return output
}

func assertSARIFRunMetadata(t *testing.T, run sarifTestRun) map[string]int {
	t.Helper()

	if run.AutomationDetails.ID != appName {
		t.Fatalf("automation id = %q; want %q", run.AutomationDetails.ID, appName)
	}
	if run.Tool.Driver.Name != appName || run.Tool.Driver.SemanticVersion != version || run.Tool.Driver.InformationURI != gallowRepositoryURL {
		t.Fatalf("driver = %#v; want gallow driver metadata", run.Tool.Driver)
	}
	if len(run.Tool.Driver.Rules) == 0 {
		t.Fatal("rules = 0; want rule metadata")
	}
	ruleIndexes := make(map[string]int)
	for i, rule := range run.Tool.Driver.Rules {
		ruleIndexes[rule.ID] = i
		if rule.Name == "" || rule.ShortDescription["text"] == "" || rule.HelpURI == "" {
			t.Fatalf("rule %#v is missing display metadata", rule)
		}
	}

	return ruleIndexes
}

func assertSARIFResults(t *testing.T, results []sarifTestResult, ruleIndexes map[string]int) {
	t.Helper()

	unusedRuleIndex, ok := ruleIndexes[analyzer.FindingUnusedFunction]
	if !ok {
		t.Fatalf("missing rule %q", analyzer.FindingUnusedFunction)
	}
	duplicateRuleIndex, ok := ruleIndexes[analyzer.FindingDuplicateCode]
	if !ok {
		t.Fatalf("missing rule %q", analyzer.FindingDuplicateCode)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d; want 2", len(results))
	}
	assertSARIFUnusedResult(t, results[0], unusedRuleIndex)
	assertSARIFDuplicateResult(t, results[1], duplicateRuleIndex)
}

func assertSARIFUnusedResult(t *testing.T, unused sarifTestResult, unusedRuleIndex int) {
	t.Helper()

	if unused.RuleID != analyzer.FindingUnusedFunction || unused.RuleIndex == nil || *unused.RuleIndex != unusedRuleIndex || unused.Level != sarifLevelWarning {
		t.Fatalf("unused result = %#v; want warning unused-function with rule index", unused)
	}
	if unused.PartialFingerprints[sarifFingerprintKey] != "unused123" || unused.Fingerprints[sarifFingerprintKey] != "unused123" {
		t.Fatalf("unused fingerprints = %#v/%#v; want stable Gallow fingerprints", unused.PartialFingerprints, unused.Fingerprints)
	}
}

func assertSARIFDuplicateResult(t *testing.T, duplicate sarifTestResult, duplicateRuleIndex int) {
	t.Helper()

	if duplicate.RuleID != analyzer.FindingDuplicateCode || duplicate.RuleIndex == nil || *duplicate.RuleIndex != duplicateRuleIndex || duplicate.Level != sarifLevelNote {
		t.Fatalf("duplicate result = %#v; want note duplicate-code with rule index", duplicate)
	}
	if duplicate.Message["text"] == "" || !strings.Contains(duplicate.Message["text"], "12 duplicated lines") {
		t.Fatalf("duplicate message = %#v; want duplicated-line context", duplicate.Message)
	}
	if len(duplicate.Locations) != 1 || duplicate.Locations[0].PhysicalLocation.ArtifactLocation.URI != "a.go" || duplicate.Locations[0].PhysicalLocation.Region.StartLine != 10 || duplicate.Locations[0].PhysicalLocation.Region.EndLine != 21 {
		t.Fatalf("duplicate locations = %#v; want primary location range", duplicate.Locations)
	}
	if len(duplicate.RelatedLocations) != 2 || duplicate.RelatedLocations[0].ID != relatedLocationStart {
		t.Fatalf("duplicate related locations = %#v; want duplicate spans", duplicate.RelatedLocations)
	}
}
