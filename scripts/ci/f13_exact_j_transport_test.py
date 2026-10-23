"""Real fixture materialization regression; never a F13 guard/producer RED."""
import hashlib
import contextlib
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest import mock
import zipfile

import f13_exact_j_historical_red_test as harness


class SuccessorReplayBinding(unittest.TestCase):
    def test_loader_imports_distinct_source_bound_successor_with_historical_root(self):
        import importlib.util
        helper = harness.ROOT/'scripts/ci/f13_f54_history.py'
        spec = importlib.util.spec_from_file_location('private_untouched_historical', helper)
        historical = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(historical)
        before = dict(vars(historical))
        with tempfile.TemporaryDirectory() as folder:
            repo = Path(folder).resolve()
            def git(*args):
                return subprocess.check_output(['git', '-C', str(repo), *args]).decode().strip()
            git('init', '-q')
            git('config', 'user.name', 'Private replay fixture')
            git('config', 'user.email', 'private@example.invalid')
            for name in harness.OUTER_SOURCE_HASHES:
                path = repo/name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(('private historical '+name+'\n').encode())
            git('add', '.')
            git('commit', '-qm', 'private historical carrier')
            source = git('rev-parse', 'HEAD')
            hashes = {name:harness.sha((repo/name).read_bytes()) for name in harness.OUTER_SOURCE_HASHES}
            identity = dict(commit='1'*40, files={'scripts/ci/f13_f54_history.py':harness.sha(helper.read_bytes())})
            with mock.patch.multiple(harness, SOURCE=source, OUTER_SOURCE_HASHES=hashes):
                successor, provenance = harness.successor_replay_module(repo, identity)
                again, other = harness.successor_replay_module(repo, identity)
                self.assertIsNot(successor, again)
                self.assertEqual(provenance, other)
                self.assertEqual(successor.ROOT, repo)
                self.assertEqual(provenance['historical_source'], source)
                self.assertEqual(provenance['harness_commit'], identity['commit'])
                self.assertEqual(provenance['executable_sha256'], harness.sha(helper.read_bytes()))
                wrong = dict(identity, files={'scripts/ci/f13_f54_history.py':'0'*64})
                with self.assertRaises(harness.SetupFailure): harness.successor_replay_module(repo, wrong)
        self.assertEqual(vars(historical), before)




class OriginalGuardContextContract(unittest.TestCase):
    def fixture(self):
        commit = harness.OPERATION.split('/')[1]
        operation = dict(commit=commit, classification='merge', first_parent='a' * 40,
                         boundary='b' * 40, source_head='c' * 40)
        incoming = [harness.PATH, 'true', commit, operation['first_parent'],
                    operation['boundary'], operation['source_head'], 'd' * 40, operation['boundary']]
        # A qualification child has its own owner/source/trigger. An
        # unpartitioned parent never imports this row into nested_map.
        nested = [harness.PATH, 'false', 'e' * 40, 'f' * 40, '1' * 40, '2' * 40, '3' * 40, '1' * 40]
        return dict(partition_source_history='false'), operation, incoming, nested

    def test_unpartitioned_merge_uses_original_operation_not_qualification_child(self):
        frame, op, incoming, nested = self.fixture()
        self.assertEqual(harness.original_guard_context(frame, op, incoming, nested),
                         dict(nested_owner_commit=op['commit'], nested_base=op['boundary'],
                              nested_head=op['source_head'], guarded_entry_commit=op['commit'], trigger='true'))

    def test_unexpected_partition_or_incoming_binding_is_rejected(self):
        for field in ('partition', 'commit', 'classification', 'path', 'trigger', 'parent', 'base', 'head'):
            with self.subTest(field=field):
                frame, op, incoming, nested = self.fixture()
                if field == 'partition': frame['partition_source_history'] = 'true'
                elif field == 'commit': op['commit'] = '4' * 40
                elif field == 'classification': op['classification'] = 'direct'
                else: incoming[{'path': 0, 'trigger': 1, 'parent': 3, 'base': 4, 'head': 5}[field]] = 'wrong'
                with self.assertRaises(harness.SetupFailure):
                    harness.original_guard_context(frame, op, incoming, nested)


class PrivateEntropyContract(unittest.TestCase):
    """Fixed endpoint component tests; real private mount/DAC remains hosted."""
    def test_namespace_constructs_private_sealed_entropy_endpoint_before_preparation(self):
        _, _, spawn, _ = SameFinalViewContract().capture_namespace()
        script = spawn.call_args.args[0][-1]
        self.assertIn('/usr/bin/mount -t tmpfs -o size=64k,nr_inodes=8,mode=0555,uid=0,gid=0,nosuid,noexec,dev f13-private-dev ',script)
        self.assertIn('/usr/bin/mknod -m 0444 ',script)
        self.assertIn(' c 1 9\n',script)
        self.assertIn('/usr/bin/chown 0:0 ',script)
        tmpfs = script.index('/usr/bin/mount -t tmpfs ')
        node = script.index('/usr/bin/mknod ')
        null = script.index('mount --bind /dev/null ')
        seal = script.index('/usr/bin/mount -o remount,ro,nosuid,noexec,dev ')
        prepare = script.index(' --prepare\n')
        self.assertLess(tmpfs,node)
        self.assertLess(node,null)
        self.assertLess(null,seal)
        self.assertLess(seal,prepare)
        self.assertNotIn('mount --bind /dev/urandom',script)
        self.assertNotIn('/dev/random',script)


    @contextlib.contextmanager
    def device_fixture(self):
        import errno
        import stat
        import types
        fixture = types.SimpleNamespace()
        fixture.mount = b'44 1 0:55 / /dev ro,nosuid,noexec - tmpfs f13-private-dev ro,size=64k,nr_inodes=8\n'
        fixture.status = b'CapEff:\t0000000000000000\nCapPrm:\t0\nCapInh:\t0\nCapAmb:\t0\n'
        def metadata(mode, rdev=0):
            return types.SimpleNamespace(st_mode=mode,st_rdev=rdev,st_uid=0,st_gid=0,st_dev=55,st_ino=rdev+1)
        fixture.nodes = {'/dev':metadata(stat.S_IFDIR|0o555),
                         '/dev/urandom':metadata(stat.S_IFCHR|0o444,os.makedev(1,9)),
                         '/dev/null':metadata(stat.S_IFCHR|0o666,os.makedev(1,3))}
        fixture.entries = ['null','urandom']
        class Entries:
            def __enter__(self): return iter(types.SimpleNamespace(name=n) for n in fixture.entries)
            def __exit__(self,*_): return False
        def metadata_stream(path,*args,**kwargs):
            raw = fixture.mount if str(path)=='/proc/self/mountinfo' else fixture.status
            return io.BytesIO(raw)
        def open_device(path,flags):
            if path=='/dev/null': return 42
            if flags & os.O_ACCMODE == os.O_RDONLY: return 41
            raise PermissionError(errno.EACCES,'PRIVATE_SECRET')
        with contextlib.ExitStack() as stack:
            stack.enter_context(mock.patch.object(harness,'ROOT',Path('/harness')))
            for name in ('getuid','geteuid','getgid','getegid'):
                stack.enter_context(mock.patch.object(harness.os,name,return_value=1000))
            stack.enter_context(mock.patch.object(Path,'open',autospec=True,side_effect=metadata_stream))
            stack.enter_context(mock.patch.object(harness.os,'statvfs',return_value=types.SimpleNamespace(
                f_flag=os.ST_RDONLY|os.ST_NOSUID|os.ST_NOEXEC,f_blocks=16,f_frsize=4096,f_files=8)))
            stack.enter_context(mock.patch.object(harness.os,'lstat',side_effect=lambda path:fixture.nodes[path]))
            stack.enter_context(mock.patch.object(harness.os,'scandir',side_effect=lambda _:Entries()))
            fixture.acl=stack.enter_context(mock.patch.object(harness.os,'getxattr',side_effect=OSError(errno.ENODATA,'PRIVATE_SECRET')))
            fixture.open=stack.enter_context(mock.patch.object(harness.os,'open',side_effect=open_device))
            fixture.fstat=stack.enter_context(mock.patch.object(harness.os,'fstat',side_effect=lambda fd:fixture.nodes['/dev/urandom' if fd==41 else '/dev/null']))
            fixture.read=stack.enter_context(mock.patch.object(harness.os,'read',side_effect=lambda fd,n:b'x'*32 if fd==41 else b''))
            fixture.write=stack.enter_context(mock.patch.object(harness.os,'write',return_value=0))
            fixture.close=stack.enter_context(mock.patch.object(harness.os,'close'))
            yield fixture

    def test_bounded_device_verifier_closes_fds_and_never_writes_entropy(self):
        out, err = io.StringIO(), io.StringIO()
        with self.device_fixture() as f, contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            self.assertIsNone(harness.verify_private_devices())
            self.assertEqual(f.close.call_args_list,[mock.call(41),mock.call(42)])
            self.assertEqual(f.write.call_args_list,[mock.call(42,b'')])
            self.assertEqual(f.open.call_count,4)
            for call in f.open.call_args_list:
                self.assertTrue(call.args[1] & os.O_NOFOLLOW)
                self.assertTrue(call.args[1] & os.O_CLOEXEC)
            self.assertEqual([c.args[1] & os.O_ACCMODE for c in f.open.call_args_list],
                             [os.O_RDONLY,os.O_WRONLY,os.O_RDWR,os.O_RDWR])
        self.assertEqual((out.getvalue(),err.getvalue()),('',''))

    def test_mount_metadata_shape_bounds_and_privilege_reject_before_device_open(self):
        bad_mounts=[b'',b'x\n',b'x'*131073,
                    b'44 1 0:55 / /dev rw,nosuid,noexec - tmpfs f13-private-dev ro\n',
                    b'44 1 0:55 / /dev ro,nosuid,noexec,nodev - tmpfs f13-private-dev ro\n',
                    b'44 1 0:55 / /dev ro,nosuid - tmpfs f13-private-dev ro\n',
                    b'44 1 0:55 / /dev ro,nosuid,noexec - tmpfs f13-private-dev rw\n',
                    b'44 1 0:55 / /dev ro,nosuid,noexec - ext4 f13-private-dev ro\n',
                    b'44 1 0:55 / /other/dev ro,nosuid,noexec - tmpfs f13-private-dev ro\n']
        with self.device_fixture() as f:
            bad_mounts.append(f.mount+f.mount)
        for raw in bad_mounts:
            with self.subTest(mount=raw[:60]), self.device_fixture() as f:
                f.mount=raw
                with self.assertRaises(harness.SetupFailure):harness.verify_private_devices()
                f.open.assert_not_called()
        for raw in (b'CapEff:1\nCapPrm:0\nCapInh:0\nCapAmb:0\n',b'CapEff:0\n',
                    b'CapEff:0\nCapPrm:0\nCapInh:0\nCapAmb:0\nCapEff:0\n',b'Z'*16385):
            with self.device_fixture() as f:
                f.status=raw
                with self.assertRaises(harness.SetupFailure):harness.verify_private_devices()
                f.open.assert_not_called()

    def test_node_directory_acl_and_entry_rejections_are_fail_closed(self):
        import errno
        import stat
        cases=[('/dev','st_uid',1000),('/dev','st_mode',stat.S_IFDIR|0o755),
               ('/dev/urandom','st_mode',stat.S_IFREG|0o444),('/dev/urandom','st_mode',stat.S_IFLNK|0o777),
               ('/dev/urandom','st_mode',stat.S_IFCHR|0o666),('/dev/urandom','st_rdev',os.makedev(1,8)),
               ('/dev/urandom','st_uid',1000),('/dev/urandom','st_gid',1000),
               ('/dev/null','st_rdev',os.makedev(1,9))]
        for path,field,value in cases:
            with self.subTest(path=path,field=field),self.device_fixture() as f:
                setattr(f.nodes[path],field,value)
                with self.assertRaises(harness.SetupFailure):harness.verify_private_devices()
                f.open.assert_not_called()
        for entries in (['null'],['null','urandom','unexpected']):
            with self.device_fixture() as f:
                f.entries=entries
                with self.assertRaises(harness.SetupFailure):harness.verify_private_devices()
                f.open.assert_not_called()
        for failure in (b'ACL',OSError(errno.EACCES,'PRIVATE_SECRET')):
            with self.device_fixture() as f:
                f.acl.side_effect=None if isinstance(failure,bytes) else failure
                f.acl.return_value=failure
                with self.assertRaises(harness.SetupFailure) as caught:harness.verify_private_devices()
                self.assertNotIn('PRIVATE_SECRET',str(caught.exception))
                f.open.assert_not_called()

    def test_read_identity_failure_and_writable_open_close_without_entropy_write(self):
        import errno
        import types
        for kind in ('identity','short','read_error','writable','wrong_denial'):
            with self.subTest(kind=kind),self.device_fixture() as f:
                if kind=='identity':
                    data=vars(f.nodes['/dev/urandom']).copy();data['st_ino']+=1
                    f.fstat.side_effect=None;f.fstat.return_value=types.SimpleNamespace(**data)
                elif kind=='short':f.read.side_effect=None;f.read.return_value=b'x'
                elif kind=='read_error':f.read.side_effect=OSError(errno.EIO,'PRIVATE_SECRET')
                elif kind=='writable':f.open.side_effect=[41,73]
                elif kind=='wrong_denial':f.open.side_effect=[41,OSError(errno.EMFILE,'PRIVATE_SECRET')]
                with self.assertRaises(harness.SetupFailure) as caught:harness.verify_private_devices()
                self.assertNotIn('PRIVATE_SECRET',str(caught.exception))
                f.write.assert_not_called()
                self.assertEqual(f.close.call_args_list,[mock.call(41),mock.call(73)] if kind=='writable' else [mock.call(41)])

    def test_dispatch_verifies_before_each_ordinary_entry_and_failure_stops_both(self):
        events=[]
        with mock.patch.object(harness,'verify_private_devices',side_effect=lambda:events.append('verify')) as verify, \
             mock.patch.object(harness,'prepare',side_effect=lambda:events.append('prepare')) as prepare, \
             mock.patch.object(harness,'inside',side_effect=lambda:events.append('inside')) as inside:
            for mode in ('--prepare','--inside'):
                with mock.patch.object(harness.sys,'argv',['driver',mode]):harness.main()
            self.assertEqual(events,['verify','prepare','verify','inside'])
            prepare.reset_mock();inside.reset_mock()
            verify.side_effect=harness.SetupFailure('fixed_device_failure')
            for mode in ('--prepare','--inside'):
                with mock.patch.object(harness.sys,'argv',['driver',mode]),self.assertRaisesRegex(harness.SetupFailure,'fixed_device_failure'):
                    harness.main()
            prepare.assert_not_called();inside.assert_not_called()

    def test_existing_unsealed_device_namespace_rejects_without_opening_entropy(self):
        with mock.patch.object(harness,'ROOT',Path('/harness')),mock.patch.object(harness.os,'open') as opened:
            with self.assertRaises(harness.SetupFailure):harness.verify_private_devices()
            opened.assert_not_called()


    def test_private_device_setup_failures_stop_before_preparation_or_inside(self):
        _,_,spawn,_=SameFinalViewContract().capture_namespace()
        script=spawn.call_args.args[0][-1].replace('exec chroot ','chroot ')
        script=script.replace('/usr/bin/mount ','mount ').replace('/usr/bin/mknod ','mknod ').replace('/usr/bin/chown ','chown ')
        script='\n'.join(':' if line.startswith(': > ') else line for line in script.splitlines())+'\n'
        prefix='set -eu\n'
        prefix+='mount() { if [ "$FAILURE" = tmpfs ] && [ "$1" = -t ] && [ "$2" = tmpfs ]; then return 40; fi; '
        prefix+='if [ "$FAILURE" = null ] && [ "$1" = --bind ] && [ "$2" = /dev/null ]; then return 40; fi; '
        prefix+='if [ "$FAILURE" = seal ] && [ "$1" = -o ] && [ "${3##*/}" = dev ]; then return 40; fi; return 0; }\n'
        prefix+='mknod() { if [ "$FAILURE" = node ]; then return 40; fi; return 0; }\n'
        prefix+='chown() { if [ "$FAILURE" = owner ]; then return 40; fi; return 0; }\n'
        prefix+='chroot() { printf "UNEXPECTED_ORDINARY_ENTRY\\n"; return 0; }\n'
        for failure in ('tmpfs','node','owner','null','seal'):
            with self.subTest(failure=failure):
                result=subprocess.run(['bash','-c',prefix+script],capture_output=True,env=dict(os.environ,FAILURE=failure))
                self.assertEqual(result.returncode,40)
                self.assertNotIn(b'UNEXPECTED_ORDINARY_ENTRY',result.stdout)


