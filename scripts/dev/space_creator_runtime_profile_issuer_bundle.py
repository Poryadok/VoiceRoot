"""Produce the fixed reviewed Creator root bundle; never invoke it here."""
import sys

import realtime_p3_create_only_source as source
import realtime_p3_root_bundle as existing
import space_creator_runtime_profile_issuer_source as contract

def build_git(repo,commit,output):
    contract.verify(repo,commit)
    contents={name:source._blob(repo,commit,'scripts/dev/'+name) for name in contract.FILES}
    template=source._blob(repo,commit,contract.TEMPLATE).decode('utf8')
    old="FILES = {'credentials.py', 'realtime_p3_missing_issuer.py', 'realtime_p3_root_export.py', 'reset_credentials.py', 'capture-manifest.json'}"
    replacements={
        '#!/usr/bin/env bash':'#!/bin/bash',
        old:'FILES = set('+repr(sorted(set(contract.FILES)|{'capture-manifest.json'}))+')',
        '/home/pmd/voice-realtime-p3/p3-source.tar':'/home/pmd/voice-space-creator/creator-source.tar',
        "'p3-source-' + uuid.uuid4().hex":"'space-creator-source-' + uuid.uuid4().hex",
        "module = 'realtime_p3_missing_issuer'":"module = 'space_creator_runtime_profile_issuer'",
        "module, arguments = 'realtime_p3_root_export', arguments[1:]":"module, arguments = 'space_creator_runtime_profile_issuer_export', arguments[1:]",
        'arguments = sys.argv[1:]':"arguments = sys.argv[1:]\n    if arguments.count('--source-sha') != 1 or arguments.index('--source-sha') + 1 >= len(arguments) or arguments[arguments.index('--source-sha') + 1] != "+repr(commit)+": raise ValueError()",
        "{'PATH': '/usr/local/bin:/usr/bin:/bin', 'HOME': '/root'}":"{'PATH': '/usr/bin:/bin', 'LANG': 'C', 'LC_ALL': 'C'}",
    }
    for before,after in replacements.items():
        if template.count(before)!=1: raise ValueError('Creator root launcher contract changed')
        template=template.replace(before,after)
    return existing._build(contents,template,output)

if __name__=='__main__':
    if len(sys.argv)!=4: raise SystemExit('usage: creator issuer bundle REPOSITORY FULL_COMMIT OUTPUT_DIR')
    print(build_git(sys.argv[1],sys.argv[2],sys.argv[3]))
