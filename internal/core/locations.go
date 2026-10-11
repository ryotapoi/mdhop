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

// EffectivePaths identifies the selected files without reading or creating them.
type EffectivePaths struct {
	Vault  string `json:"vault"`
	Config string `json:"config"`
	DB     string `json:"db"`
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

// Paths resolves effective absolute locations without requiring a config or DB.
func Paths(vaultPath string, locations ...Locations) (*EffectivePaths, error) {
	vault, err := canonicalVault(vaultPath)
	if err != nil {
		return nil, err
	}
	config, err := filepath.Abs(selectedLocations(locations).ConfigFile(vaultPath))
	if err != nil {
		return nil, err
	}
	db, err := resolveDBPath(vault, locations...)
	if err != nil {
		return nil, err
	}
	return &EffectivePaths{Vault: vault, Config: config, DB: db}, nil
}

// rejectIndexResource keeps the selected index out of registered file operations.
// Resolve the disk spelling while preserving the final entry, as isIndexFile does.
func rejectIndexResource(vaultPath, path string, locations []Locations) error {
	diskPath, err := deleteDiskPath(newVaultDiskPathResolver(vaultPath), path)
	if err != nil {
		return err
	}
	protected, err := isIndexFile(vaultPath, diskPath, locations)
	if err != nil {
		return err
	}
	if protected {
		return fmt.Errorf("selected index resource cannot be registered or mutated: %s", path)
	}
	return nil
}