class SameFinalViewContract(unittest.TestCase):
    """Preparation component proof; real namespace and actual guard remain separate."""
    def capture_namespace(self, result=None):
        import types
        outputs = (io.BytesIO(), io.BytesIO())
        def command(args):
            if args[-2:] == ['rev-parse', 'HEAD']:
                return b'a' * 40 + b'\n'
            return (harness.ROOT / args[-1].split(':', 1)[1]).read_bytes()
        def old_materialize(root):
            (root / 'historical-git').mkdir()
        guard = b'{"kind":"FAITHFUL_HISTORICAL_SHARED_GUARD","guard_observed":1,"contract_expected":0}\n'
        with mock.patch.object(harness.os, 'geteuid', return_value=1000), \
             mock.patch.object(harness.shutil, 'which', return_value='/trusted/tool'), \
             mock.patch.object(harness, 'run', side_effect=command), \
             mock.patch.object(harness, 'materialize', side_effect=old_materialize) as outer, \
             mock.patch.object(harness, 'prepare_public_diagnostic_context', return_value=None), \
             mock.patch.object(harness.subprocess, 'run', return_value=result or subprocess.CompletedProcess([], 1, guard, b'')) as spawn, \
             mock.patch.object(harness.sys, 'stdout', types.SimpleNamespace(buffer=outputs[0])), \
             mock.patch.object(harness.sys, 'stderr', types.SimpleNamespace(buffer=outputs[1])):
            status = harness.namespace_main()
        return status, outer, spawn, outputs

    def test_preparation_runs_in_final_view_before_seal_without_outer_materialization(self):
        status, outer, spawn, _ = self.capture_namespace()
        self.assertEqual(status, 1)
        outer.assert_not_called()
        self.assertEqual(spawn.call_count, 1)
        self.assertEqual(spawn.call_args.kwargs, {'capture_output': True})
        argv = spawn.call_args.args[0]
        self.assertEqual(argv[:-1], ['sudo', '-n', 'unshare', '--mount', '--net', '--pid', '--fork', 'bash', '-c'])
        script = argv[-1]
        self.assertEqual(script.count(' --prepare\n'), 1)
        self.assertEqual(script.count(' --inside\n'), 1)
        prepare = script.index(' --prepare\n')
        seal = script.index('mount --bind ', prepare)
        inside = script.index(' --inside\n')
        self.assertLess(prepare, seal)
        self.assertLess(seal, inside)
        self.assertEqual(script.count('chroot --userspec='), 2)
        self.assertEqual(script.count(' /usr/bin/env -i PATH=/historical-git/launch:/historical-git/util '), 2)
        self.assertLess(script.index('mount -t proc proc '), prepare)
        self.assertLess(script.index('mount -o remount,bind,ro '), prepare)
        self.assertIn('test -d ', script[prepare:seal])
        self.assertIn('test ! -L ', script[prepare:seal])
        self.assertIn('exec chroot --userspec=', script[seal:])


    def test_prepare_validates_ordinary_identity_and_exact_source_before_materializing(self):
        prepare = getattr(harness, 'prepare', None)
        self.assertTrue(callable(prepare), 'missing selected source-bound prepare mode')
        identity = dict(commit='a' * 40, files={name:harness.sha(b'fixed source') for name in harness.HARNESS_SOURCE_PATHS})
        with mock.patch.object(harness, 'ROOT', Path('/harness')), \
             mock.patch.object(harness.os, 'geteuid', return_value=1000), \
             mock.patch.dict(os.environ, {'F13_HARNESS_IDENTITY':json.dumps(identity)}), \
             mock.patch.object(Path, 'read_bytes', return_value=b'fixed source'), \
             mock.patch.object(harness, 'materialize') as materialize, \
             mock.patch.object(harness.subprocess, 'run') as child:
            self.assertEqual(prepare(), 0)
            materialize.assert_called_once_with(Path('/'))
            child.assert_not_called()
            materialize.reset_mock()
            for invalid in ({'commit':'bad','files':identity['files']},
                            {'commit':identity['commit'],'files':{}},
                            {'commit':identity['commit'],'files':{**identity['files'],next(iter(identity['files'])):'0'*64}}):
                with mock.patch.dict(os.environ, {'F13_HARNESS_IDENTITY':json.dumps(invalid)}), self.assertRaises(harness.SetupFailure):
                    prepare()
                materialize.assert_not_called()
            with mock.patch.object(harness.os, 'geteuid', return_value=0), self.assertRaisesRegex(harness.SetupFailure, 'ordinary_runner_uid_required'):
                prepare()
            materialize.assert_not_called()
            with mock.patch.object(harness, 'ROOT', Path('/wrong')), self.assertRaisesRegex(harness.SetupFailure, 'private_namespace_required'):
                prepare()
            materialize.assert_not_called()
            materialize.side_effect = harness.SetupFailure('fixed_failure')
            with self.assertRaisesRegex(harness.SetupFailure, '^fixed_failure$'):
                prepare()

    def test_dispatch_is_exact_and_does_not_fall_back_to_namespace_for_unknown_modes(self):
        main = getattr(harness, 'main', None)
        self.assertTrue(callable(main))
        with mock.patch.object(harness, 'verify_private_devices'), \
             mock.patch.object(harness, 'prepare', return_value=0) as prepare, \
             mock.patch.object(harness, 'inside', return_value=1) as inside, \
             mock.patch.object(harness, 'namespace_main', return_value=2) as outer:
            for argv, expected in (([],2), (['--prepare'],0), (['--inside'],1)):
                with mock.patch.object(harness.sys, 'argv', ['driver']+argv):
                    self.assertEqual(main(), expected)
            self.assertEqual((prepare.call_count,inside.call_count,outer.call_count),(1,1,1))
            for argv in (['--unknown'], ['--prepare','extra'], ['--inside','--prepare']):
                with mock.patch.object(harness.sys, 'argv', ['driver']+argv), self.assertRaisesRegex(harness.SetupFailure, 'explicit_harness_mode'):
                    main()
            self.assertEqual((prepare.call_count,inside.call_count,outer.call_count),(1,1,1))

    def test_shell_aborts_before_inside_on_prepare_or_seal_failure(self):
        # Execute the real generated shell with fixed test-only mount/chroot functions.
        # This proves ordering/fail-closed shell semantics, not actual mount privileges.
        _, _, spawn, _ = self.capture_namespace()
        script = spawn.call_args.args[0][-1].replace('exec chroot ', 'chroot ')
        script = script.replace('/usr/bin/mount ', 'mount ').replace('/usr/bin/mknod ', 'mknod ').replace('/usr/bin/chown ', 'chown ')
        script = '\n'.join(':' if line.startswith(': > ') else line for line in script.splitlines()) + '\n'
        for failure in ('none', 'prepare', 'seal'):
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as folder:
                trace = Path(folder)/'trace'
                prefix = 'set -eu\n'
                prefix += 'mount() { printf "mount\\n" >> "$TRACE"; '
                prefix += 'if [ "$FAILURE" = seal ] && [ "$1" = -o ] && [ "${3##*/}" = historical-git ]; then return 42; fi; }\n'
                prefix += 'chroot() { for last do :; done; printf "%s\\n" "$last" >> "$TRACE"; '
                prefix += 'if [ "$last" = --prepare ] && [ "$FAILURE" = prepare ]; then return 41; fi; }\n'
                # Test-only directory predicates accept the sealed private target.
                prefix += 'test() { return 0; }\n'
                prefix += 'mknod() { return 0; }; chown() { return 0; }\n'
                result = subprocess.run(['bash','-c',prefix+script], capture_output=True,
                                        env=dict(os.environ,TRACE=str(trace),FAILURE=failure))
                events = trace.read_text().splitlines()
                self.assertEqual(events.count('--prepare'),1)
                self.assertEqual(events.count('--inside'), 1 if failure=='none' else 0)
                self.assertEqual(result.returncode, {'none':0,'prepare':41,'seal':42}[failure])

    def test_original_map_rejects_stale_or_forged_entries_after_preparation(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)/'root'
            root.mkdir()
            utilities = Path(folder)/'utilities'
            utilities.mkdir()
            referent = Path(folder)/'referent'
            referent.write_bytes(b'executable fixture')
            referent.chmod(0o555)
            (utilities/'fixture_inside').symlink_to(referent)
            with mock.patch.object(harness,'UTILITY_DIRS',(str(utilities),)):
                base = harness.materialize_pinned_git(root)
                harness.verify_pinned_git(root)
                referent.chmod(0o444)
                with contextlib.redirect_stderr(io.StringIO()), self.assertRaisesRegex(harness.SetupFailure,'pinned_Git_private_utility_view'):
                    harness.verify_pinned_git(root)
                referent.chmod(0o555)
                harness.verify_pinned_git(root)
                referent.unlink()
                with contextlib.redirect_stderr(io.StringIO()), self.assertRaisesRegex(harness.SetupFailure,'pinned_Git_private_utility_view'):
                    harness.verify_pinned_git(root)
                referent.write_bytes(b'executable fixture')
                referent.chmod(0o555)
                harness.verify_pinned_git(root)
                (base/'util').chmod(0o755)
                (base/'util'/'forged').write_bytes(b'non-symlink')
                (base/'util').chmod(0o555)
                with contextlib.redirect_stderr(io.StringIO()), self.assertRaisesRegex(harness.SetupFailure,'pinned_Git_private_utility_view'):
                    harness.verify_pinned_git(root)


    def test_complete_original_predicate_uses_stable_final_view_without_outer_only_closure(self):
        # Controlled pathname availability model, not a real mount/chroot experiment.
        with tempfile.TemporaryDirectory() as folder:
            workspace = Path(folder)
            early, late = workspace/'early', workspace/'late'
            early.mkdir()
            late.mkdir()
            inside, outside = workspace/'inside', workspace/'outside'
            inside.write_bytes(b'inside executable')
            outside.write_bytes(b'outside executable')
            inside.chmod(0o555)
            outside.chmod(0o555)
            (early/'fixture_inside').symlink_to(inside)
            (early/'fixture_outer_only').symlink_to(outside)
            (late/'fixture_inside').symlink_to(outside)
            (early/'git').symlink_to(inside)
            (early/'git-private').symlink_to(inside)
            (early/'directory').mkdir()
            (early/'nonexec').write_bytes(b'not executable')
            oldroot, newroot = workspace/'old', workspace/'new'
            oldroot.mkdir()
            newroot.mkdir()
            with mock.patch.object(harness,'UTILITY_DIRS',(str(early),str(late))):
                self.assertEqual(harness.expected_utilities(),
                                 {'fixture_inside':str(early/'fixture_inside'),
                                  'fixture_outer_only':str(early/'fixture_outer_only')})
                oldbase = harness.materialize_pinned_git(oldroot)
                outside.rename(workspace/'outside-hidden')
                final_expected = {'fixture_inside':str(early/'fixture_inside')}
                self.assertEqual(harness.expected_utilities(), final_expected)
                with contextlib.redirect_stderr(io.StringIO()), self.assertRaisesRegex(harness.SetupFailure,'pinned_Git_private_utility_view'):
                    harness.verify_pinned_git(oldroot)
                newbase = harness.materialize_pinned_git(newroot)
                self.assertEqual({p.name:str(p.readlink()) for p in (newbase/'util').iterdir()}, final_expected)
                harness.verify_pinned_git(newroot)
                (workspace/'outside-hidden').rename(outside)
                with contextlib.redirect_stderr(io.StringIO()), self.assertRaisesRegex(harness.SetupFailure,'pinned_Git_private_utility_view'):
                    harness.verify_pinned_git(newroot)
                harness.verify_pinned_git(oldroot)


