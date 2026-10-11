package core

import "path/filepath"

// Locations selects index and configuration files independently of the vault.
// Relative paths are relative to the process working directory.
// Empty paths retain the default vault-local locations.
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
