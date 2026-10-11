#!/usr/bin/env python3
"""Make an isolated diagnostic source copy; never modify production sources."""
from pathlib import Path
import shutil
import sys

root = Path(__file__).resolve().parents[1]
target = Path(sys.argv[1]).resolve()
target.mkdir(parents=True, exist_ok=True)
for name in ['go.mod', 'go.sum']:
    shutil.copy2(root / name, target / name)
for name in ['internal', 'cmd']:
    shutil.copytree(root / name, target / name, dirs_exist_ok=True)
p = target / 'internal/core/build_incremental.go'
s = p.read_text().replace('"database/sql"', '"database/sql"\n"fmt"')
for signature, field in [
    ('func collectBuildInputs(vaultPath, omitted string, locations ...Locations) (*buildInputs, error) {', 'InventoryNS'),
    ('func copyBuildDB(source, destination string) error {', 'CopyNS'),
    ('func affectedBuildSources(db dbExecer, oldMaps, newMaps *resolveMaps) (map[string]bool, error) {', 'DependencyNS'),
    ('func loadBuildState(path, fingerprint string) (*savedBuildState, error) {', 'StateNS'),
    ('func refreshBuildVirtualNames(db dbExecer, notes []string, rm *resolveMaps, links map[string][]linkOccur, keys map[string]bool) error {', 'NamesNS'),
]:
    assert signature in s, signature
    s = s.replace(signature, signature + '\nstart:=time.Now();defer func(){buildTrace.' + field + '+=time.Since(start).Nanoseconds()}()')
s = s.replace('if target, ok := strings.CutPrefix(dependency, "basename:"); ok {', 'if target, ok := strings.CutPrefix(dependency, "basename:"); ok {\nbuildTrace.BasenameEvaluations++')
s = s.replace('func (r buildVirtualNameResolver) resolveBasename(target string, _ linkOccur) (buildVirtualName, string, error) {', 'func (r buildVirtualNameResolver) resolveBasename(target string, _ linkOccur) (buildVirtualName, string, error) {\nbuildTrace.BasenameEvaluations++')
s = s.replace('func build(vaultPath string, full bool, locations ...Locations) (*BuildResult, error) {', 'func build(vaultPath string, full bool, locations ...Locations) (*BuildResult, error) {\nstart:=time.Now();defer func(){buildTrace.TotalNS=time.Since(start).Nanoseconds(); b,_:=json.Marshal(buildTrace);fmt.Fprintln(os.Stderr,"BUILD_TRACE "+string(b))}()')
s += '\nvar buildTrace struct{InventoryNS,StateNS,CopyNS,DependencyNS,NamesNS,TotalNS int64; ReadParseNotes,ResolvedLinks,BasenameEvaluations int}\n'
p.write_text(s)
p = target / 'internal/core/build_prepare.go'
s = p.read_text().replace('pr := parseLinksWithLinkKeys', 'buildTrace.ReadParseNotes++\npr := parseLinksWithLinkKeys')
p.write_text(s)
p = target / 'internal/core/build.go'
s = p.read_text().replace('func resolveLink(db dbExecer, sourcePath string, link linkOccur, rm *resolveMaps) (int64, string, error) {', 'func resolveLink(db dbExecer, sourcePath string, link linkOccur, rm *resolveMaps) (int64, string, error) {\nbuildTrace.ResolvedLinks++')
p.write_text(s)
print(target)
