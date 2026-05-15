package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const DefaultConfigFile = ".fellowrc.json"

const (
	RuleOff   = "off"
	RuleWarn  = "warn"
	RuleError = "error"
)

type Config struct {
	Root            string            `json:"root"`
	Format          string            `json:"format"`
	Production      *bool             `json:"production"`
	AllRequires     *bool             `json:"allRequires"`
	IgnoreGenerated *bool             `json:"ignoreGenerated"`
	Summary         *bool             `json:"summary"`
	FailOnIssues    *bool             `json:"failOnIssues"`
	Rules           map[string]string `json:"rules"`
	IgnorePatterns  []string          `json:"ignorePatterns"`
	IgnoreFindings  []FindingMatcher  `json:"ignoreFindings"`
	Workspace       []string          `json:"workspace"`
	BuildTags       []string          `json:"buildTags"`
	Health          Health            `json:"health"`
}

type FindingMatcher struct {
	Type        string `json:"type"`
	File        string `json:"file"`
	Package     string `json:"package"`
	Module      string `json:"module"`
	ImportPath  string `json:"importPath"`
	Symbol      string `json:"symbol"`
	Receiver    string `json:"receiver"`
	Struct      string `json:"struct"`
	Fingerprint string `json:"fingerprint"`
}

type Health struct {
	MaxCyclomatic int `json:"maxCyclomatic"`
	MaxCognitive  int `json:"maxCognitive"`
}

func Load(path string) (Config, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, false, nil
		}
		return Config{}, false, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, false, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, false, fmt.Errorf("validate config %s: %w", path, err)
	}

	return cfg, true, nil
}

func (c Config) Validate() error {
	for rule, severity := range c.Rules {
		switch severity {
		case RuleOff, RuleWarn, RuleError:
		default:
			return fmt.Errorf("rule %q has unsupported severity %q", rule, severity)
		}
	}

	return nil
}

func DefaultPath(root string) string {
	if root == "" {
		root = "."
	}
	return filepath.Join(root, DefaultConfigFile)
}
