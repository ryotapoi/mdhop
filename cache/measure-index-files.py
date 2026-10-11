# Measure paired CLI operations on disposable documentation and 20,000-entry vaults.
import argparse, os, pathlib, shutil, subprocess, tempfile, time, json
parser = argparse.ArgumentParser()
parser.add_argument('before', type=pathlib.Path)
parser.add_argument('after', type=pathlib.Path)
args_cli = parser.parse_args()
binaries = {'before': str(args_cli.before.resolve()), 'after': str(args_cli.after.resolve())}
repo = pathlib.Path(__file__).resolve().parent.parent
root = pathlib.Path(tempfile.mkdtemp(prefix='mdhop-placement-'))
print('artifacts:', root, flush=True)
env = dict(os.environ, XDG_CACHE_HOME=str(root / 'cache'), TMPDIR=str(root))
rep = root / 'representative'
if not rep.exists():
    rep.mkdir()
    for name in ['docs', 'decisions']:
        shutil.copytree(repo / name, rep / name, ignore=shutil.ignore_patterns('.*'))
    shutil.copy2(repo / 'README.ja.md', rep / 'README.ja.md')
large = root / 'large'
if not large.exists():
    large.mkdir()
    for i in range(20000):
        d = large / f'group{i // 100:03d}'
        d.mkdir(exist_ok=True)
        if i % 4 == 0:
            (d / f'Asset{i:05d}.txt').write_text('asset\n')
        else:
            (d / f'Note{i:05d}.md').write_text(f'---\nstatus: active\nrank: {i}\n---\n# Note {i}\n[[Note00001]]\n#measured\n')
results = []
for vault in [rep, large]:
    for explicit in [False, True]:
        db = vault / 'generated' / 'selected.sqlite'
        args = ['--vault', str(vault)] + (['--db', str(db)] if explicit else [])
        subprocess.run([binaries['before'], 'build', *args], env=env, check=True, capture_output=True, timeout=60)
        for op in ['build', 'status']:
            for variant in ['before', 'after']:
                subprocess.run([binaries[variant], op, *args] + (['--format', 'json'] if op == 'status' else []), env=env, check=True, capture_output=True, timeout=60)
            for run in range(10 if vault == rep else 3):
                for variant in ['before', 'after'] if run % 2 == 0 else ['after', 'before']:
                    cmd = [binaries[variant], op, *args] + (['--format', 'json'] if op == 'status' else [])
                    start = time.perf_counter()
                    p = subprocess.run(cmd, env=env, capture_output=True, timeout=60)
                    elapsed = time.perf_counter() - start
                    if p.returncode:
                        raise RuntimeError(p.stderr.decode())
                    if op == 'status':
                        output = json.loads(p.stdout)
                        if any(output.values()):
                            raise RuntimeError(output)
                    elif p.stdout:
                        raise RuntimeError(p.stdout)
                    result = dict(vault=vault.name, explicit=explicit, operation=op, run=run, variant=variant, seconds=elapsed, exit=p.returncode)
                    results.append(result)
                    print(json.dumps(result), flush=True)
(root / 'product.json').write_text(json.dumps(results, indent=2))
for vault in [rep, large]:
    print(vault.name, 'notes', len(list(vault.rglob('*.md'))), 'input_files', sum((p.is_file() for p in vault.rglob('*') if 'generated' not in p.parts)))
# Validate completed indexes after timing, independently of status filtering.
for vault, expected_notes, expected_assets in [(rep, 42, 0), (large, 15000, 5000)]:
    for explicit in [False, True]:
        args = ['--vault', str(vault), '--format', 'json']
        if explicit:
            args += ['--db', str(vault / 'generated' / 'selected.sqlite')]
        p = subprocess.run([binaries['after'], 'stats', *args], env=env,
                           check=True, capture_output=True, timeout=60)
        stats = json.loads(p.stdout)
        if stats['notes_exists'] != expected_notes or stats['assets_total'] != expected_assets:
            raise RuntimeError(stats)
        print('verified index:', vault.name, explicit, json.dumps(stats), flush=True)
