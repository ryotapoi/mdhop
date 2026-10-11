package core

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

// Migrate converts the fixed legacy configuration and rebuilds the default index.
// Legacy resources are removed only after the completed index is published.
func Migrate(vaultPath string) (*BuildResult, error) {
	vault, err := canonicalVault(vaultPath)
	if err != nil {
		return nil, err
	}
	legacy := filepath.Join(vault, "mdhop.yaml")
	config := filepath.Join(vault, "mdhop.toml")
	oldDir := filepath.Join(vault, dataDirName)
	exists := make(map[string]bool)
	for _, path := range []string{legacy, config, oldDir} {
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		exists[path] = true
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("migration target is a symlink: %s", path)
		}
		if path != oldDir && !info.Mode().IsRegular() {
			return nil, fmt.Errorf("configuration is not a regular file: %s", path)
		}
		if path == oldDir && !info.IsDir() {
			return nil, fmt.Errorf("legacy index path is not a directory: %s", path)
		}
	}
	if exists[legacy] && exists[config] {
		return nil, fmt.Errorf("both %s and %s exist; preserve or remove one before migrating", legacy, config)
	}
	dbp, err := resolveDBPath(vault)
	if err != nil {
		return nil, err
	}
	realDB, err := migrationPhysicalPath(dbp)
	if err != nil {
		return nil, err
	}
	if realDB == oldDir || strings.HasPrefix(realDB, oldDir+string(filepath.Separator)) || realDB == legacy || realDB == config {
		return nil, fmt.Errorf("default cache index conflicts with migration targets: %s", dbp)
	}
	created := false
	if exists[legacy] {
		data, err := os.ReadFile(legacy)
		if err != nil {
			return nil, err
		}
		converted, err := convertLegacyConfig(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", legacy, err)
		}
		file, err := os.OpenFile(config, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return nil, fmt.Errorf("save %s: %w", config, err)
		}
		created = true
		_, writeErr := file.Write(converted)
		err = errors.Join(writeErr, file.Close())
		if err != nil {
			return nil, rollbackMigrationConfig(config, fmt.Errorf("save %s: %w", config, err))
		}
	}
	prepared, err := prepareBuildWithout(vault, "mdhop.yaml")
	if err == nil {
		err = os.MkdirAll(filepath.Dir(dbp), 0o755)
	}
	var result *BuildResult
	if err == nil {
		result, err = buildPrepared(vault, prepared)
	}
	if err != nil {
		if created {
			err = rollbackMigrationConfig(config, err)
		}
		return nil, fmt.Errorf("rebuild failed; legacy configuration and index retained: %w", err)
	}
	if err := cleanupMigration(legacy, oldDir); err != nil {
		return nil, fmt.Errorf("index rebuilt at %s; configuration is %s; %w", dbp, config, err)
	}
	return result, nil
}

func rollbackMigrationConfig(path string, cause error) error {
	if err := os.Remove(path); err != nil {
		return errors.Join(cause, fmt.Errorf("rollback failed; generated configuration remains at %s: %w", path, err))
	}
	return cause
}

func cleanupMigration(legacy, oldDir string) error {
	var failures []error
	if err := os.Remove(legacy); err != nil && !os.IsNotExist(err) {
		failures = append(failures, fmt.Errorf("cleanup failed; legacy configuration remains at %s: %w", legacy, err))
	}
	if err := os.RemoveAll(oldDir); err != nil {
		failures = append(failures, fmt.Errorf("cleanup failed; legacy index may remain at %s: %w", oldDir, err))
	}
	return errors.Join(failures...)
}

// Resolve existing ancestors as well as an existing final symlink, so a cache
// alias cannot place the new index inside the directory scheduled for deletion.
func migrationPhysicalPath(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	parent := filepath.Dir(path)
	if parent == path {
		return "", err
	}
	resolved, err = migrationPhysicalPath(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolved, filepath.Base(path)), nil
}

