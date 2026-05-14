package analyzer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"
)

const (
	duplicateWindowLines = 6
	duplicateHashLength  = 12
)

type duplicateWindow struct {
	file      string
	startLine int
	endLine   int
}

func duplicateFindings(module moduleState) []Finding {
	windows := make(map[string][]duplicateWindow)
	for _, pkg := range module.packages {
		for _, source := range pkg.files {
			data, err := os.ReadFile(source.absPath)
			if err != nil {
				continue
			}
			collectDuplicateWindows(windows, source.relPath, string(data))
		}
	}

	keys := make([]string, 0, len(windows))
	for key, locations := range windows {
		if len(locations) > 1 {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	findings := make([]Finding, 0, len(keys))
	for _, key := range keys {
		locations := dedupeDuplicateWindows(windows[key])
		if len(locations) < 2 {
			continue
		}
		first := locations[0]
		findingLocations := make([]Location, 0, len(locations))
		for _, loc := range locations {
			findingLocations = append(findingLocations, Location{
				File:      loc.file,
				StartLine: loc.startLine,
				EndLine:   loc.endLine,
			})
		}

		findings = append(findings, Finding{
			Type:        FindingDuplicateCode,
			Symbol:      duplicateSymbol(key),
			File:        first.file,
			Line:        first.startLine,
			Lines:       duplicateWindowLines * (len(locations) - 1),
			Locations:   findingLocations,
			Fingerprint: "",
		})
	}

	return findings
}

func collectDuplicateWindows(windows map[string][]duplicateWindow, file string, content string) {
	lines := strings.Split(content, "\n")
	normalized := make([]string, 0, len(lines))
	lineNumbers := make([]int, 0, len(lines))
	for i, line := range lines {
		line = normalizeDuplicateLine(line)
		if line == "" {
			continue
		}
		normalized = append(normalized, line)
		lineNumbers = append(lineNumbers, i+1)
	}
	if len(normalized) < duplicateWindowLines {
		return
	}

	for i := 0; i <= len(normalized)-duplicateWindowLines; i++ {
		chunk := strings.Join(normalized[i:i+duplicateWindowLines], "\n")
		key := duplicateKey(chunk)
		windows[key] = append(windows[key], duplicateWindow{
			file:      file,
			startLine: lineNumbers[i],
			endLine:   lineNumbers[i+duplicateWindowLines-1],
		})
	}
}

func normalizeDuplicateLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "//") {
		return ""
	}
	return strings.Join(strings.Fields(line), " ")
}

func duplicateKey(chunk string) string {
	sum := sha256.Sum256([]byte(chunk))
	return hex.EncodeToString(sum[:])
}

func duplicateSymbol(key string) string {
	return fmt.Sprintf("clone-%s", key[:duplicateHashLength])
}

func dedupeDuplicateWindows(locations []duplicateWindow) []duplicateWindow {
	sort.Slice(locations, func(i, j int) bool {
		if locations[i].file == locations[j].file {
			return locations[i].startLine < locations[j].startLine
		}
		return locations[i].file < locations[j].file
	})

	seenFiles := make(map[string]struct{}, len(locations))
	deduped := make([]duplicateWindow, 0, len(locations))
	for _, location := range locations {
		key := fmt.Sprintf("%s:%d", location.file, location.startLine)
		if _, ok := seenFiles[key]; ok {
			continue
		}
		seenFiles[key] = struct{}{}
		deduped = append(deduped, location)
	}

	return deduped
}
