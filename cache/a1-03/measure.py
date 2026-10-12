#!/usr/bin/env python3
"""Reproduce A1-03 measurements with baseline/candidate binaries and a Plan probe."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import sqlite3
import statistics
import subprocess
import time
import tempfile

p = argparse.ArgumentParser()
p.add_argument('--baseline', required=True)
p.add_argument('--candidate', required=True)
p.add_argument('--probe', required=True)
p.add_argument('--output', required=True)
p.add_argument('--repeats', type=int, default=7)
a = p.parse_args()
root = Path(a.output).resolve()
root.mkdir(parents=True, exist_ok=True)
logs = []

def run(cmd):
    start = time.perf_counter()
    with tempfile.TemporaryFile() as stdout, tempfile.TemporaryFile() as stderr:
        child = subprocess.Popen(list(map(str, cmd)), stdout=stdout, stderr=stderr)
        while True:
            pid, status, usage = os.wait4(child.pid, os.WNOHANG)
            if pid:
                child.returncode = os.waitstatus_to_exitcode(status)
                break
            if time.perf_counter() - start > 60:
                child.kill()
                os.wait4(child.pid, 0)
                raise TimeoutError(cmd)
            time.sleep(0.001)
        stdout.seek(0)
        stderr.seek(0)
        r = subprocess.CompletedProcess(cmd, child.returncode, stdout.read().decode(), stderr.read().decode())
        r.usage = usage
    elapsed = time.perf_counter() - start
    logs.append(dict(command=list(map(str, cmd)), exit_code=r.returncode, duration=elapsed, stdout=r.stdout, stderr=r.stderr))
    (root / 'commands.json').write_text(json.dumps(logs, indent=2))
    if r.returncode:
        raise RuntimeError(r.stderr)
    return r, elapsed

def state(vault, db):
    return {str(f.relative_to(root)): hashlib.sha256(f.read_bytes()).hexdigest()
            for f in [*vault.rglob('*.md'), db]}

rows = []
for count in (1, 20, 200):
    case = root / str(count)
    vault, db = case / 'vault', case / 'index.sqlite'
    vault.mkdir(parents=True, exist_ok=True)
    (vault / 'src').mkdir()
    (vault / 'Target.md').write_text('# Target\n')
    for i in range(count):
        (vault / 'src' / f'N{i:04d}.md').write_text('---\nclient: Acme\nupdated: 2026-07-04\n---\n# Note\n[Target](../Target.md)\n' + 'body text\n' * 30)
    (vault / 'Index.md').write_text(''.join(f'[[src/N{i:04d}.md]]\n' for i in range(count)))
    run([a.baseline, 'build', '--vault', vault, '--db', db])
    snapshot = case / 'snapshot'
    shutil.copytree(vault, snapshot)
    shutil.copy2(db, case / 'snapshot.sqlite')
    assert sum(f.stat().st_size for f in case.rglob('*') if f.is_file()) < 1024**3
    def restore():
        shutil.rmtree(vault)
        shutil.copytree(snapshot, vault, copy_function=shutil.copy2)
        shutil.copy2(case / 'snapshot.sqlite', db)
        for suffix in ('-wal', '-shm', '-journal'):
            Path(str(db) + suffix).unlink(missing_ok=True)
    source = 'src/N0000.md' if count == 1 else 'src/'
    opts = ['--vault', vault, '--db', db, '--from', source, '--to-template', 'archive/{client}/{updated:year}/{basename}', '--format', 'json']
    # Warm caches with one full invocation of each binary, outside measurements.
    for binary in (a.baseline, a.candidate):
        restore()
        run([binary, 'move', *opts])
    # A/B alternates order to avoid systematic temporal bias; all runs restore mtimes/DB.
    for repeat in range(a.repeats):
        for version in (('baseline', 'candidate') if repeat % 2 == 0 else ('candidate', 'baseline')):
            restore()
            binary = getattr(a, version)
            r, elapsed = run([binary, 'move', *opts])
            got = json.loads(r.stdout)
            moved = got.get('moved', [dict(from_=got.get('from'), to=got.get('to'))])
            assert len(moved) == count and len(got['rewritten']) > 0
            for i in range(count):
                dest = f'archive/Acme/2026/N{i:04d}.md'
                assert not (vault / 'src' / f'N{i:04d}.md').exists()
                assert (vault / dest).exists()
                assert '[Target](../../../Target.md)' in (vault / dest).read_text()
            assert '[[src/' not in (vault / 'Index.md').read_text()
            with sqlite3.connect(db) as conn:
                assert conn.execute("select count(*) from nodes where path like 'archive/Acme/2026/%' and exists_flag=1").fetchone()[0] == count
                assert conn.execute("select count(*) from nodes where path like 'src/%' and exists_flag=1").fetchone()[0] == 0
            rows.append(dict(count=count, repeat=repeat, version=version, wall_seconds=elapsed, cpu_seconds=r.usage.ru_utime+r.usage.ru_stime, peak_rss_bytes=r.usage.ru_maxrss))
    for repeat in range(a.repeats):
        restore()
        before = state(vault, db)
        r, _ = run([a.probe, str(vault), str(db), source])
        metric = json.loads(r.stdout)
        assert metric['count'] == count and before == state(vault, db)
        rows.append(dict(repeat=repeat, version='plan_probe', **metric))
    # Actual candidate CLI dry-run and representative preflight failure are byte-preserving.
    restore()
    before = state(vault, db)
    r, _ = run([a.candidate, 'move', *opts, '--dry-run'])
    assert json.loads(r.stdout)['rewritten'] == [] and before == state(vault, db)
    cmd = [a.candidate, 'move', *opts]
    cmd[cmd.index('--to-template') + 1] = 'archive/{missing}/{basename}'
    started = time.perf_counter()
    r = subprocess.run(list(map(str,cmd)), capture_output=True, text=True, timeout=60)
    logs.append(dict(command=list(map(str,cmd)), exit_code=r.returncode, duration=time.perf_counter()-started, stdout=r.stdout, stderr=r.stderr))
    assert r.returncode != 0 and not r.stdout and 'template field missing' in r.stderr and before == state(vault, db)
    (root/'commands.json').write_text(json.dumps(logs,indent=2))
    (root/'results.json').write_text(json.dumps(rows,indent=2))
summary = {}
for count in (1,20,200):
    data = {}
    for version in ('baseline','candidate','plan_probe'):
        selected = [r for r in rows if r['count']==count and r['version']==version]
        keys = ('wall_seconds','cpu_seconds','peak_rss_bytes') if version!='plan_probe' else ('seconds','allocated_bytes')
        data[version] = {k: dict(median=statistics.median(r[k] for r in selected), min=min(r[k] for r in selected), max=max(r[k] for r in selected)) for k in keys}
    differences = [next(r['wall_seconds'] for r in rows if r['count']==count and r['repeat']==i and r['version']=='baseline') - next(r['wall_seconds'] for r in rows if r['count']==count and r['repeat']==i and r['version']=='candidate') for i in range(a.repeats)]
    data['paired_seconds'] = dict(median=statistics.median(differences), min=min(differences), max=max(differences))
    summary[count] = data
(root/'summary.json').write_text(json.dumps(summary,indent=2))
print(json.dumps(summary,indent=2))
