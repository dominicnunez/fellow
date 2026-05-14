package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"fellow/internal/analyzer"
)

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
	writeAnnotations(&annotations, report)
	if got := annotations.String(); !strings.Contains(got, "::warning file=main.go,line=4::unused-function: Dead") {
		t.Fatalf("annotations = %q; want warning for unused function", got)
	}
}
