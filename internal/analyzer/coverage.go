package analyzer

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type coverageRange struct {
	file      string
	startLine int
	endLine   int
	count     int
}

func ApplyCoverage(path string, report *Report) error {
	ranges, err := parseCoverageProfile(path)
	if err != nil {
		return err
	}
	if len(ranges) == 0 {
		return nil
	}

	for moduleIndex := range report.Modules {
		for findingIndex := range report.Modules[moduleIndex].Findings {
			finding := &report.Modules[moduleIndex].Findings[findingIndex]
			coverage, ok := coverageForFinding(*finding, ranges)
			if !ok {
				continue
			}
			finding.Coverage = &coverage
		}
	}
	RefreshReport(report)

	return nil
}

func parseCoverageProfile(path string) ([]coverageRange, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open coverage profile %s: %w", path, err)
	}
	defer file.Close()

	var ranges []coverageRange
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		entry, err := parseCoverageLine(line)
		if err != nil {
			return nil, err
		}
		ranges = append(ranges, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read coverage profile %s: %w", path, err)
	}

	return ranges, nil
}

func parseCoverageLine(line string) (coverageRange, error) {
	parts := strings.Fields(line)
	if len(parts) != 3 {
		return coverageRange{}, fmt.Errorf("invalid coverage line %q", line)
	}
	filePart, rangePart, ok := strings.Cut(parts[0], ":")
	if !ok {
		return coverageRange{}, fmt.Errorf("invalid coverage range %q", line)
	}
	startPart, endPart, ok := strings.Cut(rangePart, ",")
	if !ok {
		return coverageRange{}, fmt.Errorf("invalid coverage range %q", line)
	}
	startLine, err := coverageLineNumber(startPart)
	if err != nil {
		return coverageRange{}, err
	}
	endLine, err := coverageLineNumber(endPart)
	if err != nil {
		return coverageRange{}, err
	}
	count, err := strconv.Atoi(parts[2])
	if err != nil {
		return coverageRange{}, fmt.Errorf("invalid coverage count %q: %w", parts[2], err)
	}

	return coverageRange{
		file:      normalizePatternPath(filePart),
		startLine: startLine,
		endLine:   endLine,
		count:     count,
	}, nil
}

func coverageLineNumber(value string) (int, error) {
	line, _, _ := strings.Cut(value, ".")
	n, err := strconv.Atoi(line)
	if err != nil {
		return 0, fmt.Errorf("invalid coverage line %q: %w", value, err)
	}

	return n, nil
}

func coverageForFinding(finding Finding, ranges []coverageRange) (Coverage, bool) {
	matched := false
	maxCount := 0
	for _, coverage := range ranges {
		if !coverageFileMatches(finding.File, coverage.file) {
			continue
		}
		if finding.Line < coverage.startLine || finding.Line > coverage.endLine {
			continue
		}
		matched = true
		if coverage.count > maxCount {
			maxCount = coverage.count
		}
	}

	return Coverage{Covered: maxCount > 0, Count: maxCount}, matched
}

func coverageFileMatches(findingFile string, coverageFile string) bool {
	findingFile = normalizePatternPath(findingFile)
	coverageFile = normalizePatternPath(coverageFile)
	return findingFile == coverageFile || strings.HasSuffix(coverageFile, "/"+findingFile)
}
