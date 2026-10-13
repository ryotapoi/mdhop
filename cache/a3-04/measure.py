"""Paired CLI timings; instrumentation is supplied separately and never timed.
Budget: 15 min overall, 30 s/operation, 512 MiB peak RSS, <100 MiB fixture/output.
Usage: python3 cache/a3-04/measure.py BEFORE AFTER OUT [PROBE_BEFORE PROBE_AFTER]
"""
import hashlib, json, os, pathlib, platform, shutil, sqlite3, statistics, subprocess, sys, time, unicodedata
before, after, out = map(pathlib.Path, sys.argv[1:4])
probes = dict(zip(('before', 'after'), map(pathlib.Path, sys.argv[4:6])))
out = out.resolve(); out.mkdir(parents=True, exist_ok=True)
binaries = {'before': str(before.resolve()), 'after': str(after.resolve())}
started = time.monotonic()
records, observations, products = [], [], {}
env = dict(os.environ, XDG_CACHE_HOME=str(out/'cache'))

def execute(binary, args, trace=None):
    if time.monotonic()-started > 900: raise RuntimeError('overall time budget exceeded')
    command = [str(binary), *args]
    child_env = dict(env)
    if trace:
        trace.unlink(missing_ok=True); child_env['MDHOP_DIRECTORY_TRACE'] = str(trace)
    rssfile = out/'rss.txt'
    linux = platform.system() == 'Linux'
    wrapped = ['/usr/bin/time', '-f', '%M', '-o', str(rssfile), *command] if linux else ['/usr/bin/time', '-l', *command]
    t = time.perf_counter()
    p = subprocess.run(wrapped, env=child_env, capture_output=True, timeout=30)
    elapsed = time.perf_counter()-t
    if p.returncode: raise RuntimeError((command,p.returncode,p.stderr.decode()))
    if linux: rss = int(rssfile.read_text().strip())*1024
    else:
        rss = int(next(line.split()[0] for line in p.stderr.decode().splitlines() if 'maximum resident set size' in line))
    if rss > 512*1024*1024: raise RuntimeError('RSS budget exceeded')
    return elapsed, rss, p.stdout