// convertLegacyConfig accepts only the complete supported configuration schema.
// The YAML node boundary prevents unknown fields and nonrepresentable values
// from disappearing during conversion, including mixed metadata declarations.
func convertLegacyConfig(data []byte) ([]byte, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	err := decoder.Decode(&doc)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("expected one YAML document")
	}
	var raw any = map[string]any{}
	if len(doc.Content) != 0 && !(doc.Content[0].Tag == "!!null" && doc.Content[0].Value == "") {
		raw, err = legacyConfigValue(doc.Content[0], "", 0)
		if err != nil {
			return nil, err
		}
	}
	converted, err := toml.Marshal(raw)
	if err != nil {
		return nil, err
	}
	cfg, err := decodeConfig(converted, "converted configuration")
	if err != nil {
		return nil, err
	}
	for _, patterns := range [][]string{cfg.Build.ExcludePaths, cfg.Exclude.Paths, cfg.Query.Hide.Paths, cfg.Query.Via.Include.Paths} {
		if err := validateGlobPatterns(patterns); err != nil {
			return nil, err
		}
	}
	if cfg.Query.Via.Exclude != nil {
		if err := validateGlobPatterns(cfg.Query.Via.Exclude.Paths); err != nil {
			return nil, err
		}
	}
	return converted, nil
}

func legacyConfigValue(node *yaml.Node, path string, depth int) (any, error) {
	if depth > 100 {
		return nil, fmt.Errorf("%s: recursive or excessively nested YAML", path)
	}
	if node.Kind == yaml.AliasNode {
		return legacyConfigValue(node.Alias, path, depth+1)
	}
	// Select the allowed shape at each configuration boundary.
	var fields map[string]string
	shape := "map"
	switch path {
	case "":
		fields = map[string]string{"build": "build", "exclude": "exclude", "query": "query", "meta": "meta"}
	case "build":
		fields = map[string]string{"exclude_paths": "strings"}
	case "exclude", "query.hide", "query.via.include", "query.via.exclude":
		fields = map[string]string{"paths": "strings", "tags": "strings"}
	case "query":
		fields = map[string]string{"hide": "query.hide", "via": "query.via"}
	case "query.via":
		fields = map[string]string{"include": "query.via.include", "exclude": "query.via.exclude"}
	case "meta":
		fields = map[string]string{"types": "types", "profiles": "profiles", "link_keys": "strings"}
	case "profile":
		fields = map[string]string{"path": "string", "require": "strings"}
	case "type":
		if node.Kind == yaml.ScalarNode && node.Tag == "!!str" {
			return node.Value, nil
		}
		fields = map[string]string{"ordered": "strings"}
	case "types":
	case "strings", "profiles":
		shape = "array"
	case "string":
		shape = "string"
	}
	if shape == "string" {
		if node.Kind != yaml.ScalarNode {
			return nil, fmt.Errorf("%s: expected string", path)
		}
		var text string
		if err := node.Decode(&text); err != nil {
			return nil, err
		}
		return text, nil
	}
	if node.Tag == "!!null" && path != "query.via.exclude" && path != "type" {
		if shape == "array" {
			return []any{}, nil
		}
		if shape == "map" {
			return map[string]any{}, nil
		}
	}
	if shape == "array" {
		if node.Kind != yaml.SequenceNode {
			return nil, fmt.Errorf("%s: expected array", path)
		}
		values := make([]any, 0, len(node.Content))
		child := "string"
		if path == "profiles" {
			child = "profile"
		}
		for _, entry := range node.Content {
			value, err := legacyConfigValue(entry, child, depth+1)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
			values = append(values, value)
		}
		return values, nil
	}
	if node.Kind != yaml.MappingNode || node.Tag != "!!map" {
		return nil, fmt.Errorf("%s: expected mapping", path)
	}
	values := make(map[string]any)
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
			return nil, fmt.Errorf("%s: expected string key (YAML merge keys are unsupported)", path)
		}
		child, ok := fields[key.Value]
		if path == "types" {
			child, ok = "type", true
		}
		if !ok {
			return nil, fmt.Errorf("%s: unknown field %q", path, key.Value)
		}
		if _, exists := values[key.Value]; exists {
			return nil, fmt.Errorf("%s: duplicate field %q", path, key.Value)
		}
		value, err := legacyConfigValue(node.Content[i+1], child, depth+1)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", path, key.Value, err)
		}
		values[key.Value] = value
	}
	return values, nil
}
