#!/usr/bin/env python3
"""Compare complete meta-check commands on isolated generated vaults (Darwin)."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import statistics
import subprocess
import time


def snapshot(root):
    return {str(p.relative_to(root)): hashlib.sha256(p.read_bytes()).hexdigest()
            for p in sorted(root.rglob('*')) if p.is_file()}


def run(cmd):
    start = time.monotonic()
    p = subprocess.run(cmd, capture_output=True, timeout=60)
    return p, time.monotonic() - start


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--baseline', type=Path, required=True)
    ap.add_argument('--candidate', type=Path)
    ap.add_argument('--out', type=Path, required=True)
    ap.add_argument('--notes', type=int, default=1500)
    ap.add_argument('--assets', type=int, default=1500)
    ap.add_argument('--references', type=int, default=300)
    ap.add_argument('--repeats', type=int, default=7)
    ap.add_argument('--instrumented', type=Path, nargs='*', default=[])
    args = ap.parse_args()
    out = args.out.resolve()
    out.mkdir(parents=True, exist_ok=True)
    vault = out / 'vault'
    if vault.exists():
        raise SystemExit('output vault already exists; use a fresh output directory')
    vault.mkdir()
    for i in range(100):
        (vault / f'd{i:03}').mkdir()
    for i in range(args.references):
        (vault / f'Cafe\u0301{i:04}').mkdir()
    for i in range(args.notes):
        (vault / f'd{i % 100:03}' / f'n{i:05}.md').write_text('# Note\n')
    for i in range(args.assets):
        (vault / f'd{i % 100:03}' / f'a{i:05}.dat').write_bytes(b'asset\n')
    values = {
        'missing': [f'/absent{i:04}/' for i in range(args.references)],
        'unicode': [f'/Caf\u00e9{i:04}/' for i in range(args.references)],
        'direct': [f'/d{i % 100:03}/' for i in range(args.references)],
    }
    note = '---\n' + ''.join(k + ':\n' + ''.join('  - ' + json.dumps(v, ensure_ascii=False) + '\n' for v in vs)
                                     for k, vs in values.items()) + '---\n'
    (vault / 'Index.md').write_text(note)
    db = out / 'index.sqlite'
    binaries = [args.baseline.resolve()]
    if args.candidate:
        binaries.append(args.candidate.resolve())
    p, seconds = run([str(binaries[0]), 'build', '--vault', str(vault), '--db', str(db)])
    if p.returncode:
        raise RuntimeError(p.stderr.decode())
    before = snapshot(out)
    results = {'environment': platform.platform(), 'scale': {
        'notes': args.notes + 1, 'assets': args.assets, 'directories_excluding_root': 100 + args.references,
        'references_per_key': args.references, 'total_metadata_references': 3 * args.references,
        'unicode_direct_stat_succeeds': (vault / 'Caf\u00e90000').is_dir()},
        'build_seconds': seconds, 'samples': [], 'contracts': [], 'instrumentation': []}
    def command(binary, key, kind='path', fmt='json'):
        return [str(binary), 'meta-check', '--vault', str(vault), '--db', str(db), '--key', key,
                '--kind', kind, '--format', fmt]
    for key in values:
        for kind in ['path', 'auto']:
            for fmt in ['json', 'text']:
                observations = [run(command(b, key, kind, fmt))[0] for b in binaries]
                a = observations[0]
                assert a.returncode == 0 and not a.stderr, (key, a.stderr)
                if fmt == 'json':
                    issues = json.loads(a.stdout)['issues']
                    assert len(issues or []) == (args.references if key == 'missing' else 0)
                for b in observations[1:]:
                    assert (a.returncode, a.stdout, a.stderr) == (b.returncode, b.stdout, b.stderr)
                results['contracts'].append({'key': key, 'kind': kind, 'format': fmt,
                    'exit': a.returncode, 'stdout_sha256': hashlib.sha256(a.stdout).hexdigest(),
                    'identical': True})
    for flags in [['--key', 'missing', '--kind', 'invalid'], ['--key', 'missing', '--format', 'invalid'], []]:
        cmd = ['meta-check', '--vault', str(vault), '--db', str(db)] + flags
        observations = [run([str(b)] + cmd)[0] for b in binaries]
        a = observations[0]
        for b in observations[1:]:
            assert (a.returncode, a.stdout, a.stderr) == (b.returncode, b.stdout, b.stderr)
        results['contracts'].append({'flags': flags, 'exit': a.returncode, 'identical': True})
    # Warm OS page caches and runtime startup; no application state crosses processes.
    for key in values:
        for b in binaries:
            run(command(b, key))
        for iteration in range(args.repeats):
            order = list(enumerate(binaries))
            if iteration % 2:
                order.reverse()
            for index, b in order:
                p, wall = run(['/usr/bin/time', '-l'] + command(b, key))
                assert p.returncode == 0, p.stderr
                stderr = p.stderr.decode()
                cpu = re.search(r'([\d.]+) real\s+([\d.]+) user\s+([\d.]+) sys', stderr)
                rss = re.search(r'(\d+)\s+maximum resident set size', stderr)
                assert cpu and rss, stderr
                results['samples'].append({'key': key, 'version': ['baseline', 'candidate'][index],
                    'iteration': iteration, 'wall_seconds': wall, 'user_seconds': float(cpu[2]),
                    'system_seconds': float(cpu[3]), 'peak_rss_bytes': int(rss[1])})
    for binary in args.instrumented:
        for key in values:
            p, wall = run(command(binary.resolve(), key))
            assert p.returncode == 0, p.stderr
            results['instrumentation'].append({'binary': binary.name, 'key': key,
                'wall_seconds': wall, 'stderr': p.stderr.decode()})
    results['unchanged_vault_and_db'] = before == snapshot(out)
    assert results['unchanged_vault_and_db']
    results['summary'] = {}
    for key in values:
        results['summary'][key] = {}
        for version in ['baseline', 'candidate'][:len(binaries)]:
            ss = [s for s in results['samples'] if s['key'] == key and s['version'] == version]
            results['summary'][key][version] = {metric: {
                'median': statistics.median(s[metric] for s in ss),
                'min': min(s[metric] for s in ss), 'max': max(s[metric] for s in ss)}
                for metric in ['wall_seconds', 'user_seconds', 'system_seconds', 'peak_rss_bytes']}
    (out / 'results.json').write_text(json.dumps(results, indent=2, ensure_ascii=False) + '\n')
    print(json.dumps({'scale': results['scale'], 'summary': results['summary']}, indent=2))

if __name__ == '__main__':
    main()