class PairedResolutionContract(unittest.TestCase):
    """Opaque function proof; no actual hosted cause, guard or ownership claim."""
    def test_real_closure_loss_keeps_rejection_and_pairs_fixed_missing_observation(self):
        self.assertNotEqual(os.geteuid(), 0)
        with tempfile.TemporaryDirectory(prefix='f13-paired-') as folder:
            workspace = Path(folder)
            root = workspace / 'root'
            root.mkdir()
            utilities = workspace / 'fixture-bin'
            utilities.mkdir()
            closure = workspace / 'fixture-closure'
            closure.mkdir()
            program = closure / 'program'
            program.write_text('#!/bin/sh\nexit 0\n')
            program.chmod(0o555)
            for name in ('fixture_alpha', 'fixture_beta'):
                (utilities / name).symlink_to(program)
            identity = dict(commit='a' * 40, files={'driver': 'b' * 64})
            with mock.patch.object(harness, 'UTILITY_DIRS', (str(utilities),)):
                base = harness.materialize_pinned_git(root)
                harness.verify_pinned_git(root)
                actual = {p.name: str(p.readlink()) for p in (base / 'util').iterdir()}
                self.assertEqual(len(actual), 2)
                prepare = getattr(harness, 'prepare_paired_utility_context', lambda *_:None)
                context = prepare(root, identity)
                closure.rename(workspace / 'fixture-hidden')
                original = io.StringIO()
                with contextlib.redirect_stderr(original), self.assertRaisesRegex(harness.SetupFailure, '^pinned_Git_private_utility_view$'):
                    harness.verify_pinned_git(root)
                expected = harness.expected_utilities()
                self.assertEqual(expected, {})
                legacy = json.loads(original.getvalue().splitlines()[0])
                self.assertEqual(legacy['counts']['extra'], 2)
                self.assertEqual(legacy['counts']['changed'], 0)
                for entry in legacy['entries']:
                    self.assertEqual(entry['slot'], dict(is_symlink=True, is_file=False, executable=False))
                observe = getattr(harness, 'paired_utility_observation', lambda *_:None)
                observation = observe(base, actual, expected, 2, tuple(actual), identity)
                encode = getattr(harness, 'encode_paired_utility_observation', lambda _:b'')
                raw = encode(observation) if observation is not None else b''
                resolve = getattr(harness, 'resolve_owned_paired_output', lambda out,err,ctx:(out,err,None))
                legacy_bytes = original.getvalue().encode()
                stdout, stderr, receipt = resolve(b'original stdout\n', raw+legacy_bytes+b'original stderr\n', context)
                self.assertIsNotNone(receipt, 'original mismatch still rejects; desired paired diagnostic is absent on baseline')
                self.assertEqual(stdout, b'original stdout\n')
                self.assertEqual(stderr, legacy_bytes+b'original stderr\n')
                self.assertEqual(receipt['kind'], 'F13_PAIRED_UTILITY_CONTEXT_RECEIPT_v1')
                self.assertTrue(receipt['credentials_equal'])
                self.assertEqual(len(receipt['entries']), 2)
                for entry in receipt['entries']:
                    self.assertEqual(entry['outer']['state'], 'regular')
                    self.assertEqual(entry['inner']['state'], 'missing')
                    self.assertEqual(entry['inner']['error'], 'ENOENT')
                self.assertNotIn(b'fixture_alpha', harness.diagnostic_json(receipt))
                self.assertNotIn(str(workspace).encode(), harness.diagnostic_json(receipt))


    def paired_fixture(self):
        folder = tempfile.TemporaryDirectory(prefix='f13-paired-schema-')
        self.addCleanup(folder.cleanup)
        root = Path(folder.name)
        base = root / 'historical-git'
        (base / 'util').mkdir(parents=True)
        target = root / 'PRIVATE_PATH_SENTINEL'
        target.write_text('PRIVATE_CONTENT_SENTINEL')
        target.chmod(0o555)
        (base / 'util' / 'PRIVATE_NAME_SENTINEL').symlink_to(target)
        actual = {'PRIVATE_NAME_SENTINEL': str(target)}
        identity = dict(commit='a' * 40, files={'driver': 'b' * 64})
        context = harness.prepare_paired_utility_context(root, identity)
        observation = harness.paired_utility_observation(base, actual, {}, 1, tuple(actual), identity)
        mismatch = harness.diagnostic_json(harness.utility_view_diagnostic(base, actual, {}, 1, tuple(actual))) + b'\n'
        return root, base, target, actual, identity, context, observation, mismatch

    def test_probe_real_denied_not_directory_loop_limit_and_metadata_change(self):
        with tempfile.TemporaryDirectory(prefix='f13-paired-probe-') as folder:
            root = Path(folder)
            denied = root / 'denied'
            denied.mkdir()
            (denied / 'program').write_text('PRIVATE_CONTENT_SENTINEL')
            denied.chmod(0)
            try:
                observation = harness.paired_referent_observation(str(denied / 'program'))
                self.assertEqual((observation['state'], observation['error']), ('denied', 'EACCES'))
            finally:
                denied.chmod(0o700)
            regular = root / 'program'
            regular.write_text('PRIVATE_CONTENT_SENTINEL')
            regular.chmod(0o555)
            before = harness.paired_referent_observation(str(regular))
            self.assertEqual(before['state'], 'regular')
            self.assertTrue(before['executable'])
            regular.chmod(0o755)
            regular.write_text('different private contents and size')
            regular.chmod(0o555)
            after = harness.paired_referent_observation(str(regular))
            self.assertNotEqual(before['metadata_sha256'], after['metadata_sha256'])
            self.assertEqual(harness.paired_referent_observation(str(regular / 'child'))['error'], 'ENOTDIR')
            (root / 'loop-a').symlink_to('loop-b')
            (root / 'loop-b').symlink_to('loop-a')
            self.assertEqual(harness.paired_referent_observation(str(root / 'loop-a'))['state'], 'loop')
            for index in range(9):
                (root / ('edge' + str(index))).symlink_to('edge' + str(index+1))
            self.assertEqual(harness.paired_referent_observation(str(root / 'edge0'))['state'], 'limit')
            self.assertEqual(harness.paired_referent_observation('relative/PRIVATE_PATH')['state'], 'not_inspected')
            self.assertNotIn(b'PRIVATE', harness.diagnostic_json(before))

    def test_same_context_no_catalog_dependency_and_opaque_only(self):
        root, base, target, actual, identity, context, observation, mismatch = self.paired_fixture()
        with mock.patch.object(harness, 'read_public_utility_catalog', side_effect=AssertionError('must not inspect catalog')):
            self.assertIsNotNone(harness.prepare_paired_utility_context(root, identity))
        raw = harness.encode_paired_utility_observation(observation)
        out, err, receipt = harness.resolve_owned_paired_output(b'', mismatch+raw, context)
        self.assertEqual((out, err), (b'', mismatch))
        self.assertIsNotNone(receipt)
        self.assertEqual(receipt['entries'][0]['outer'], receipt['entries'][0]['inner'])
        self.assertTrue(receipt['credentials_equal'])
        for secret in (b'PRIVATE_NAME', b'PRIVATE_PATH', b'PRIVATE_CONTENT', str(root).encode()):
            self.assertNotIn(secret, raw)
            self.assertNotIn(secret, harness.diagnostic_json(receipt))
        self.assertIsNone(harness.resolve_owned_paired_output(b'', mismatch+raw, None)[2])
        self.assertIsNone(harness.resolve_owned_paired_output(b'', raw, context)[2], 'no original mismatch cannot authorize a paired record')

    def test_pair_strict_source_map_tokens_schema_and_negative_shapes(self):
        root, base, target, actual, identity, context, value, mismatch = self.paired_fixture()
        mutations = []
        def changed(path, replacement):
            item = json.loads(json.dumps(value))
            cursor = item
            for key in path[:-1]:
                cursor = cursor[key]
            cursor[path[-1]] = replacement
            mutations.append(item)
        for field in ('source_sha256', 'actual_sha256'):
            changed((field,), '0' * 64)
        changed(('entry_count',), 2)
        changed(('omitted',), True)
        changed(('credential_sha256',), 'PRIVATE_SECRET')
        changed(('entries', 0, 'target', 'sha256'), '0' * 64)
        changed(('entries', 0, 'tool', 'utf8_bytes'), True)
        changed(('entries', 0, 'probe', 'state'), 'PRIVATE_SECRET')
        changed(('entries', 0, 'probe', 'path'), {'path': '/PRIVATE_SECRET'})
        changed(('entries', 0, 'probe', 'error'), 'ENOENT')  # incompatible with regular metadata
        changed(('entries',), value['entries'] * 2)
        for item in mutations:
            with self.subTest(item=item):
                raw = harness.encode_paired_utility_observation(item)
                self.assertIsNone(harness.resolve_owned_paired_output(b'', mismatch+raw, context)[2])
        raw = harness.encode_paired_utility_observation(value)
        duplicated = raw.replace(b'{"actual_count":1,', b'{"actual_count":1,"actual_count":1,', 1)
        self.assertNotEqual(raw, duplicated)
        self.assertIsNone(harness.resolve_owned_paired_output(b'', mismatch+duplicated, context)[2])

    def test_new_prefix_suppression_preserves_old_resolver_and_unrelated_bytes(self):
        root, base, target, actual, identity, context, value, mismatch = self.paired_fixture()
        raw = harness.encode_paired_utility_observation(value)
        old = harness.encode_opaque_observation(harness.opaque_utility_observation(base, actual, {}, 1, tuple(actual), identity))
        public = harness.public_diagnostic_context((), actual, identity)
        expected = harness.resolve_owned_utility_output(b'guard\r\n', mismatch+old+b'unrelated\n', public)
        out, err, receipt = harness.resolve_owned_paired_output(b'guard\r\n', mismatch+old+raw+b'unrelated\n', context)
        self.assertIsNotNone(receipt)
        self.assertEqual(harness.resolve_owned_utility_output(out, err, public), expected)
        broken = b'F13_PAIRED_UTILITY_BROKEN PRIVATE_SECRET\n'
        for first, second in ((broken, broken[:-1]), (raw, raw), (b'', raw+raw),
                              (b'', harness.PAIRED_UTILITY_PREFIX+b'x'*16385+b'\n')):
            out, err, receipt = harness.resolve_owned_paired_output(b'out\n'+first, mismatch+second, context)
            self.assertEqual((out, err, receipt), (b'out\n', mismatch, None))

    def test_paired_bounds_truthful_omission_and_failure_containment(self):
        root, base, target, actual, identity, context, value, mismatch = self.paired_fixture()
        many = {'PRIVATE_NAME_'+str(i): str(target) for i in range(100)}
        observed = harness.paired_utility_observation(base, many, {}, 100, tuple(many), identity)
        self.assertEqual(len(observed['entries'])+observed['omitted'], 100)
        self.assertGreater(observed['omitted'], 0)
        self.assertLessEqual(harness.diagnostic_units(observed), 32)
        self.assertLessEqual(len(harness.encode_paired_utility_observation(observed)), 16384)
        with self.assertRaises(ValueError):
            harness.encode_paired_utility_observation({'entries': [{} for _ in range(33)]})
        with mock.patch.object(harness.os, 'getgroups', return_value=list(range(33))):
            self.assertIsNone(harness.prepare_paired_utility_context(root, identity))
        with mock.patch.object(harness.os, 'lstat', side_effect=OSError(5, 'PRIVATE_SECRET')):
            observed = harness.paired_referent_observation(str(target))
            self.assertEqual((observed['state'], observed['error']), ('io_error', 'EIO'))
        class Broken(io.StringIO):
            def write(self, text):
                raise OSError('PRIVATE_SECRET')
        with mock.patch.dict(os.environ, {'F13_HARNESS_IDENTITY': json.dumps(identity)}), contextlib.redirect_stderr(Broken()):
            harness.emit_paired_utility_observation(base, actual, {}, 1, tuple(actual))


    def test_namespace_pairs_owned_output_without_spawn_growth_or_setup_waiver(self):
        import types
        for broken_output in (False, True):
            with self.subTest(broken_output=broken_output):
                holder = {}
                class Output(io.BytesIO):
                    def write(self, raw):
                        if broken_output and b'F13_PAIRED_UTILITY_CONTEXT_RECEIPT_v1' in raw:
                            raise OSError('PRIVATE_OUTPUT_SECRET')
                        return super().write(raw)
                outputs = (io.BytesIO(), Output())
                def command(args):
                    if args[-2:] == ['rev-parse', 'HEAD']:
                        return b'a' * 40+b'\n'
                    return (harness.ROOT / args[-1].split(':', 1)[1]).read_bytes()
                def materialize(root):
                    program = root / 'PRIVATE_PROGRAM'
                    program.write_text('private data')
                    program.chmod(0o555)
                    (root / 'historical-git' / 'util').mkdir(parents=True)
                    (root / 'historical-git' / 'util' / 'PRIVATE_TOOL').symlink_to(program)
                    holder.update(root=root, program=program)
                def spawn(args, **kwargs):
                    # Child-only synthetic candidate: no owning outer snapshot exists.
                    root = Path(tempfile.mkdtemp(prefix='f13-deferred-candidate-'))
                    self.addCleanup(shutil.rmtree, root)
                    materialize(root)
                    actual = {'PRIVATE_TOOL': str(holder['program'])}
                    holder['program'].unlink()
                    identity = dict(commit='a' * 40, files={name:harness.sha((harness.ROOT / name).read_bytes())
                                                          for name in harness.HARNESS_SOURCE_PATHS})
                    value = harness.paired_utility_observation(root/'historical-git', actual, {}, 1, tuple(actual), identity)
                    mismatch = harness.diagnostic_json(harness.utility_view_diagnostic(root/'historical-git', actual, {}, 1, tuple(actual)))+b'\n'
                    return subprocess.CompletedProcess(args, 2, b'unchanged stdout\n', mismatch+harness.encode_paired_utility_observation(value))
                with mock.patch.object(harness.os, 'geteuid', return_value=1000), \
                     mock.patch.object(harness.shutil, 'which', return_value='/trusted/tool'), \
                     mock.patch.object(harness, 'run', side_effect=command), \
                     mock.patch.object(harness, 'materialize', side_effect=materialize), \
                     mock.patch.object(harness, 'prepare_public_diagnostic_context', return_value=None), \
                     mock.patch.object(harness.subprocess, 'run', side_effect=spawn) as child, \
                     mock.patch.object(harness.sys, 'stdout', types.SimpleNamespace(buffer=outputs[0])), \
                     mock.patch.object(harness.sys, 'stderr', types.SimpleNamespace(buffer=outputs[1])):
                    with self.assertRaisesRegex(harness.SetupFailure, '^namespace_must_return_actual_guard_receipt_not_setup_exit$'):
                        harness.namespace_main()
                self.assertEqual(child.call_count, 1)
                self.assertEqual(child.call_args.kwargs, {'capture_output':True})
                self.assertEqual(outputs[0].getvalue(), b'unchanged stdout\n')
                raw = outputs[1].getvalue()
                self.assertFalse(any(line.startswith(harness.PAIRED_UTILITY_NAMESPACE) for line in raw.splitlines()))
                records = [json.loads(line) for line in raw.splitlines()]
                pairs = [r for r in records if r.get('kind')=='F13_PAIRED_UTILITY_CONTEXT_RECEIPT_v1']
                self.assertEqual(len(pairs), 0, 'deferred preparation has no authenticated outer snapshot')
                if pairs:
                    self.assertEqual(pairs[0]['entries'][0]['outer']['state'], 'regular')
                    self.assertEqual(pairs[0]['entries'][0]['inner']['error'], 'ENOENT')
                self.assertNotIn(b'PRIVATE_PROGRAM', raw)
                self.assertNotIn(b'PRIVATE_TOOL', raw)

