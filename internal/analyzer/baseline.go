package analyzer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	baselineDirPerm  = 0o755
	baselineFilePerm = 0o644
)

type Baseline struct {
	SchemaVersion string          `json:"schema_version"`
	Findings      []BaselineEntry `json:"findings"`
}

type BaselineEntry struct {
	Fingerprint string `json:"fingerprint"`
	Type        string `json:"type"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Symbol      string `json:"symbol,omitempty"`
	Module      string `json:"module,omitempty"`
	ImportPath  string `json:"import_path,omitempty"`
}

func SaveBaseline(path string, report *Report) error {
	baseline := Baseline{
		SchemaVersion: SchemaVersion,
	}
	for _, module := range report.Modules {
		for _, finding := range module.Findings {
			baseline.Findings = append(baseline.Findings, BaselineEntry{
				Fingerprint: finding.Fingerprint,
				Type:        finding.Type,
				File:        finding.File,
				Line:        finding.Line,
				Symbol:      finding.Symbol,
				Module:      finding.Module,
				ImportPath:  finding.ImportPath,
			})
		}
	}

	data, err := json.MarshalIndent(baseline, "", "  ")
	if err != nil {
		return fmt.Errorf("encode baseline: %w", err)
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(path), baselineDirPerm); err != nil {
		return fmt.Errorf("create baseline directory: %w", err)
	}
	if err := os.WriteFile(path, data, baselineFilePerm); err != nil {
		return fmt.Errorf("write baseline %s: %w", path, err)
	}

	return nil
}

func ApplyBaseline(path string, report *Report) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read baseline %s: %w", path, err)
	}

	var baseline Baseline
	if err := json.Unmarshal(data, &baseline); err != nil {
		return fmt.Errorf("parse baseline %s: %w", path, err)
	}

	fingerprints := make(map[string]struct{}, len(baseline.Findings))
	for _, entry := range baseline.Findings {
		fingerprints[entry.Fingerprint] = struct{}{}
	}

	suppressed := 0
	for moduleIndex := range report.Modules {
		findings := report.Modules[moduleIndex].Findings
		active := findings[:0]
		for _, finding := range findings {
			if _, ok := fingerprints[finding.Fingerprint]; ok {
				suppressed++
				continue
			}
			active = append(active, finding)
		}
		report.Modules[moduleIndex].Findings = active
	}

	previousSuppressed := report.Summary.SuppressedFindings
	finalizeReport(report)
	report.Summary.SuppressedByBaseline = suppressed
	report.Summary.SuppressedFindings = previousSuppressed + suppressed

	return nil
}
