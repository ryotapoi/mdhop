# CHANGE-VERIFY: Temporary CI verification for A3-02; not a permanent test.
import pathlib, tempfile, subprocess, json, sqlite3, os, platform
root=pathlib.Path(tempfile.mkdtemp(prefix='mdhop-a3-02-')); vault=root/'vault';outside=root/'outside';vault.mkdir();outside.mkdir();db=root/'index.sqlite'
def run(args, success=True):
 cmd=['bin/mdhop',*args,'--vault',str(vault),'--db',str(db)]
 if args[0]!='build':cmd+=['--format','json']
 p=subprocess.run(cmd,capture_output=True,text=True);print(json.dumps({'command':cmd,'exit':p.returncode,'stdout':p.stdout,'stderr':p.stderr},ensure_ascii=False));assert (p.returncode==0)==success
(vault/'Target.md').write_text('# Target\n');run(['build'])
(outside/'Re\u0301sume\u0301').mkdir();(outside/'Re\u0301sume\u0301'/'A\u0301.md').write_text('[target](../../Target.md)\n');(vault/'Cafe\u0301').symlink_to(outside,target_is_directory=True)
run(['add','--file','Café/Résumé/Á.md']);before=db.read_bytes();run(['add','--file','../escape.md'],False);assert db.read_bytes()==before
run(['move','--from','Café/Résumé/Á.md','--to','Café/Résumé/new/Á.md']);assert not (outside/'Re\u0301sume\u0301'/'A\u0301.md').exists();assert (outside/'Re\u0301sume\u0301'/'new'/'Á.md').read_text()=='[target](../../../Target.md)\n'
conn=sqlite3.connect(db);nodes=conn.execute('select path,node_key from nodes where type="note"').fetchall();conn.close();assert ('Café/Résumé/new/Á.md','note:path:Café/Résumé/new/Á.md') in nodes
print('CLI normal success: NFC DB keys, actual NFD entries, rewritten content',nodes,os.listdir(outside))
if platform.system()=='Linux':
 (vault/'Café').mkdir();entries=os.listdir(vault);assert 'Café' in entries and 'Cafe\u0301' in entries;print('collision entry bytes',[(name,list(name.encode())) for name in entries])
 before=db.read_bytes();body=(outside/'Re\u0301sume\u0301'/'new'/'Á.md').read_bytes()
 run(['add','--file','Café/Résumé/Fresh.md'],False)
 run(['move','--from','Target.md','--to','Café/Résumé/Target.md'],False)
 assert before==db.read_bytes();assert body==(outside/'Re\u0301sume\u0301'/'new'/'Á.md').read_bytes();assert (vault/'Target.md').read_text()=='# Target\n';assert not (outside/'Re\u0301sume\u0301'/'Target.md').exists();print('Linux CLI collision rejection: disk and DB unchanged')
else:print('macOS CLI collision omitted: permanent regression records normalization alias')