class ContextFailureContract(unittest.TestCase):
    """Parent-only failure observation; never hosted cause or guard proof."""
    def test_real_descriptor_failure_keeps_none_and_emits_only_fixed_reason(self):
        import errno
        import resource
        import types
        self.assertNotEqual(os.geteuid(), 0)
        self.assertIsNotNone(harness.read_public_utility_catalog())
        folder = tempfile.TemporaryDirectory(prefix='f13-context-recovery-')
        self.addCleanup(folder.cleanup)
        prepared_root = Path(folder.name)
        harness.materialize_pinned_git(prepared_root)
        self.assertIsNotNone(harness.prepare_public_diagnostic_context(prepared_root, {}))
        mismatch, _, _ = CoarseStatusContract().fixture()
        outputs = (io.BytesIO(), io.BytesIO())
        original_open = harness.os.open
        observed = []
        def traced_open(*args, **kwargs):
            try:
                return original_open(*args, **kwargs)
            except OSError as error:
                observed.append(error.errno)
                raise
        def command(args):
            if args[-2:] == ['rev-parse', 'HEAD']:
                return b'a' * 40 + b'\n'
            return (harness.ROOT / args[-1].split(':', 1)[1]).read_bytes()
        def materialize(root):
            (root / 'historical-git').mkdir()
        limits = resource.getrlimit(resource.RLIMIT_NOFILE)
        before = len(os.listdir('/proc/self/fd'))
        try:
            resource.setrlimit(resource.RLIMIT_NOFILE, (32, limits[1]))
            with mock.patch.object(harness.os, 'open', side_effect=traced_open):
                direct_failure = {}
                self.assertIsNone(harness.prepare_public_diagnostic_context(prepared_root, {}, direct_failure))
                with mock.patch.object(harness.os, 'geteuid', return_value=1000), \
                     mock.patch.object(harness.shutil, 'which', return_value='/trusted/tool'), \
                     mock.patch.object(harness, 'run', side_effect=command), \
                     mock.patch.object(harness, 'materialize', side_effect=materialize), \
                     mock.patch.object(harness.subprocess, 'run', return_value=subprocess.CompletedProcess([], 2, b'original stdout\n', mismatch)) as spawn, \
                     mock.patch.object(harness.sys, 'stdout', types.SimpleNamespace(buffer=outputs[0])), \
                     mock.patch.object(harness.sys, 'stderr', types.SimpleNamespace(buffer=outputs[1])):
                    with self.assertRaisesRegex(harness.SetupFailure, '^namespace_must_return_actual_guard_receipt_not_setup_exit$'):
                        harness.namespace_main()
        finally:
            resource.setrlimit(resource.RLIMIT_NOFILE, limits)
        self.assertIn(errno.EMFILE, observed, 'must exercise actual original open exhaustion')
        self.assert_fixed_reason(direct_failure, 'catalog_descriptor_exhausted')
        self.assertEqual(len(os.listdir('/proc/self/fd')), before)
        self.assertIsNotNone(harness.read_public_utility_catalog())
        self.assertIsNotNone(harness.prepare_public_diagnostic_context(prepared_root, {}))
        self.assertEqual(spawn.call_count, 1)
        self.assertEqual(spawn.call_args.kwargs, {'capture_output': True})
        self.assertEqual(outputs[0].getvalue(), b'original stdout\n')
        raw = outputs[1].getvalue()
        self.assertTrue(raw.startswith(mismatch))
        self.assertEqual([json.loads(line) for line in raw[len(mismatch):].splitlines()], [
            dict(kind='F13_PUBLIC_UTILITY_DIAGNOSTIC_STATUS_v1', status='context_unavailable')])
        # Namespace did not attempt outer catalog collection; never invent its failure.


    def assert_fixed_reason(self, failure, reason):
        import types
        self.assertEqual(failure, {'reason': reason})
        mismatch, _, _ = CoarseStatusContract().fixture()
        status = harness.coarse_public_diagnostic_status(b'', mismatch, None, None)
        output = io.BytesIO()
        with mock.patch.object(harness.sys, 'stderr', types.SimpleNamespace(buffer=output)):
            harness.emit_public_context_failure(status, failure)
        self.assertEqual(json.loads(output.getvalue()),
                         dict(kind='F13_PUBLIC_UTILITY_CONTEXT_FAILURE_v1', reason=reason))
        self.assertEqual(output.getvalue().count(b'\n'), 1)
        self.assertNotIn(b'PRIVATE_SECRET', output.getvalue())

    def test_catalog_fixed_open_trust_and_directory_boundary(self):
        import types
        cases = [
            (mock.patch.object(harness.os, 'open', side_effect=PermissionError('PRIVATE_SECRET/path')),
             'catalog_open_failed'),
            (mock.patch.object(harness.os, 'getxattr', side_effect=PermissionError('PRIVATE_SECRET/path')),
             'catalog_permissions_or_acl_failed'),
            (mock.patch.object(harness.os, 'scandir', return_value=contextlib.nullcontext(iter(
                [types.SimpleNamespace(name='PRIVATE_SECRET') for _ in range(4097)]))),
             'catalog_directory_limit'),
        ]
        for patch, reason in cases:
            with self.subTest(reason=reason), patch:
                failure = {}
                self.assertIsNone(harness.read_public_utility_catalog(failure))
            self.assert_fixed_reason(failure, reason)

    def test_catalog_fixed_list_open_trust_read_and_canonical_sites(self):
        import stat
        original_open, original_xattr = harness.os.open, harness.os.getxattr
        def list_open(*args, **kwargs):
            if str(args[0]).endswith('.list'):
                raise PermissionError('PRIVATE_SECRET/path')
            return original_open(*args, **kwargs)
        def list_trust(fd, attr):
            if stat.S_ISREG(os.fstat(fd).st_mode):
                raise PermissionError('PRIVATE_SECRET/path')
            return original_xattr(fd, attr)
        cases = [
            (mock.patch.object(harness.os, 'open', side_effect=list_open), 'catalog_list_open_failed'),
            (mock.patch.object(harness.os, 'getxattr', side_effect=list_trust), 'catalog_list_trust_failed'),
            (mock.patch.object(harness.os, 'read', side_effect=OSError('PRIVATE_SECRET/path')), 'catalog_list_bytes_or_changed'),
            (mock.patch.object(harness, 'public_catalog_from_lists', side_effect=ValueError('PRIVATE_SECRET/path')),
             'catalog_canonical_failed'),
        ]
        for patch, reason in cases:
            with self.subTest(reason=reason), patch:
                failure = {}
                self.assertIsNone(harness.read_public_utility_catalog(failure))
            self.assert_fixed_reason(failure, reason)

    def test_catalog_final_revalidation_and_generator_parser_phase(self):
        parser, xattr = harness.public_catalog_from_lists, harness.os.getxattr
        done = []
        def parsed(lists):
            result = parser(lists)
            done.append(True)
            return result
        def final_trust(fd, attr):
            if done:
                raise PermissionError('PRIVATE_SECRET/path')
            return xattr(fd, attr)
        failure = {}
        with mock.patch.object(harness, 'public_catalog_from_lists', side_effect=parsed), \
             mock.patch.object(harness.os, 'getxattr', side_effect=final_trust):
            self.assertIsNone(harness.read_public_utility_catalog(failure))
        self.assert_fixed_reason(failure, 'catalog_changed')
        def reject_actual_list(lists):
            self.assertIsInstance(next(iter(lists)), bytes)
            raise ValueError('PRIVATE_SECRET/path')
        failure = {}
        with mock.patch.object(harness, 'public_catalog_from_lists', side_effect=reject_actual_list):
            self.assertIsNone(harness.read_public_utility_catalog(failure))
        self.assert_fixed_reason(failure, 'catalog_canonical_failed')

    def test_context_fixed_fallback_snapshot_limits_and_build_sites(self):
        import types
        catalog = ('/usr/bin/publictool',)
        failure = {}
        with mock.patch.object(harness, 'read_public_utility_catalog', return_value=None):
            self.assertIsNone(harness.prepare_public_diagnostic_context(Path('/absent'), {}, failure))
        self.assert_fixed_reason(failure, 'context_catalog_unavailable')
        with mock.patch.object(harness, 'read_public_utility_catalog', return_value=catalog):
            failure = {}
            self.assertIsNone(harness.prepare_public_diagnostic_context(Path('/absent'), {}, failure))
            self.assert_fixed_reason(failure, 'context_snapshot_failed')
            root = mock.MagicMock()
            base = root.__truediv__.return_value.__truediv__.return_value
            base.iterdir.return_value = iter([types.SimpleNamespace(is_symlink=lambda:False)] * 8193)
            failure = {}
            self.assertIsNone(harness.prepare_public_diagnostic_context(root, {}, failure))
            self.assert_fixed_reason(failure, 'context_snapshot_limit')
            base.iterdir.return_value = iter([types.SimpleNamespace(name='publictool', is_symlink=lambda:True,
                readlink=lambda:'/' + 'x' * 2097152)])
            failure = {}
            self.assertIsNone(harness.prepare_public_diagnostic_context(root, {}, failure))
            self.assert_fixed_reason(failure, 'context_snapshot_limit')
            with tempfile.TemporaryDirectory(prefix='f13-context-build-') as folder:
                root = Path(folder)
                (root / 'historical-git' / 'util').mkdir(parents=True)
                failure = {}
                with mock.patch.object(harness, 'public_diagnostic_context', side_effect=ValueError('PRIVATE_SECRET/path')):
                    self.assertIsNone(harness.prepare_public_diagnostic_context(root, {}, failure))
                self.assert_fixed_reason(failure, 'context_build_failed')

    def test_success_non_mismatch_and_other_context_statuses_are_silent(self):
        import types
        failure = {}
        self.assertIsNotNone(harness.read_public_utility_catalog(failure))
        self.assertEqual(failure, {})
        mismatch, context, receipt = CoarseStatusContract().fixture(public=True)
        inputs = [(b'', b'', None, None), (b'not-json\n', b'', None, None),
                  (b'', mismatch, context, None), (b'', mismatch, context, receipt)]
        output = io.BytesIO()
        with mock.patch.object(harness.sys, 'stderr', types.SimpleNamespace(buffer=output)):
            for args in inputs:
                harness.emit_public_context_failure(harness.coarse_public_diagnostic_status(*args),
                                                   {'reason': 'catalog_descriptor_exhausted'})
        self.assertEqual(output.getvalue(), b'')

    def test_recorder_and_emitter_never_disclose_untrusted_values(self):
        import types
        class Hostile(dict):
            def get(self, *args):
                raise AssertionError('PRIVATE_SECRET/path')
            def __setitem__(self, *args):
                raise AssertionError('PRIVATE_SECRET/path')
        class Text(str):
            pass
        output = io.BytesIO()
        with mock.patch.object(harness.sys, 'stderr', types.SimpleNamespace(buffer=output)):
            for failure in (None, [], Hostile(reason='catalog_open_failed'), {},
                            {'reason': 'PRIVATE_SECRET/path'}, {'reason': Text('catalog_open_failed')}):
                harness.emit_public_context_failure('context_unavailable', failure)
            for status in (None, Text('context_unavailable'), 'PRIVATE_SECRET/path'):
                harness.emit_public_context_failure(status, {'reason': 'catalog_open_failed'})
        self.assertEqual(output.getvalue(), b'')
        for failure in (None, [], Hostile()):
            harness.record_public_context_failure(failure, 'catalog_open_failed')
            with mock.patch.object(harness.os, 'open', side_effect=PermissionError('PRIVATE_SECRET/path')):
                self.assertIsNone(harness.read_public_utility_catalog(failure))
        failure = {}
        harness.record_public_context_failure(failure, 'PRIVATE_SECRET/path')
        self.assertEqual(failure, {})
        harness.record_public_context_failure(failure, 'catalog_open_failed')
        harness.record_public_context_failure(failure, 'context_catalog_unavailable', preserve=True)
        self.assertEqual(failure, {'reason': 'catalog_open_failed'})
        broken = mock.Mock()
        broken.write.side_effect = OSError('PRIVATE_SECRET/path')
        with mock.patch.object(harness.sys, 'stderr', types.SimpleNamespace(buffer=broken)):
            harness.emit_public_context_failure('context_unavailable', failure)
        self.assertEqual(broken.write.call_count, 1)

