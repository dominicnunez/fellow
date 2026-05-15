package analyzer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/scanner"
	"go/token"
	"os"
	"sort"
	"strings"
)

const (
	duplicateWindowLines   = 6
	duplicateWindowTokens  = 50
	duplicateTokenMinLines = 8
	duplicateHashLength    = 12
)

const (
	duplicateLineKeyPrefix  = "line:"
	duplicateTokenKeyPrefix = "token:"
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
			collectTokenDuplicateWindows(windows, source.relPath, data, source.file, source.fset)
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
	groups = removeOverlappingDuplicateGroups(groups)
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

func removeOverlappingDuplicateGroups(groups []duplicateGroup) []duplicateGroup {
	filtered := make([]duplicateGroup, 0, len(groups))
	for _, group := range groups {
		if overlapsAnyDuplicateGroup(group, filtered) {
			continue
		}
		filtered = append(filtered, group)
	}

	return filtered
}

func overlapsAnyDuplicateGroup(group duplicateGroup, groups []duplicateGroup) bool {
	for _, existing := range groups {
		if duplicateGroupsOverlap(existing, group) {
			return true
		}
	}

	return false
}

func duplicateGroupsOverlap(left duplicateGroup, right duplicateGroup) bool {
	for _, leftLocation := range left.locations {
		for _, rightLocation := range right.locations {
			if duplicateWindowsOverlap(leftLocation, rightLocation) {
				return true
			}
		}
	}

	return false
}

func duplicateWindowsOverlap(left duplicateWindow, right duplicateWindow) bool {
	if left.file != right.file {
		return false
	}
	return left.startLine <= right.endLine && right.startLine <= left.endLine
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
		key := duplicateKey(duplicateLineKeyPrefix + chunk)
		windows[key] = append(windows[key], duplicateWindow{
			file:      file,
			startLine: lineNumbers[i],
			endLine:   lineNumbers[i+duplicateWindowLines-1],
		})
	}
}

type duplicateToken struct {
	value string
	line  int
}

func collectTokenDuplicateWindows(windows map[string][]duplicateWindow, file string, content []byte, syntax *ast.File, fset *token.FileSet) {
	tokens := duplicateTokens(file, content)
	for _, body := range functionBodyLineRanges(syntax, fset) {
		collectTokenSequenceDuplicateWindows(windows, file, tokensInLineRange(tokens, body))
	}
}

func duplicateTokens(file string, content []byte) []duplicateToken {
	fset := token.NewFileSet()
	tokenFile := fset.AddFile(file, fset.Base(), len(content))
	var scan scanner.Scanner
	scan.Init(tokenFile, content, nil, 0)

	tokens := make([]duplicateToken, 0)
	previous := token.ILLEGAL
	for {
		pos, tok, _ := scan.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.SEMICOLON {
			continue
		}
		value, ok := normalizeDuplicateToken(tok, previous)
		if !ok {
			continue
		}
		tokens = append(tokens, duplicateToken{
			value: value,
			line:  fset.Position(pos).Line,
		})
		previous = tok
	}

	return tokens
}

func functionBodyLineRanges(syntax *ast.File, fset *token.FileSet) []duplicateWindow {
	if syntax == nil || fset == nil {
		return nil
	}

	ranges := make([]duplicateWindow, 0)
	for _, decl := range syntax.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		ranges = append(ranges, duplicateWindow{
			startLine: fset.Position(fn.Body.Lbrace).Line,
			endLine:   fset.Position(fn.Body.Rbrace).Line,
		})
	}

	return ranges
}

func tokensInLineRange(tokens []duplicateToken, lines duplicateWindow) []duplicateToken {
	filtered := make([]duplicateToken, 0)
	for _, tok := range tokens {
		if tok.line < lines.startLine || tok.line > lines.endLine {
			continue
		}
		filtered = append(filtered, tok)
	}

	return filtered
}

func collectTokenSequenceDuplicateWindows(windows map[string][]duplicateWindow, file string, tokens []duplicateToken) {
	if len(tokens) < duplicateWindowTokens {
		return
	}

	for i := 0; i <= len(tokens)-duplicateWindowTokens; i++ {
		end := i + duplicateWindowTokens - 1
		startLine := tokens[i].line
		endLine := tokens[end].line
		if endLine-startLine+1 < duplicateTokenMinLines {
			continue
		}

		key := duplicateKey(duplicateTokenKeyPrefix + duplicateTokenChunk(tokens[i:i+duplicateWindowTokens]))
		windows[key] = append(windows[key], duplicateWindow{
			file:      file,
			startLine: startLine,
			endLine:   endLine,
		})
	}
}

func normalizeDuplicateToken(tok token.Token, previous token.Token) (string, bool) {
	switch {
	case tok == token.IDENT:
		if previous == token.PERIOD {
			return "FIELD", true
		}
		return tok.String(), true
	case tok.IsLiteral():
		return tok.String(), true
	case tok == token.ILLEGAL || tok == token.COMMENT:
		return "", false
	default:
		return tok.String(), true
	}
}

func duplicateTokenChunk(tokens []duplicateToken) string {
	values := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		values = append(values, tok.value)
	}

	return strings.Join(values, " ")
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

	return mergeOverlappingDuplicateWindows(deduped)
}

func mergeOverlappingDuplicateWindows(locations []duplicateWindow) []duplicateWindow {
	merged := make([]duplicateWindow, 0, len(locations))
	for _, location := range locations {
		lastIndex := len(merged) - 1
		if lastIndex < 0 || !duplicateWindowsCanMerge(merged[lastIndex], location) {
			merged = append(merged, location)
			continue
		}
		if location.endLine > merged[lastIndex].endLine {
			merged[lastIndex].endLine = location.endLine
		}
	}

	return merged
}

func duplicateWindowsCanMerge(left duplicateWindow, right duplicateWindow) bool {
	return left.file == right.file && right.startLine <= left.endLine+1
}
