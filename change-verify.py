# CHANGE-VERIFY: Disposable CLI validation for Unicode build collision acceptance.
import json, os, pathlib, platform, sqlite3, subprocess, tempfile, time, unicodedata
results = []
def run(vault, db, full=False, success=True):
    command = ['bin/mdhop', 'build', '--vault', str(vault), '--db', str(db)]
    if full: command += ['--full']
    start = time.monotonic()
    p = subprocess.run(command, capture_output=True, text=True)
    record = dict(command=command, exit_code=p.returncode, duration_seconds=round(time.monotonic()-start, 3), stdout=p.stdout, stderr=p.stderr)
    results.append(record)
    print(json.dumps(record, ensure_ascii=False), flush=True)
    assert (p.returncode == 0) == success, record
    assert p.stdout == '', record
    if success: assert p.stderr == '', record
    else:
        assert 'ambiguous normalized disk path: ' in p.stderr, record
        assert r'Caf\u00e9' in p.stderr and r'Cafe\u0301' in p.stderr, record
    return p

def snapshot(root):
    return {str(p.relative_to(root)): ('directory' if p.is_dir() else p.read_bytes().hex()) for p in root.rglob('*')}

with tempfile.TemporaryDirectory(prefix='mdhop-build-unicode-') as name:
    root = pathlib.Path(name)
    vault = root/'single'; vault.mkdir(); db = root/'single.sqlite'
    (vault/'Cafe\u0301.md').write_text('# Cafe\n')
    (vault/'Ref.md').write_text('[[Café]]\n')
    before = snapshot(vault)
    run(vault, db)
    run(vault, db, full=True)
    run(vault, db)
    assert snapshot(vault) == before
    with sqlite3.connect(db) as conn:
        assert conn.execute("select count(*) from nodes where path='Café.md' and type='note'").fetchone() == (1,)
        assert conn.execute("select count(*) from edges e join nodes n on n.id=e.target_id where n.path='Café.md' and n.type='note'").fetchone() == (1,)
    print('single NFD build/full/incremental success: NFC node and resolved link; vault unchanged', flush=True)
    for kind, mode in [('note','initial'), ('asset','full'), ('ancestor','incremental')]:
        vault = root/kind; vault.mkdir(); index = root/(kind+'-index'); index.mkdir(); db = index/'index.sqlite'
        (vault/'Keep.md').write_text('keep\n')
        if mode != 'initial': run(vault, db)
        suffix = '.md' if kind == 'note' else '.png' if kind == 'asset' else ''
        for spelling, body in [('Café','first'), ('Cafe\u0301','second')]:
            path = vault/(spelling+suffix)
            if kind == 'ancestor':
                path.mkdir(exist_ok=True); path = path/(body+'.md')
            path.write_text(body+'\n')
        entries = os.listdir(vault)
        matching = [s for s in entries if unicodedata.normalize('NFC',s) == 'Café'+suffix]
        print(json.dumps(dict(kind=kind, mode=mode, platform=platform.system(), entries=[dict(name=s, bytes=list(s.encode())) for s in matching], candidates=len(matching)),ensure_ascii=False), flush=True)
        if len(matching) != 2:
            assert platform.system() != 'Linux' and len(matching) == 1
            print('SKIP collision CLI: measured filesystem NFC/NFD aliasing', flush=True)
            continue
        before_vault, before_index = snapshot(vault), snapshot(index)
        run(vault, db, full=(mode=='full'), success=False)
        assert snapshot(vault) == before_vault
        assert snapshot(index) == before_index
        print('PASS collision CLI: empty stdout, nonzero exit, distinguishable names, disk/DB unchanged, no temp residue', flush=True)
print(json.dumps(dict(result='PASS', commands=results),ensure_ascii=False), flush=True)
