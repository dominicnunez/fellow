package analyzer

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
)

const (
	diffHeaderMarker  = "+++ "
	diffHeaderBPrefix = "b/"
	goSumFileName     = "go.sum"
)

func moduleHygieneFindings(root string, modules []moduleState, module moduleState) ([]Finding, error) {
	findings := make([]Finding, 0)
	tidyFindings, err := tidyDriftFindings(root, module)
	if err != nil {
		return nil, err
	}
	findings = append(findings, tidyFindings...)
	findings = append(findings, localReplaceFindings(root, modules, module)...)
	findings = append(findings, unresolvedToolDependencyFindings(root, module)...)
	return findings, nil
}

func tidyDriftFindings(root string, module moduleState) ([]Finding, error) {
	cmd := exec.Command("go", "mod", "tidy", "-diff")
	cmd.Dir = module.absDir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		return nil, nil
	}
	if stdout.Len() == 0 || !strings.Contains(stdout.String(), diffHeaderMarker) {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("go mod tidy -diff %s: %s", module.report.Dir, message)
	}

	changedFiles := tidyDiffFiles(root, module.absDir, stdout.String())
	if len(changedFiles) == 0 {
		return []Finding{{
			Type: FindingTidyDrift,
			File: module.report.GoMod,
			Line: firstLine,
		}}, nil
	}

	locations := make([]Location, 0, len(changedFiles))
	for _, file := range changedFiles {
		locations = append(locations, Location{File: file, StartLine: firstLine, EndLine: firstLine})
	}

	return []Finding{{
		Type:      FindingTidyDrift,
		File:      changedFiles[0],
		Line:      firstLine,
		Locations: locations,
	}}, nil
}

func tidyDiffFiles(root string, moduleDir string, diff string) []string {
	seen := make(map[string]struct{})
	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, diffHeaderMarker) {
			continue
		}
		file := strings.TrimSpace(strings.TrimPrefix(line, diffHeaderMarker))
		file = strings.TrimPrefix(file, diffHeaderBPrefix)
		if file != goModFileName && file != goSumFileName {
			continue
		}
		seen[relPath(root, filepath.Join(moduleDir, file))] = struct{}{}
	}

	files := make([]string, 0, len(seen))
	for file := range seen {
		files = append(files, file)
	}
	sort.Strings(files)
	return files
}

func localReplaceFindings(root string, modules []moduleState, module moduleState) []Finding {
	parsed, err := parseGoMod(filepath.Join(module.absDir, goModFileName))
	if err != nil {
		return nil
	}

	moduleDirs := make(map[string]struct{}, len(modules))
	for _, known := range modules {
		moduleDirs[filepath.Clean(known.absDir)] = struct{}{}
	}

	findings := make([]Finding, 0)
	for _, replace := range parsed.Replace {
		if !isLocalReplacePath(replace.New.Path) {
			continue
		}

		resolved := replace.New.Path
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(module.absDir, resolved)
		}
		resolved = filepath.Clean(resolved)
		if _, ok := moduleDirs[resolved]; ok {
			continue
		}

		findings = append(findings, Finding{
			Type:       FindingLocalReplace,
			Module:     replace.Old.Path,
			ImportPath: replace.New.Path,
			File:       relPath(root, filepath.Join(module.absDir, goModFileName)),
			Line:       modfileLine(replace.Syntax),
		})
	}

	return findings
}

func unresolvedToolDependencyFindings(root string, module moduleState) []Finding {
	uses := toolDependencyUses(root, module)
	findings := make([]Finding, 0)
	for _, use := range uses {
		if _, ok := longestMatchingRequire(use.ImportPath, module.requires); ok {
			continue
		}
		findings = append(findings, Finding{
			Type:       FindingToolDependency,
			ImportPath: use.ImportPath,
			File:       use.File,
			Line:       use.Line,
		})
	}
	return findings
}

func toolDependencyUses(root string, module moduleState) []ImportUse {
	parsed, err := parseGoMod(filepath.Join(module.absDir, goModFileName))
	if err != nil {
		return nil
	}

	uses := make([]ImportUse, 0, len(parsed.Tool))
	for _, tool := range parsed.Tool {
		uses = append(uses, ImportUse{
			ImportPath: tool.Path,
			File:       relPath(root, filepath.Join(module.absDir, goModFileName)),
			Line:       modfileLine(tool.Syntax),
			Tool:       true,
		})
	}
	return uses
}

func parseGoMod(goModPath string) (*modfile.File, error) {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", goModPath, err)
	}
	return modfile.Parse(goModPath, data, nil)
}

func modfileLine(line *modfile.Line) int {
	if line == nil || line.Start.Line == 0 {
		return firstLine
	}
	return line.Start.Line
}

func isLocalReplacePath(value string) bool {
	return filepath.IsAbs(value) || strings.HasPrefix(value, ".")
}