class CoarseStatusContract(unittest.TestCase):
    """Optional observation status, never guard/admission evidence."""
    def fixture(self, public=False):
        actual = {'publictool': '/usr/bin/publictool'} if public else {'PRIVATE_SECRET': '/private/PRIVATE_SECRET'}
        identity = {'commit': 'a' * 40, 'files': {'driver': 'b' * 64}}
        context = harness.public_diagnostic_context(('/usr/bin/publictool',), actual, identity)
        observation = harness.opaque_utility_observation(Path('/absent'), actual, {}, 1, tuple(actual), identity)
        receipt = harness.resolve_owned_utility_output(b'', harness.encode_opaque_observation(observation), context)[2]
        self.assertIsNotNone(receipt)
        mismatch = harness.diagnostic_json(harness.utility_view_diagnostic(Path('/absent'), actual, {}, 1, tuple(actual))) + b'\n'
        return mismatch, context, receipt

    def test_context_unavailable(self):
        mismatch, _, _ = self.fixture()
        self.assertEqual(harness.coarse_public_diagnostic_status(b'', mismatch, None, None), 'context_unavailable')

    def test_candidate_rejected_or_not_selected(self):
        mismatch, context, _ = self.fixture()
        self.assertEqual(harness.coarse_public_diagnostic_status(mismatch, b'', context, None), 'candidate_rejected_or_not_selected')

    def test_no_catalog_match(self):
        mismatch, context, receipt = self.fixture()
        self.assertEqual(harness.coarse_public_diagnostic_status(b'', mismatch, context, receipt), 'catalog_match_failed')

    def test_qualified_match_stays_quiet(self):
        mismatch, context, receipt = self.fixture(public=True)
        self.assertIsNone(harness.coarse_public_diagnostic_status(b'', mismatch, context, receipt))

    def test_no_original_mismatch_stays_quiet(self):
        for raw in (b'', b'F13_HISTORICAL_SETUP_FAILURE\n', b'{"kind":"F13_UTILITY_VIEW_DIAGNOSTIC_v1","status":"diagnostic_unavailable"}\n'):
            self.assertIsNone(harness.coarse_public_diagnostic_status(raw, b'', None, None))

    def test_malformed_and_overbudget_stay_quiet(self):
        for raw in (b'{PRIVATE_SECRET\n', b'\xff\n', b'x' * 16385,
                    json.dumps({'kind': 'PRIVATE_SECRET', 'status': 'mismatch'}).encode()):
            self.assertIsNone(harness.coarse_public_diagnostic_status(b'', raw, None, None))

    def test_qualified_closure_match_and_malformed_receipt(self):
        mismatch, context, receipt = self.fixture()
        for invalid in ({}, {'kind': receipt['kind'], 'entries': 'PRIVATE_SECRET'}):
            self.assertIsNone(harness.coarse_public_diagnostic_status(b'', mismatch, context, invalid))
        token = receipt['entries'][0]['actual']['target']
        receipt['entries'][0]['actual']['closure']['edges'] = [dict(token, path='/usr/bin/publictool',
            qualification='OUTER_CATALOG_RESOLUTION_OF_CHILD_TOKEN')]
        self.assertIsNone(harness.coarse_public_diagnostic_status(b'', mismatch, context, receipt))

    def test_fixed_emission_only_and_output_failure_contained(self):
        import types
        output = io.BytesIO()
        with mock.patch.object(harness.sys, 'stderr', types.SimpleNamespace(buffer=output)):
            for value in ('context_unavailable', 'candidate_rejected_or_not_selected', 'catalog_match_failed',
                          None, 'PRIVATE_SECRET', {'path': '/PRIVATE_SECRET'}):
                harness.emit_coarse_public_diagnostic_status(value)
        records = [json.loads(line) for line in output.getvalue().splitlines()]
        self.assertEqual([item['status'] for item in records],
                         ['context_unavailable', 'candidate_rejected_or_not_selected', 'catalog_match_failed'])
        self.assertTrue(all(set(item) == {'kind', 'status'} for item in records))
        self.assertNotIn(b'PRIVATE_SECRET', output.getvalue())
        broken = mock.Mock()
        broken.write.side_effect = OSError('PRIVATE_SECRET')
        with mock.patch.object(harness.sys, 'stderr', types.SimpleNamespace(buffer=broken)):
            harness.emit_coarse_public_diagnostic_status('context_unavailable')

    def test_namespace_setup_failure_preserved_with_coarse_context_status(self):
        import types
        mismatch, _, _ = self.fixture()
        original_out = b'original stdout\n'
        original_err = mismatch + b'F13_HISTORICAL_SETUP_FAILURE: pinned_Git_private_utility_view\n'
        outputs = (io.BytesIO(), io.BytesIO())
        def command(args):
            if args[-2:] == ['rev-parse', 'HEAD']:
                return b'a' * 40 + b'\n'
            return (harness.ROOT / args[-1].split(':', 1)[1]).read_bytes()
        def materialize(root):
            (root / 'historical-git').mkdir()
        with mock.patch.object(harness.os, 'geteuid', return_value=1000), \
             mock.patch.object(harness.shutil, 'which', return_value='/trusted/tool'), \
             mock.patch.object(harness, 'run', side_effect=command), \
             mock.patch.object(harness, 'materialize', side_effect=materialize), \
             mock.patch.object(harness, 'prepare_public_diagnostic_context', return_value=None), \
             mock.patch.object(harness.subprocess, 'run', return_value=subprocess.CompletedProcess([], 2, original_out, original_err)) as spawn, \
             mock.patch.object(harness.sys, 'stdout', types.SimpleNamespace(buffer=outputs[0])), \
             mock.patch.object(harness.sys, 'stderr', types.SimpleNamespace(buffer=outputs[1])):
            with self.assertRaisesRegex(harness.SetupFailure, '^namespace_must_return_actual_guard_receipt_not_setup_exit$'):
                harness.namespace_main()
        self.assertEqual(spawn.call_count, 1)
        self.assertEqual(spawn.call_args.kwargs, {'capture_output': True}, 'stdin/env/invocation unchanged')
        self.assertEqual(outputs[0].getvalue(), original_out)
        raw = outputs[1].getvalue()
        self.assertTrue(raw.startswith(original_err))
        self.assertEqual(json.loads(raw[len(original_err):]),
                         dict(kind='F13_PUBLIC_UTILITY_DIAGNOSTIC_STATUS_v1', status='context_unavailable'))


