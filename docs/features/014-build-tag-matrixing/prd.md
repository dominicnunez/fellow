# PRD 014: Build Tag Matrixing

Status: Draft

## Goal

Analyze Go code across configured build contexts so platform-specific code is handled correctly.

## Users

- Developers maintaining cross-platform Go code.
- CI maintainers validating multiple targets.
- Reviewers checking build-tagged code.

## Requirements

- Accept custom build tags.
- Support multiple `GOOS` and `GOARCH` combinations.
- Include build context metadata on findings.
- Merge duplicate findings where appropriate.
- Default to current environment when no matrix is configured.

## Non-Functional Requirements

- Matrix size must be bounded.
- Output must distinguish context-specific findings.
- Analysis should not run builds or tests for every context.

## Acceptance Criteria

- OS-specific fixtures are analyzed for configured targets.
- Context-specific findings include target metadata.
- Default behavior matches current environment analysis.
- JSON includes build context data.
