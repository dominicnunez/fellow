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

type duplicateGroup struct {
	key       string
	locations []duplicateWindow
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

	groups := make([]duplicateGroup, 0, len(keys))
	for _, key := range keys {
		locations := dedupeDuplicateWindows(windows[key])
		if len(locations) < 2 {
			continue
		}
		groups = append(groups, duplicateGroup{key: key, locations: locations})
	}

	groups = coalesceDuplicateGroups(groups)
	findings := make([]Finding, 0, len(groups))
	for _, group := range groups {
		locations := group.locations
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
			Symbol:      duplicateSymbol(group.key),
			File:        first.file,
			Line:        first.startLine,
			Lines:       duplicatedLineCount(locations),
			Locations:   findingLocations,
			Fingerprint: "",
		})
	}

	return findings
}

func coalesceDuplicateGroups(groups []duplicateGroup) []duplicateGroup {
	sort.Slice(groups, func(i, j int) bool {
		left := groups[i].locations[0]
		right := groups[j].locations[0]
		if left.file == right.file {
			return left.startLine < right.startLine
		}
		return left.file < right.file
	})

	coalesced := make([]duplicateGroup, 0, len(groups))
	for _, group := range groups {
		lastIndex := len(coalesced) - 1
		if lastIndex >= 0 && canMergeDuplicateGroups(coalesced[lastIndex], group) {
			mergeDuplicateGroup(&coalesced[lastIndex], group)
			continue
		}
		coalesced = append(coalesced, group)
	}

	return coalesced
}

func canMergeDuplicateGroups(left duplicateGroup, right duplicateGroup) bool {
	if len(left.locations) != len(right.locations) {
		return false
	}
	for i := range left.locations {
		leftLocation := left.locations[i]
		rightLocation := right.locations[i]
		if leftLocation.file != rightLocation.file {
			return false
		}
		if rightLocation.startLine < leftLocation.startLine || rightLocation.startLine > leftLocation.endLine+1 {
			return false
		}
	}

	return true
}

func mergeDuplicateGroup(left *duplicateGroup, right duplicateGroup) {
	for i := range left.locations {
		if right.locations[i].endLine > left.locations[i].endLine {
			left.locations[i].endLine = right.locations[i].endLine
		}
	}
}

func duplicatedLineCount(locations []duplicateWindow) int {
	if len(locations) < 2 {
		return 0
	}
	return (locations[0].endLine - locations[0].startLine + 1) * (len(locations) - 1)
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
