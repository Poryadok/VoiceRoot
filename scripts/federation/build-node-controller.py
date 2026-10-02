"""Build only the node controller's verified library/command dependency closure."""
import hashlib,json,pathlib,shutil,subprocess,tempfile
root=pathlib.Path(__file__).resolve().parents[2]
working=root/'tmp/node-authority-20261002';working.mkdir(parents=True,exist_ok=True)
context=pathlib.Path(tempfile.mkdtemp(prefix='build-',dir=working))
for module in ['federation','pkg']:
    target=context/'src/backend'/module;target.mkdir(parents=True)
    for name in ['go.mod','go.sum']:shutil.copy2(root/'src/backend'/module/name,target/name)
for package in ['protocol','nodecache','mediaauthority','nodepublisher','cmd/node-authority']:
    target=context/'src/backend/federation'/package;target.mkdir(parents=True,exist_ok=True)
    for source in (root/'src/backend/federation'/package).glob('*.go'):shutil.copy2(source,target/source.name)
shutil.copy2(root/'docker/voice-node/authority/Dockerfile',context/'Dockerfile')
hasher=hashlib.sha256()
for path in sorted(context.rglob('*')):
    if path.is_file():hasher.update(path.relative_to(context).as_posix().encode()+b'\0'+path.read_bytes())
digest=hasher.hexdigest()
record={'context':str(context),'input_sha256':digest,'image':'voice-node-authority:game-sprint'}
log=working/'build.log'
with log.open('w',encoding='utf-8') as output:
    result=subprocess.run(['rtk','proxy','docker','build','--label','voice.input.sha256='+digest,'-t',record['image'],str(context)],cwd=root,stdout=output,stderr=subprocess.STDOUT)
record['exit']=result.returncode
if not result.returncode:
    image=subprocess.check_output(['rtk','proxy','docker','image','inspect','--format','{{.Id}}',record['image']],cwd=root,text=True).strip()
    record['image_id']=image
(working/'build-result.json').write_text(json.dumps(record,indent=2))
print(json.dumps(record))
raise SystemExit(result.returncode)
