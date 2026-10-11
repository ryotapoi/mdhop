package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const cacheOriginName = "origin.json"

// CacheEntry describes one existing default cache folder.
type CacheEntry struct {
	CachePath string  `json:"cache_path"`
	SizeBytes int64   `json:"size_bytes"`
	Vault     *string `json:"vault"`
}

type cacheOrigin struct {
	Vault string `json:"vault"`
}

func isCacheHash(name string) bool {
	return len(name) == 64 && strings.Trim(name, "0123456789abcdef") == ""
}

// ListCaches reads the current default cache without opening a vault or DB.
// Sizes sum allocated 512-byte blocks for all entries, including directories;
// symlinks are not followed and hardlinks are counted per entry.
func ListCaches() ([]CacheEntry, error) {
	root, err := cacheVaultsRoot()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	result := make([]CacheEntry, 0)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	// ReadDir sorts names, which also sorts these absolute cache paths.
	for _, entry := range entries {
		if !entry.IsDir() || !isCacheHash(entry.Name()) {
			continue
		}
		path := filepath.Join(root, entry.Name())
		vault, err := readCacheOrigin(path)
		if err != nil {
			return nil, err
		}
		size, err := cacheDiskUsage(path)
		if err != nil {
			return nil, err
		}
		result = append(result, CacheEntry{CachePath: path, SizeBytes: size, Vault: vault})
	}
	return result, nil
}

func cacheDiskUsage(root string) (int64, error) {
	var size int64
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info() // lstat semantics: count the symlink entry itself.
		if err != nil {
			return err
		}
		stat := info.Sys().(*syscall.Stat_t)
		size += stat.Blocks * 512
		return nil
	})
	return size, err
}

func readCacheOrigin(folder string) (*string, error) {
	path := filepath.Join(folder, cacheOriginName)
	_, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var origin cacheOrigin
	if err := json.Unmarshal(data, &origin); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if !filepath.IsAbs(origin.Vault) {
		return nil, fmt.Errorf("%s: vault must be an absolute path", path)
	}
	return &origin.Vault, nil
}

func writeCacheOrigin(vaultPath, folder string) error {
	vault, err := canonicalVault(vaultPath)
	if err != nil {
		return err
	}
	data, err := json.Marshal(cacheOrigin{Vault: vault})
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(folder, cacheOriginName+".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, writeErr := tmp.Write(append(data, '\n'))
	closeErr := tmp.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(tmp.Name(), filepath.Join(folder, cacheOriginName))
}
