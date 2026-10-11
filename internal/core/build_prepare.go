package core

import (
	"fmt"
	"os"
)

type preparedBuild struct {
	notes       []preparedNote
	assets      []preparedAsset
	resolveMaps *resolveMaps
	metaTypes   map[string]MetaTypeInfo
	inputs      *buildInputs
}

type preparedNote struct {
	path  string
	mtime int64
	lines int
	links []linkOccur
	meta  []FrontmatterEntry
}

type preparedAsset struct {
	path  string
	mtime int64
}

func prepareBuild(vaultPath string, locations ...Locations) (*preparedBuild, error) {
	return prepareBuildWithout(vaultPath, "", locations...)
}

// Migration deliberately leaves reuse state invalid, since it removes an input
// after publication. The next ordinary build takes the full path.
func prepareBuildWithout(vaultPath, omittedAsset string, locations ...Locations) (*preparedBuild, error) {
	inputs, err := collectBuildInputs(vaultPath, omittedAsset, locations...)
	if err != nil {
		return nil, err
	}
	prepared := &preparedBuild{resolveMaps: newResolveMaps(inputs.notes, inputs.assets), metaTypes: inputs.cfg.Meta.Types, inputs: inputs}
	var userErrors []error
	for _, rel := range inputs.notes {
		note, err := readBuildNote(inputs, rel)
		if err != nil {
			return nil, err
		}
		for _, link := range note.links {
			if err := validateParsedLink(rel, link, prepared.resolveMaps); err != nil {
				userErrors = append(userErrors, err)
			}
			if len(userErrors) >= maxBuildErrors {
				break
			}
		}
		prepared.notes = append(prepared.notes, note)
		if len(userErrors) >= maxBuildErrors {
			break
		}
	}
	if len(userErrors) > 0 {
		return nil, formatBuildErrors(userErrors)
	}
	for _, rel := range inputs.assets {
		prepared.assets = append(prepared.assets, preparedAsset{rel, inputs.files[rel].seconds()})
	}
	return prepared, nil
}

func readBuildNote(inputs *buildInputs, rel string) (preparedNote, error) {
	fullPath, err := newVaultDiskPathResolver(inputs.root).existingPath(rel)
	if err != nil {
		return preparedNote{}, err
	}
	before, err := os.Stat(fullPath)
	if err != nil {
		return preparedNote{}, err
	}
	if signatureOf(before) != inputs.files[rel] {
		return preparedNote{}, buildInputChanged(rel)
	}
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return preparedNote{}, err
	}
	after, err := os.Stat(fullPath)
	if err != nil {
		return preparedNote{}, err
	}
	if signatureOf(after) != signatureOf(before) {
		return preparedNote{}, buildInputChanged(rel)
	}
	pr := parseLinksWithLinkKeys(string(content), inputs.cfg.Meta.LinkKeys)
	return preparedNote{path: rel, mtime: before.ModTime().Unix(), lines: countLines(string(content)), links: pr.Links, meta: pr.Meta}, nil
}

func buildInputChanged(path string) error {
	return fmt.Errorf("build input changed during build: %s; run build again", path)
}
