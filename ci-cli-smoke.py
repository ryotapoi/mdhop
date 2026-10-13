# CHANGE-VERIFY: Real CLI routing and mutation evidence for A1-02.
import pathlib, tempfile, subprocess, json, sqlite3, os, platform, hashlib

def snapshot(vault,db):
 return ({str(p.relative_to(vault)):p.read_bytes() for p in vault.rglob('*') if p.is_file()},db.read_bytes())

def run(vault,db,args,success=True):
 cmd=['bin/mdhop',*args,'--vault',str(vault),'--db',str(db)]
 if args[0]!='build':cmd+=['--format','json']
 p=subprocess.run(cmd,capture_output=True,text=True)
 print(json.dumps({'command':cmd,'exit':p.returncode,'stdout':p.stdout,'stderr':p.stderr},ensure_ascii=False))
 assert (p.returncode==0)==success
 if not success:
  assert not p.stdout and 'ambiguous normalized disk path' in p.stderr
 else:
  assert not p.stderr
  return json.loads(p.stdout) if args[0]!='build' else None

for mode in ['to','template','dry-run','collision']:
 root=pathlib.Path(tempfile.mkdtemp(prefix='mdhop-a1-02-'));vault=root/'vault';vault.mkdir();db=root/'index.sqlite';actual=vault/'Cafe\u0301';actual.mkdir()
 for name in ['A.md','B.md']:(actual/name).write_text('# '+name+'\n')
 (vault/'Index.md').write_text('[a](Café/A.md)\n[b](Café/B.md)\n')
 entries=os.listdir(vault);print('entries',[(n,list(n.encode())) for n in entries]);assert 'Cafe\u0301' in entries
 try:os.stat(vault/'Café');stat_missing=False
 except FileNotFoundError:stat_missing=True
 print('NFC direct Stat missing',stat_missing)
 if platform.system()=='Linux':assert stat_missing
 run(vault,db,['build'])
 if mode=='collision':
  try:(vault/'Café').mkdir()
  except FileExistsError:pass
  entries=os.listdir(vault);count=sum(n in ['Café','Cafe\u0301'] for n in entries)
  print('collision entries',[(n,list(n.encode())) for n in entries])
  if count!=2:
   assert platform.system()!='Linux';print('SKIP collision: filesystem aliases NFC/NFD; count=',count);continue
  before=snapshot(vault,db)
  for opts in [['--to','Archive'],['--to-template','Archive/{basename}'],['--to-template','Archive/{basename}','--dry-run']]:
   run(vault,db,['move','--from','Café',*opts],False);assert snapshot(vault,db)==before
  print('PASS collision: empty stdout, nonzero exit, disk/DB unchanged');continue
 before=snapshot(vault,db)
 opts=['--to','Archive'] if mode=='to' else ['--to-template','Archive/{basename}']
 if mode=='dry-run':opts+=['--dry-run']
 got=run(vault,db,['move','--from','Café',*opts]);assert got['moved']==[{'from':'Café/A.md','to':'Archive/A.md'},{'from':'Café/B.md','to':'Archive/B.md'}];assert 'from' not in got
 if mode=='dry-run':assert snapshot(vault,db)==before
 else:
  assert not (actual/'A.md').exists() and not (actual/'B.md').exists()
  assert (vault/'Archive/A.md').exists() and (vault/'Archive/B.md').exists()
  assert (vault/'Index.md').read_text()=='[a](/Archive/A.md)\n[b](/Archive/B.md)\n'
  conn=sqlite3.connect(db);nodes=conn.execute('select path,node_key from nodes where type="note"').fetchall();conn.close()
  for name in ['A.md','B.md']:assert ('Archive/'+name,'note:path:Archive/'+name) in nodes
  assert not any(n[0].startswith('Café/') for n in nodes)
 print('PASS real CLI',mode)
