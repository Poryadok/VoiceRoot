"""Build the fixed, reviewed Space root launcher from authenticated Git blobs."""
import sys

import realtime_p3_create_only_source as source
import realtime_p3_root_bundle as existing
import space_p3_issuer_source as contract


def build_git(repo,commit,output):
    contract.verify(repo,commit)
    contents={name:source._blob(repo,commit,'scripts/dev/'+name) for name in contract.ISSUER_FILES}
    path='scripts/dev/realtime_p3_root_launch.template.sh'
    template=source._blob(repo,commit,path)
    if template != source._blob(repo,contract.ISSUER_SHA,path) or source._disk(repo,path)!=template: raise ValueError()
    template=template.decode('utf-8')
    old="FILES = {'credentials.py', 'realtime_p3_missing_issuer.py', 'realtime_p3_root_export.py', 'reset_credentials.py', 'capture-manifest.json'}"
    new='FILES = set('+repr(sorted(set(contract.ISSUER_FILES)|{'capture-manifest.json'}))+')'
    replacements={old:new, '/home/pmd/voice-realtime-p3/p3-source.tar':'/home/pmd/voice-space-p3/p3-source.tar',
                  "'p3-source-' + uuid.uuid4().hex":"'space-p3-source-' + uuid.uuid4().hex",
                  "module = 'realtime_p3_missing_issuer'":"module = 'space_p3_issuer'",
                  "module, arguments = 'realtime_p3_root_export', arguments[1:]":"module, arguments = 'space_p3_issuer_export', arguments[1:]"}
    for before,after in replacements.items():
        if template.count(before)!=1: raise ValueError()
        template=template.replace(before,after)
    return existing._build(contents,template,output)


if __name__ == '__main__':
    if len(sys.argv)!=4: raise SystemExit('usage: space_p3_issuer_bundle.py GIT_REPOSITORY FULL_COMMIT OUTPUT_DIR')
    print(build_git(sys.argv[1],sys.argv[2],sys.argv[3]))
