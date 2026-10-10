"""Real CLOSED consumer decoding under captured current-code custody."""
import base64
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import preserve
from native_store import archive_closed_store


class CurrentDecoderTests(unittest.TestCase):
    def setUp(self):
        supplied=os.environ.get('VOICE_TEST_NATIVE_KERNEL')
        self.temp=tempfile.TemporaryDirectory(dir=str(Path(supplied).parent) if supplied else None)
        self.addCleanup(self.temp.cleanup)
        self.base=Path(self.temp.name);self.base.chmod(0o700)
        self.code=self.base/'code';self.code.mkdir(mode=0o700)
        self.module=self.code/'nats-rollout-preservation';self.module.mkdir(mode=0o700)
        helper=self.module/'preserve.py';shutil.copyfile(preserve.__file__,helper);helper.chmod(0o400)
        folder=self.code/'nats-known-baseline';folder.mkdir(mode=0o700)
        self.kernel=folder/'kernel'
        if supplied:
            shutil.copyfile(supplied,self.kernel)
        else:
            # CI already supplies Go. Compile the actual decoder source with a
            # tiny entrypoint, without dependencies, broker or network access.
            source=Path(preserve.__file__).resolve().parents[1]/'nats-known-baseline/native_consumer_state.go'
            shutil.copyfile(source,folder/'native_consumer_state.go')
            (folder/'main.go').write_text('package main\nimport("os";"fmt")\nfunc main(){if len(os.Args)<2||os.Args[1]!="--consumer-state"{os.Exit(2)};if err:=nativeConsumerState(os.Args[2:],os.Stdin,os.Stdout);err!=nil{fmt.Fprintln(os.Stderr,err);os.Exit(1)}}\n')
            subprocess.run(['go','build','-o',str(self.kernel),str(folder/'main.go'),str(folder/'native_consumer_state.go')],check=True,timeout=90,env={**os.environ,'GO111MODULE':'off','GOTOOLCHAIN':'local','GOPROXY':'off'})
        self.kernel.chmod(0o500)
        self.binding={'nats-rollout-preservation/preserve.py':hashlib.sha256(helper.read_bytes()).hexdigest(),
            'nats-known-baseline/kernel':hashlib.sha256(self.kernel.read_bytes()).hexdigest()}
        (self.code/'capture-manifest.json').write_text(json.dumps(self.binding));(self.code/'capture-manifest.json').chmod(0o400)
        self.original=b'#!/bin/sh\necho historical_kernel_isolation_invalid >&2\nexit 1\n'
        (self.base/'kernel').write_bytes(self.original);(self.base/'kernel').chmod(0o500)
        data=self.base/'data';data.mkdir(mode=0o700)
        self.path='jetstream/ACCOUNT/streams/BASE/obs/durable/o.dat'
        consumer=data/Path(self.path).parent;consumer.mkdir(parents=True)
        for leaf,raw in [('o.dat',base64.b64decode('FgIAAAAAAAA=')),('meta.inf',b'{}'),('meta.sum',b'0123456789abcdef')]:
            (consumer/leaf).write_bytes(raw)
        self.archive=self.base/'cut.tar';self.manifest=archive_closed_store(data,self.archive)
        self.row={'account':'ACCOUNT','consumers':[{'stream':'BASE','name':'durable'}]}

    def decode(self):
        with patch.object(preserve,'__file__',str(self.module/'preserve.py')):
            return preserve.closed_native_states(type('Runtime',(),{'base':self.base})(),
                self.archive,self.manifest,self.row,lambda:None)

    def test_historical_kernel_is_untouched_and_current_decoder_is_actually_executed(self):
        result=self.decode()[self.path]
        self.assertEqual(result['schema'],'nats-closed-durable-consumer-v1')
        self.assertEqual(result['pending'],{});self.assertEqual(result['redelivered'],{})
        self.assertEqual((self.base/'kernel').read_bytes(),self.original)

    def test_changed_or_writable_captured_decoder_is_refused_without_fallback(self):
        for defect in ('hash','writable'):
            with self.subTest(defect=defect):
                saved=self.kernel.read_bytes();self.kernel.chmod(0o700)
                if defect=='hash':self.kernel.write_bytes(self.original)
                else:self.kernel.chmod(0o520)
                with self.assertRaises(preserve.Blocked):self.decode()
                self.kernel.chmod(0o700);self.kernel.write_bytes(saved);self.kernel.chmod(0o500)
