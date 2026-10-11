package core

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
)

// Locations selects index and configuration files independently of the vault.
// Relative paths are relative to the process working directory.
// Empty paths select the default cache index and vault-local configuration.
type Locations struct {
	DBPath     string
	ConfigPath string
}

// ConfigFile returns the selected configuration filename.
func (l Locations) ConfigFile(vaultPath string) string {
	if l.ConfigPath != "" {
		return l.ConfigPath
	}
	return filepath.Join(vaultPath, "mdhop.toml")
}

func selectedLocations(locations []Locations) Locations {
	if len(locations) > 0 {
		return locations[0]
	}
	return Locations{}
}

func canonicalVault(vault string) (string, error) {
	absolute, err := filepath.Abs(vault)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(absolute)
}

func resolveDBPath(vaultPath string, locations ...Locations) (string, error) {
	if p := selectedLocations(locations).DBPath; p != "" {
		return filepath.Abs(p)
	}
	vault, err := canonicalVault(vaultPath)
	if err != nil {
		return "", err
	}
	cache := os.Getenv("XDG_CACHE_HOME")
	if !filepath.IsAbs(cache) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		cache, err = filepath.Abs(filepath.Join(home, ".cache"))
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(cache, "mdhop", "vaults", fmt.Sprintf("%x", sha256.Sum256([]byte(vault))), dbFileName), nil
}