def db_product(db):
    con = sqlite3.connect(db)
    tables = [x[0] for x in con.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")]
    product = {}
    for table in tables:
        rows = con.execute('SELECT * FROM "'+table+'"').fetchall()
        product[table] = sorted(rows, key=repr)
    con.close()
    return hashlib.sha256(json.dumps(product, sort_keys=True).encode()).hexdigest()

for count, runs in [(80,1),(1600,3)]:
  for spelling in ['exact','nfd']:
    case = f'{spelling}-{count}'
    vault = out/case; vault.mkdir(exist_ok=True)
    (vault/'Index.md').write_text('# Index\n')
    paths = []
    for i in range(count):
        directory = f'Café{i%4}'
        name = f'Résumé{i:04}.md'
        if spelling == 'nfd':
            directory = unicodedata.normalize('NFD',directory); name = unicodedata.normalize('NFD',name)
        folder = vault/directory; folder.mkdir(exist_ok=True)
        path = folder/name; path.write_text(f'# Note {i}\n[[Index]]\nbody\n'); paths.append(path)
    if spelling == 'nfd':
        nfc = vault/unicodedata.normalize('NFC',str(paths[0].relative_to(vault)))
        observation = {'case':case,'nfd_exists':paths[0].exists(),'nfc_direct_exists':nfc.exists()}
        observations.append(observation)
        print(json.dumps(observation),flush=True)
        if platform.system() == 'Linux' and nfc.exists(): raise RuntimeError('Linux fixture does not distinguish NFC/NFD')
    db = out/f'{case}.sqlite'; initial = out/f'{case}-initial.sqlite'
    common = ['--vault',str(vault),'--db',str(db)]
    execute(binaries['before'],['build','--full',*common]); shutil.copy2(db,initial)
    pristine = paths[0].read_bytes(); original_stat = paths[0].stat()
    ops = {
      'full': ['build','--full',*common],
      'unchanged': ['build',*common],
      'changed': ['build',*common],
      'query': ['query','--file','Index.md','--relations','backlinks','--include-head','2','--include-snippet','0','--format','json',*common],
      'search': ['search','--include-head','2','--format','json',*common],
    }
    def reset(op):
        paths[0].write_bytes(pristine)
        os.utime(paths[0],ns=(original_stat.st_atime_ns,original_stat.st_mtime_ns))
        shutil.copy2(initial,db)
        if op == 'full': db.unlink()
        if op == 'changed':
            paths[0].write_bytes(pristine+b'changed\n')
            os.utime(paths[0],ns=(original_stat.st_atime_ns,original_stat.st_mtime_ns+2_000_000_000))
    for op,args in ops.items():
      # Warm up both sides; pilot is excluded from adoption aggregates.
      for variant in binaries:
          reset(op); execute(binaries[variant],args)
      for run in range(runs):
        for variant in (['before','after'] if run%2==0 else ['after','before']):
            reset(op)
            elapsed,rss,stdout = execute(binaries[variant],args)
            if op in ('query','search'):
                payload = json.loads(stdout)
                items = payload['backlinks'] if op=='query' else payload['items']
                if len(items) != count+(op=='search'): raise RuntimeError(('item count',len(items),count,op))
                if not all(item.get('head') for item in items): raise RuntimeError('missing head')
                if op=='query' and not all(item.get('snippet') for item in items): raise RuntimeError('missing snippet')
                product = hashlib.sha256(stdout).hexdigest()
            else:
                if stdout: raise RuntimeError('unexpected build stdout')
                con=sqlite3.connect(db)
                notes=con.execute("SELECT count(*) FROM nodes WHERE type='note'").fetchone()[0]
                edges=con.execute('SELECT count(*) FROM edges').fetchone()[0]; con.close()
                if notes != count+1 or edges != count: raise RuntimeError(('index counts',notes,edges))
                product = db_product(db)
            key=(case,op)
            if key in products and products[key] != product: raise RuntimeError(('output/index mismatch',key,variant))
            products[key]=product
            row=dict(case=case,notes=count+1,operation=op,run=run,variant=variant,seconds=elapsed,peak_rss_bytes=rss,exit=0,product_sha256=product,pilot=count==80)
            records.append(row); print(json.dumps(row),flush=True)
      if count==1600 and op in ('full','query'):
        for variant,probe in probes.items():
            reset(op); trace=out/f'{case}-{op}-{variant}-directories.tsv'
            execute(probe.resolve(),args,trace)
            lines=[line.split('\t') for line in trace.read_text().splitlines()]
            observation={'case':case,'operation':op,'variant':variant,'read_dir_calls':len(lines),'normalized_entries':sum(int(line[-1]) for line in lines),'directory_counts':{directory:sum(1 for row in lines if row[0]==directory) for directory in sorted({row[0] for row in lines})}}
            observations.append(observation); print(json.dumps(observation),flush=True)
summary=[]
for case,op in products:
 if case.endswith('-80'): continue
 for variant in binaries:
    subset=[r for r in records if r['case']==case and r['operation']==op and r['variant']==variant]
    values=[r['seconds'] for r in subset]
    summary.append(dict(case=case,operation=op,variant=variant,runs=len(values),median_seconds=statistics.median(values),min_seconds=min(values),max_seconds=max(values),peak_rss_bytes=max(r['peak_rss_bytes'] for r in subset)))
result=dict(environment=dict(platform=platform.platform(),python=sys.version,go=subprocess.check_output(['go','version'],text=True).strip()),budget=dict(overall_seconds=900,operation_seconds=30,peak_rss_bytes=536870912,fixture_limit_bytes=104857600),records=records,summary=summary,observations=observations)
(out/'product.json').write_text(json.dumps(result,ensure_ascii=False,indent=2))
print('SUMMARY',json.dumps(summary),flush=True)