class OuterPublicResolution(unittest.TestCase):
    """Synthetic diagnostic/privacy proof, never actual F13 guard RED."""
    def setUp(self):
        self.identity = {'commit': 'a' * 40, 'files': {'driver': 'b' * 64}}
        self.actual = {'publictool': '/usr/bin/publictool', 'private': '/private/SECRET'}
        self.catalog = ('/usr/bin/publictool',)
        self.context = harness.public_diagnostic_context(self.catalog, self.actual, self.identity)

    def observation(self):
        return harness.opaque_utility_observation(Path('/nonexistent'), self.actual, {}, 2,
                                                 tuple(self.actual), self.identity)

    def test_public_basename_never_qualifies_private_target(self):
        self.actual = {'publictool': '/private/SECRET'}
        context = harness.public_diagnostic_context(self.catalog, self.actual, self.identity)
        observation = self.observation()
        raw = harness.encode_opaque_observation(observation)
        out, err, receipt = harness.resolve_owned_utility_output(b'original\n', raw, context)
        self.assertEqual((out, err), (b'original\n', b''))
        self.assertIsNotNone(receipt)
        self.assertEqual(receipt['entries'][0]['tool']['basename'], 'publictool')
        self.assertNotIn('path', receipt['entries'][0]['actual']['target'])
        self.assertNotIn('SECRET', json.dumps(receipt))

    def test_cached_map_source_and_union_duplicate_reject(self):
        raw = harness.encode_opaque_observation(self.observation())
        for stdout, stderr in ((raw, raw), (b'', raw + raw)):
            out, err, receipt = harness.resolve_owned_utility_output(stdout, stderr, self.context)
            self.assertEqual((out, err, receipt), (b'', b'', None))
        for field in ('actual_sha256', 'source_sha256'):
            candidate = self.observation()
            candidate[field] = '0' * 64
            _, _, receipt = harness.resolve_owned_utility_output(b'', harness.encode_opaque_observation(candidate), self.context)
            self.assertIsNone(receipt)

    def test_both_stream_malformed_prefix_suppression_preserves_unrelated_bytes(self):
        raw = b'F13_OPAQUE_UTILITY_BROKEN SECRET\r\n'
        out, err, receipt = harness.resolve_owned_utility_output(b'guard\r\n' + raw, b'err\n' + raw[:-2], self.context)
        self.assertEqual((out, err, receipt), (b'guard\r\n', b'err\n', None))

    def test_opaque_child_never_emits_new_raw_names_or_targets(self):
        raw = harness.encode_opaque_observation(self.observation())
        self.assertNotIn(b'publictool', raw)
        self.assertNotIn(b'SECRET', raw)
        self.assertLessEqual(len(raw), 16384)

    def test_namespace_owner_integrates_resolution_without_stdin_or_exec_growth(self):
        import types
        outputs = (io.BytesIO(), io.BytesIO())
        calls = []
        def command(args):
            if args[-2:] == ['rev-parse', 'HEAD']:
                return b'a' * 40 + b'\n'
            return (harness.ROOT / args[-1].split(':', 1)[1]).read_bytes()
        def materialize(root):
            (root / 'historical-git/util').mkdir(parents=True)
            (root / 'historical-git/util/publictool').symlink_to('/usr/bin/publictool')
        def spawn(args, **kwargs):
            calls.append((args, kwargs))
            self.assertEqual(set(kwargs), {'capture_output'})
            self.assertTrue(kwargs['capture_output'])
            identity = dict(commit='a' * 40, files={name: harness.sha((harness.ROOT / name).read_bytes())
                                                  for name in harness.HARNESS_SOURCE_PATHS})
            observation = harness.opaque_utility_observation(Path('/absent'), {'publictool': '/usr/bin/publictool'}, {}, 1,
                                                            ('publictool',), identity)
            guard = b'{"kind":"FAITHFUL_HISTORICAL_SHARED_GUARD","guard_observed":1,"contract_expected":0}\n'
            return subprocess.CompletedProcess(args, 1, guard, harness.encode_opaque_observation(observation))
        with mock.patch.object(harness.os, 'geteuid', return_value=1000), \
             mock.patch.object(harness.shutil, 'which', return_value='/trusted/tool'), \
             mock.patch.object(harness, 'run', side_effect=command), \
             mock.patch.object(harness, 'materialize', side_effect=materialize), \
             mock.patch.object(harness, 'read_public_utility_catalog', return_value=self.catalog), \
             mock.patch.object(harness.subprocess, 'run', side_effect=spawn), \
             mock.patch.object(harness.sys, 'stdout', types.SimpleNamespace(buffer=outputs[0])), \
             mock.patch.object(harness.sys, 'stderr', types.SimpleNamespace(buffer=outputs[1])):
            self.assertEqual(harness.namespace_main(), 1)
        self.assertEqual(len(calls), 1)
        self.assertNotIn('F13_OPAQUE', calls[0][0][-1])
        self.assertNotIn('F13_PUBLIC', calls[0][0][-1])
        self.assertNotIn(b'F13_PUBLIC_UTILITY_OUTER_RESOLUTION_RECEIPT_v1', outputs[1].getvalue(),
                         'no owning outer context exists before deferred preparation')
        self.assertNotIn(b'F13_OPAQUE_UTILITY_', outputs[1].getvalue())

    def test_catalog_parser_canonical_lexical_and_bounds(self):
        self.assertEqual(harness.public_catalog_from_lists([b'/usr/bin/z\n/bin/a\n/usr/bin/../private\n/usr/local/bin/secret\n']),
                         ('/bin/a', '/usr/bin/z'))
        for raw in (b'/usr/bin/x', b'/usr/bin/x\0\n', b'x' * 1048577):
            with self.assertRaises((ValueError, UnicodeError)):
                harness.public_catalog_from_lists([raw])
        with self.assertRaises(ValueError):
            harness.public_catalog_from_lists([b''] * 4097)

    def test_schema_cleartext_duplicate_keys_types_and_relative_edges_reject(self):
        value = self.observation()
        for mutate in (lambda x: x.update(entry_count=True), lambda x: x.update(private='/SECRET'),
                       lambda x: x['entries'][0]['tool'].update(basename='SECRET')):
            candidate = json.loads(json.dumps(value))
            mutate(candidate)
            self.assertIsNone(harness.resolve_owned_utility_output(b'', harness.encode_opaque_observation(candidate), self.context)[2])
        raw = harness.encode_opaque_observation(value).replace(b'{', b'{"actual_count":2,', 1)
        self.assertIsNone(harness.resolve_owned_utility_output(b'', raw, self.context)[2])

    def test_real_installed_catalog_or_closed_unavailability_and_acl_errors(self):
        catalog = harness.read_public_utility_catalog()
        if catalog is not None:
            self.assertIn('/usr/bin/dpkg', catalog)
            self.assertEqual(catalog, tuple(sorted(set(catalog))))
        with mock.patch.object(harness.os, 'getxattr', return_value=b'extended_acl'):
            self.assertIsNone(harness.read_public_utility_catalog())
        with mock.patch.object(harness.os, 'getxattr', side_effect=OSError(13, 'SECRET')):
            self.assertIsNone(harness.read_public_utility_catalog())

    def test_each_closure_edge_relative_unknown_and_surrogate_tokens(self):
        with tempfile.TemporaryDirectory() as folder:
            private = Path(folder) / 'PRIVATE_SECRET'
            private.write_bytes(b'contents not read')
            alias = Path(folder) / 'alias'
            alias.symlink_to(private)
            relative = Path(folder) / 'relative'
            relative.symlink_to('PRIVATE_SECRET')
            self.assertEqual(harness.opaque_utility_closure(str(alias))['edges'], [harness.opaque_utility_token(str(private))])
            self.assertEqual(harness.opaque_utility_closure(str(relative))['edges'], [harness.opaque_utility_token('PRIVATE_SECRET')])
            self.assertNotIn('PRIVATE_SECRET', json.dumps(harness.opaque_utility_closure(str(relative))))
        candidate = self.observation()
        candidate['entries'][0]['actual']['closure'] = dict(edges=[harness.opaque_utility_token('publictool'),
            harness.opaque_utility_token('/usr/bin/publictool')], status='complete')
        receipt = harness.resolve_owned_utility_output(b'', harness.encode_opaque_observation(candidate), self.context)[2]
        edges = receipt['entries'][0]['actual']['closure']['edges']
        self.assertNotIn('path', edges[0], 'basename must never qualify a relative edge')
        self.assertEqual(edges[1]['path'], '/usr/bin/publictool')
        token = harness.opaque_utility_token('\udcff')
        self.assertEqual(token['utf8_bytes'], 3)
        self.assertEqual(token, harness.utility_diagnostic_token('\udcff'))

    def test_catalog_canonical_max_and_plus_one_and_snapshot_bounds(self):
        # 8192 direct-child public paths with <=131072 canonical bytes.
        raw = b''.join(('/bin/a%04x\n' % i).encode() for i in range(8192))
        self.assertEqual(len(harness.public_catalog_from_lists([raw])), 8192)
        with self.assertRaises(ValueError):
            harness.public_catalog_from_lists([raw + b'/bin/new\n'])
        self.assertEqual(harness.public_catalog_from_lists([b'/usr/bin/publictool\r\n']), ())
        with self.assertRaises(ValueError):
            harness.public_diagnostic_context(self.catalog, {str(i): 'x' for i in range(8193)}, self.identity)
        with self.assertRaises(ValueError):
            harness.public_diagnostic_context(self.catalog, {'x': 'x' * 2097153}, self.identity)
        exact = b''.join(('/bin/a%09x\n' % i).encode() for i in range(8192))
        self.assertEqual(len(exact), 131072)
        self.assertEqual(len(harness.public_catalog_from_lists([exact])), 8192)
        with self.assertRaises(ValueError):
            harness.public_catalog_from_lists([exact.replace(b'/bin/a000000000\n', b'/bin/aa000000000\n')])
        file_max = b'x' * 1048575 + b'\n'
        self.assertEqual(harness.public_catalog_from_lists([file_max] * 32), ())
        with self.assertRaises(ValueError):
            harness.public_catalog_from_lists([file_max] * 32 + [b'\n'])

    def test_candidate_and_expanded_receipt_token_budgets_and_loop(self):
        with tempfile.TemporaryDirectory() as folder:
            links = [Path(folder) / str(i) for i in range(10)]
            for index in range(9):
                links[index].symlink_to(links[index + 1])
            self.assertEqual(harness.opaque_utility_closure(str(links[0]))['status'], 'limit')
            links[9].symlink_to(links[8])
            self.assertEqual(harness.opaque_utility_closure(str(links[8]))['status'], 'loop')
        actual = {'unknown' + str(i): '/private/' + str(i) for i in range(40)}
        candidate = harness.opaque_utility_observation(Path('/absent'), actual, {}, 40, tuple(actual), self.identity)
        self.assertEqual(len(candidate['entries']) + candidate['omitted'], 40)
        self.assertGreater(candidate['omitted'], 0)
        raw = harness.encode_opaque_observation(candidate)
        context = harness.public_diagnostic_context(self.catalog, actual, self.identity)
        receipt = harness.resolve_owned_utility_output(b'', raw, context)[2]
        self.assertIsNotNone(receipt)
        self.assertLessEqual(len(harness.diagnostic_json(receipt)) + 1, 16384)
        # Structural metadata now also consumes global32; force the independent
        # byte bound regardless of the smaller admitted original row count.
        candidate['entries'] *= 100
        with self.assertRaises(ValueError):
            harness.encode_opaque_observation(candidate)

    def test_catalog_unavailability_context_failure_and_token_ambiguity_omit(self):
        with mock.patch.object(harness, 'read_public_utility_catalog', return_value=None):
            self.assertIsNone(harness.prepare_public_diagnostic_context(Path('/absent'), self.identity))
        value = self.observation()
        key = harness.opaque_utility_token('/usr/bin/publictool')
        self.context['paths'][(key['sha256'], key['utf8_bytes'])] = {'/usr/bin/publictool', '/bin/another'}
        receipt = harness.resolve_owned_utility_output(b'', harness.encode_opaque_observation(value), self.context)[2]
        self.assertNotIn('path', receipt['entries'][1]['actual']['target'] if receipt['entries'][1]['tool'].get('basename') == 'publictool'
                         else receipt['entries'][0]['actual']['target'])

    def test_expanded_receipt_recounts_bytes_and_extra_tokens_reject(self):
        paths = tuple('/usr/bin/a' + str(i).zfill(3) + 'x' * 80 for i in range(60))
        actual = {path.rsplit('/', 1)[1]: path for path in paths}
        context = harness.public_diagnostic_context(paths, actual, self.identity)
        observation = harness.opaque_utility_observation(Path('/absent'), actual, {}, len(actual), tuple(actual), self.identity)
        raw = harness.encode_opaque_observation(observation)
        receipt = harness.resolve_owned_utility_output(b'', raw, context)[2]
        self.assertIsNotNone(receipt)
        self.assertLessEqual(len(harness.diagnostic_json(receipt)) + 1, 16384)
        self.assertEqual(len(receipt['entries']) + receipt['omitted'], 60)
        many = self.observation()
        many['entries'] = [many['entries'][0]] * 17  # 2 tokens per entry exceeds global32.
        self.assertIsNone(harness.resolve_owned_utility_output(b'', harness.encode_opaque_observation(many), self.context)[2])

    def test_original_mismatch_seam_emits_opaque_cached_short_circuit_once(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            base = harness.materialize_pinned_git(root)
            (base / 'util').chmod(0o755)
            (base / 'util/publictool').write_bytes(b'content must not be read')
            (base / 'util').chmod(0o555)
            actual = {p.name: str(p.readlink()) for p in (base / 'util').iterdir() if p.is_symlink()}
            context = harness.public_diagnostic_context(self.catalog, actual, self.identity)
            output = io.StringIO()
            with mock.patch.dict(os.environ, {'F13_HARNESS_IDENTITY': json.dumps(self.identity)}), \
                 mock.patch.object(harness, 'expected_utilities', side_effect=RuntimeError('must not evaluate')) as expected, \
                 contextlib.redirect_stderr(output):
                with self.assertRaisesRegex(harness.SetupFailure, '^pinned_Git_private_utility_view$'):
                    harness.verify_pinned_git(root)
                self.assertEqual(expected.call_count, 0)
            _, stderr, receipt = harness.resolve_owned_utility_output(b'', output.getvalue().encode(), context)
            self.assertIsNotNone(receipt)
            self.assertEqual(receipt['expected_status'], 'not_evaluated_short_circuit')
            self.assertEqual(receipt['entries'][0]['tool']['basename'], 'publictool')
            self.assertNotIn('actual', receipt['entries'][0], 'regular slot is not part of the cached symlink map')
            self.assertNotIn(b'F13_OPAQUE_UTILITY_', stderr)
            self.assertIn(b'F13_UTILITY_VIEW_DIAGNOSTIC_v1', stderr)

    def test_catalog_handles_closed_before_existing_spawn_even_on_acl_error(self):
        before = set(os.listdir('/proc/self/fd'))
        harness.read_public_utility_catalog()
        self.assertEqual(set(os.listdir('/proc/self/fd')), before)
        with mock.patch.object(harness.os, 'getxattr', side_effect=OSError(95, 'unknown_acl_semantics')):
            self.assertIsNone(harness.read_public_utility_catalog())
        self.assertEqual(set(os.listdir('/proc/self/fd')), before)


class HistoricalJTransport(unittest.TestCase):
    def test_utility_mismatch_emits_redacted_diagnostic_without_changing_failure(self):
        # Synthetic output/invariance regression, NOT the actual hosted cause.
        with tempfile.TemporaryDirectory(prefix='f13-diag-') as folder:
            root = Path(folder)
            base = harness.materialize_pinned_git(root)
            link = base / 'util/sha256sum'
            (base / 'util').chmod(0o755)
            link.unlink()
            secret = '/home/private/HOST_SECRET_SENTINEL'
            link.symlink_to(secret)
            (base / 'util').chmod(0o555)
            before = {p.name: str(p.readlink()) for p in (base / 'util').iterdir()}
            output = io.StringIO()
            with contextlib.redirect_stderr(output):
                with self.assertRaisesRegex(harness.SetupFailure, '^pinned_Git_private_utility_view$'):
                    harness.verify_pinned_git(root)
            receipts = [json.loads(line) for line in output.getvalue().splitlines()]
            self.assertEqual(len(receipts), 1, 'original mismatch must emit a bounded diagnostic')
            self.assertEqual(receipts[0]['kind'], 'F13_UTILITY_VIEW_DIAGNOSTIC_v1')
            self.assertNotIn(secret, output.getvalue())
            self.assertEqual(before, {p.name: str(p.readlink()) for p in (base / 'util').iterdir()})

    def test_utility_success_is_silent_and_empty_metadata_is_bounded(self):
        with tempfile.TemporaryDirectory(prefix='f13-success-') as folder:
            root = Path(folder)
            base = harness.materialize_pinned_git(root)
            output = io.StringIO()
            with mock.patch.object(harness, 'expected_utilities', wraps=harness.expected_utilities) as expected:
                with contextlib.redirect_stderr(output):
                    harness.verify_pinned_git(root)
                self.assertEqual(expected.call_count, 1)
            self.assertEqual(output.getvalue(), '')
            empty = harness.utility_view_diagnostic(base, {}, {}, 0)
            self.assertEqual(empty['counts'], dict(expected=0, actual=0, entries=0, non_symlink=0,
                                                  missing=0, extra=0, changed=0))
            self.assertEqual(empty['entries'], [])
            self.assertFalse(empty['truncated'])

    def test_non_symlink_short_circuit_and_diagnostic_errors_preserve_failure(self):
        with tempfile.TemporaryDirectory(prefix='f13-short-') as folder:
            root = Path(folder)
            base = harness.materialize_pinned_git(root)
            (base / 'util').chmod(0o755)
            regular = base / 'util/UNKNOWN_SECRET_NAME'
            regular.write_bytes(b'synthetic private content must not be read')
            (base / 'util').chmod(0o555)
            output = io.StringIO()
            with mock.patch.object(harness, 'expected_utilities', side_effect=RuntimeError('must remain unevaluated')) as expected:
                with contextlib.redirect_stderr(output):
                    with self.assertRaisesRegex(harness.SetupFailure, '^pinned_Git_private_utility_view$'):
                        harness.verify_pinned_git(root)
                self.assertEqual(expected.call_count, 0)
            value = json.loads(output.getvalue())
            self.assertEqual(value['expected_status'], 'not_evaluated_short_circuit')
            self.assertIsNone(value['counts']['expected'])
            self.assertEqual(value['counts']['non_symlink'], 1)
            self.assertEqual(value['entries'][0]['difference'], 'not_in_actual_symlink_snapshot')
            self.assertFalse(value['entries'][0]['slot']['is_symlink'])
            self.assertNotIn('UNKNOWN_SECRET_NAME', output.getvalue())
            output = io.StringIO()
            with mock.patch.object(harness, 'utility_view_diagnostic', side_effect=OSError('DO_NOT_LEAK_EXCEPTION_SECRET')):
                with contextlib.redirect_stderr(output):
                    with self.assertRaisesRegex(harness.SetupFailure, '^pinned_Git_private_utility_view$'):
                        harness.verify_pinned_git(root)
            self.assertEqual(json.loads(output.getvalue())['status'], 'diagnostic_unavailable')
            self.assertNotIn('DO_NOT_LEAK', output.getvalue())
            self.assertEqual(regular.read_bytes(), b'synthetic private content must not be read')

    def test_utility_metadata_redacts_unknown_names_paths_and_environment(self):
        # Deliberately hostile synthetic logging inputs, not hosted mappings.
        secret = 'SECRET_SENTINEL_token_123'
        actual = {'python3': '/home/' + secret, secret + '\n\0': '/usr/bin/python3/../' + secret}
        expected = {'python3': '/usr/bin/python3', 'sha256sum': '/usr/bin/sha256sum'}
        before = json.dumps([actual, expected], sort_keys=True)
        output = io.StringIO()
        with mock.patch.dict(os.environ, {'AWS_SECRET_ACCESS_KEY': secret}):
            with mock.patch.object(os, 'getgroups', return_value=[0, 1000, secret]):
                with contextlib.redirect_stderr(output):
                    harness.emit_utility_view_diagnostic(Path('/nonexistent'), actual, expected, 2)
        raw = output.getvalue()
        self.assertNotIn(secret, raw)
        self.assertNotIn('AWS_', raw)
        value = json.loads(raw)
        self.assertEqual(value['groups'], [0, 1000])
        self.assertEqual(value['groups_invalid'], 1)
        self.assertEqual(value['counts']['missing'], 1)
        self.assertEqual(value['counts']['extra'], 1)
        self.assertEqual(value['counts']['changed'], 1)
        self.assertEqual(harness.utility_diagnostic_token('/usr/bin/python3'), {'root': '/usr/bin', 'relative': 'python3'})
        for path in ('/usr/bin/../' + secret, '/usr/bin/python3/' + secret, '/opt/' + secret, secret, ''):
            self.assertEqual(set(harness.utility_diagnostic_token(path)), {'sha256', 'utf8_bytes'})
        self.assertEqual(json.dumps([actual, expected], sort_keys=True), before)

    def test_utility_diagnostic_truncation_entry_and_byte_bounds(self):
        secret = 'MUST_NOT_APPEAR'
        actual = {secret + str(i): '/private/' + secret + ('x' * 5000) for i in range(1000)}
        expected = {secret + str(i): '/other/' + secret + ('y' * 5000) for i in range(1000)}
        output = io.StringIO()
        with contextlib.redirect_stderr(output):
            harness.emit_utility_view_diagnostic(Path('/absent'), actual, expected, 1000)
        self.assertLessEqual(len(output.getvalue().encode()), harness.UTILITY_DIAGNOSTIC_BYTES)
        self.assertNotIn(secret, output.getvalue())
        value = json.loads(output.getvalue())
        self.assertLessEqual(len(value['entries']), harness.UTILITY_DIAGNOSTIC_ENTRIES)
        self.assertTrue(value['truncated'])
        self.assertEqual(value['omitted'] + len(value['entries']), 1000)

    def test_utility_access_metadata_does_not_expose_paths_or_contents(self):
        with tempfile.TemporaryDirectory(prefix='f13-access-') as folder:
            path = Path(folder) / 'SECRET_FILENAME'
            content = b'SECRET_FILE_CONTENT'
            path.write_bytes(content)
            path.chmod(0o600)
            link = Path(folder) / 'link'
            link.symlink_to(path)
            value = harness.utility_diagnostic_access(str(link))
            self.assertTrue(value['is_symlink'])
            self.assertTrue(value['is_file'])
            self.assertFalse(value['executable'])
            self.assertNotIn('SECRET', json.dumps(value))
            self.assertEqual(path.read_bytes(), content)
            self.assertEqual(harness.utility_diagnostic_access('relative/secret'), {'status': 'not_inspected'})

    def test_utility_predicate_exception_and_output_error_order_unchanged(self):
        with tempfile.TemporaryDirectory(prefix='f13-order-') as folder:
            root = Path(folder)
            base = harness.materialize_pinned_git(root)
            output = io.StringIO()
            with mock.patch.object(harness, 'expected_utilities', side_effect=RuntimeError('original_error')):
                with contextlib.redirect_stderr(output):
                    with self.assertRaisesRegex(RuntimeError, '^original_error$'):
                        harness.verify_pinned_git(root)
            self.assertEqual(output.getvalue(), '')
            (base / 'util').chmod(0o755)
            (base / 'util/regular').write_bytes(b'no mutation')
            (base / 'util').chmod(0o555)
            class BrokenOutput(io.StringIO):
                def write(self, text):
                    raise OSError('private output error')
            with contextlib.redirect_stderr(BrokenOutput()):
                with self.assertRaisesRegex(harness.SetupFailure, '^pinned_Git_private_utility_view$'):
                    harness.verify_pinned_git(root)

    def test_actual_materializer_installs_pinned_Git(self):
        with tempfile.TemporaryDirectory(prefix='f13-pinned-') as folder:
            root = Path(folder)
            harness.materialize(root)
            binary = root / 'historical-git/bin/git'
            self.assertTrue(binary.is_file(), 'actual materializer did not install pinned historical Git')
            self.assertEqual(hashlib.sha256(binary.read_bytes()).hexdigest(),
                             '2540879925a6881e3877ff7e3330746ba3027b04edf16a3a12dccd1644c4f32d')

    def test_real_loader_argv0_stdout_stderr_and_exit(self):
        self.assertNotEqual(os.getuid(), 0, 'component execution must be unprivileged Linux')
        with tempfile.TemporaryDirectory(prefix='f13-loader-') as folder:
            root = Path(folder)
            base = harness.materialize_pinned_git(root)
            env = harness.pinned_git_environment(root)
            result = subprocess.run(['git', '--version'], env=env, capture_output=True)
            self.assertEqual((result.returncode, result.stdout, result.stderr), (0, b'git version 2.39.5\n', b''))
            for name, args in [('git', ['-C', str(root), 'show', 'no-such-object']),
                               ('git-index-pack', ['--no-such-option']), ('git-rev-list', ['--no-such-option'])]:
                entry = base / ('launch' if name == 'git' else 'exec') / name
                actual = subprocess.run([str(entry), *args], env=env, capture_output=True)
                direct = subprocess.run([str(base / 'lib/ld-linux-x86-64.so.2'), '--library-path', str(base / 'lib'),
                                         '--argv0', name, str(base / 'bin/git-elf'), *args], env=env, capture_output=True)
                self.assertEqual((actual.returncode, actual.stdout, actual.stderr),
                                 (direct.returncode, direct.stdout, direct.stderr), name)
                self.assertNotEqual(actual.returncode, 0)

    def test_unknown_git_routes_cannot_fall_back_to_host(self):
        with tempfile.TemporaryDirectory(prefix='f13-fallback-') as folder:
            root = Path(folder) / 'private'
            root.mkdir()
            host = Path(folder) / 'fake-host'
            host.mkdir()
            marker = Path(folder) / 'forbidden-host-executed'
            for name in ('git', 'git-unknown-child', 'git-upload-pack'):
                file = host / name
                file.write_text('#!/bin/sh\ntouch ' + str(marker) + '\nexit 0\n')
                file.chmod(0o755)
            with mock.patch.object(harness, 'UTILITY_DIRS', (str(host), *harness.UTILITY_DIRS)):
                base = harness.materialize_pinned_git(root)
                env = harness.pinned_git_environment(root)
                self.assertFalse(any(p.name == 'git' or p.name.startswith('git-') for p in (base / 'util').iterdir()))
                for args in (['unknown-child'], ['--exec-path=' + str(host), '--version'], ['clone', 'https://invalid.example', 'bad']):
                    result = subprocess.run(['git', *args], env=env, capture_output=True)
                    self.assertEqual(result.returncode, 125)
                    self.assertIn(b'F13_PINNED_GIT_SETUP_FAILURE', result.stderr)
                for change in ({'GIT_EXEC_PATH': str(host)}, {'PATH': env['PATH'] + ':' + str(host)},
                               {'GIT_CONFIG_COUNT': '1', 'GIT_CONFIG_KEY_0': 'alias.version', 'GIT_CONFIG_VALUE_0': '!false'},
                               {'LD_LIBRARY_PATH': str(host)}):
                    result = subprocess.run([str(base / 'launch/git'), '--version'], env=dict(env, **change), capture_output=True)
                    self.assertEqual(result.returncode, 125)
                self.assertFalse(marker.exists())

    def test_same_size_binary_and_all_library_corruption_reject_before_launch(self):
        with tempfile.TemporaryDirectory(prefix='f13-forged-') as folder:
            root = Path(folder)
            base = harness.materialize_pinned_git(root)
            for name in harness.pinned_manifest():
                with self.subTest(name=name):
                    path = base / name
                    raw = path.read_bytes()
                    path.chmod(0o755)
                    forged = bytearray(raw)
                    forged[-1] ^= 1
                    path.write_bytes(forged)
                    path.chmod(0o555)
                    with self.assertRaisesRegex(harness.SetupFailure, 'pinned_Git_bytes'):
                        harness.pinned_git_environment(root)
                    path.chmod(0o755)
                    path.write_bytes(raw)
                    path.chmod(0o555)
            harness.pinned_git_environment(root)

    def test_forged_path_mode_and_inherited_context_reject(self):
        with tempfile.TemporaryDirectory(prefix='f13-path-') as folder:
            root = Path(folder)
            base = harness.materialize_pinned_git(root)
            binary = base / 'bin/git'
            binary.chmod(0o755)
            with self.assertRaisesRegex(harness.SetupFailure, 'pinned_Git_mode'):
                harness.pinned_git_environment(root)
            binary.chmod(0o555)
            (base / 'bin').chmod(0o755)
            binary.rename(base / 'bin/saved-git')
            binary.symlink_to(base / 'bin/saved-git')
            with self.assertRaisesRegex(harness.SetupFailure, 'pinned_Git_path'):
                harness.pinned_git_environment(root)
            binary.unlink()
            (base / 'bin/saved-git').rename(binary)
            (base / 'bin').chmod(0o555)
            for change in ({'GIT_CONFIG_COUNT': '1'}, {'LD_PRELOAD': '/wrong.so'}, {'GIT_EXEC_PATH': '/host/git'}):
                with mock.patch.dict(os.environ, change):
                    with self.assertRaises(harness.SetupFailure):
                        harness.pinned_git_environment(root)

    def test_archive_manifest_forgery_rejects_without_extraction(self):
        with tempfile.TemporaryDirectory(prefix='f13-archive-') as folder:
            fixture = Path(folder) / 'fixture'
            fixture.mkdir()
            for name in ('git-2.39.5-data.tar.gz', 'git-2.39.5-data-manifest.json'):
                shutil.copyfile(harness.FIXTURE / name, fixture / name)
            for name, reason in [('git-2.39.5-data.tar.gz', 'pinned_Git_archive'),
                                 ('git-2.39.5-data-manifest.json', 'pinned_Git_manifest')]:
                original = (fixture / name).read_bytes()
                forged = bytearray(original)
                forged[-1] ^= 1
                (fixture / name).write_bytes(forged)
                root = Path(folder) / ('private-' + reason)
                root.mkdir()
                with mock.patch.object(harness, 'FIXTURE', fixture):
                    with self.assertRaisesRegex(harness.SetupFailure, reason):
                        harness.materialize_pinned_git(root)
                self.assertFalse((root / 'historical-git').exists())
                (fixture / name).write_bytes(original)

    def test_captured_bundle_clone_child_routing_and_precheckout_rejections(self):
        with tempfile.TemporaryDirectory(prefix='f13-child-') as folder:
            root = Path(folder)
            harness.materialize(root)
            env = harness.pinned_git_environment(root)
            repo = root / 'repo'
            result = subprocess.run(['git', 'clone', '--no-checkout', str(root / 'out/prelaunch/private-target.bundle'), str(repo)],
                                    env=env, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr.decode())
            with mock.patch.dict(os.environ, env, clear=True):
                harness.verify_checkout_inputs(repo, harness.TARGET)
                parents = harness.run(['git', '-C', str(repo), 'rev-list', '--parents', '-n', '1', harness.TARGET]).decode().split()
                self.assertEqual(parents, [harness.TARGET, harness.BASE, harness.SOURCE])
                config = repo / '.git/config'
                original = config.read_bytes()
                config.write_bytes(original + b'\n[alias]\nversion = !false\n')
                with self.assertRaisesRegex(harness.SetupFailure, 'pinned_Git_unsafe_config:alias.version'):
                    harness.verify_checkout_inputs(repo, harness.TARGET)
                config.write_bytes(original)
                hooks = repo / '.git/hooks'
                hooks.mkdir(exist_ok=True)
                (hooks / 'post-checkout').write_text('#!/bin/sh\nexit 0\n')
                with self.assertRaisesRegex(harness.SetupFailure, 'pinned_Git_hooks_absent'):
                    harness.verify_checkout_inputs(repo, harness.TARGET)
            self.assertFalse((repo / 'scripts').exists(), 'component tests never checkout or run the historical helper')

    def test_capability_diagnostic_preserves_every_field_and_raw_inputs(self):
        captured = {'git_version': 'git version 2.39.5', 'native': {'paths': ['a'], 'admission': False}, 'ledger': {'sha': 'abc'}}
        before = json.dumps(captured, sort_keys=True)
        self.assertEqual(harness.capability_differences(captured, captured), [])
        changed = json.loads(before)
        changed['native']['paths'][0] = 'foreign'
        changed['ledger']['sha'] = 'wrong'
        changed['git_version'] = 'git version 2.55.0'
        self.assertEqual(harness.capability_differences(captured, changed), ['$.git_version', '$.ledger.sha', '$.native.paths[0]'])
        self.assertEqual(json.dumps(captured, sort_keys=True), before)

    def test_actual_archive_restores_original_J_record_path(self):
        with tempfile.TemporaryDirectory(prefix='f13-transport-') as folder:
            root = Path(folder)
            harness.materialize(root)
            captured = root / 'out/actual/observed/fixed3071-J-ledger.json'
            original = root / 'tmp/F13-author-547/fixed3071-J-ledger.json'
            self.assertTrue(original.is_file(), 'original literal J-ledger consumer path was not materialized')
            raw = original.read_bytes()
            self.assertEqual(len(raw), 109549632)
            self.assertEqual(hashlib.sha256(raw).hexdigest(), harness.LEDGER_SHA)
            self.assertEqual(raw, captured.read_bytes())

    def test_record_alias_rejects_wrong_declared_binding(self):
        with tempfile.TemporaryDirectory(prefix='f13-transport-') as folder:
            root = Path(folder)
            for binding in (None, ('0' * 64, 109549632), (harness.LEDGER_SHA, 0)):
                with self.subTest(binding=binding):
                    logical = {} if binding is None else {harness.J_CAPTURE: binding}
                    with self.assertRaisesRegex(harness.SetupFailure, 'J_record_declared_binding'):
                        harness.materialize_J_record_alias(root, logical)
                    self.assertEqual(list(root.iterdir()), [])

    def test_record_alias_rejects_forged_captured_bytes(self):
        with tempfile.TemporaryDirectory(prefix='f13-transport-') as folder:
            root = Path(folder)
            source = root / harness.J_CAPTURE.lstrip('/')
            source.parent.mkdir(parents=True)
            with zipfile.ZipFile(harness.FIXTURE / 'historical-inputs.zip') as archive:
                forged = bytearray(archive.read('objects/' + harness.LEDGER_SHA))
            self.assertEqual(len(forged), harness.J_RECORD_BYTES)
            forged[-1] ^= 1  # Same-length actual record mutation must hit SHA, not size.
            self.assertNotEqual(hashlib.sha256(forged).hexdigest(), harness.LEDGER_SHA)
            source.write_bytes(forged)
            with self.assertRaisesRegex(harness.SetupFailure, 'J_record_raw_binding'):
                harness.materialize_J_record_alias(root, {harness.J_CAPTURE: (harness.LEDGER_SHA, 109549632)})
            self.assertEqual(source.read_bytes(), forged)
            self.assertFalse((root / harness.J_ORIGINAL_RECORD.lstrip('/')).exists())

    def test_record_alias_never_overwrites_existing_target(self):
        with tempfile.TemporaryDirectory(prefix='f13-transport-') as folder:
            root = Path(folder)
            source, target = root / harness.J_CAPTURE.lstrip('/'), root / harness.J_ORIGINAL_RECORD.lstrip('/')
            source.parent.mkdir(parents=True)
            source.write_bytes(b'rejection-only source control')
            target.parent.mkdir(parents=True)
            target.write_bytes(b'prior target must stay byte-exact')
            with self.assertRaisesRegex(harness.SetupFailure, 'J_record_no_overwrite'):
                harness.materialize_J_record_alias(root, {harness.J_CAPTURE: (harness.LEDGER_SHA, 109549632)})
            self.assertEqual(target.read_bytes(), b'prior target must stay byte-exact')


if __name__ == '__main__':
    unittest.main()
