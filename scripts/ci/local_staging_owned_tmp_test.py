"""Owned temporary root admission for the staging preservation fixture."""
import os
from pathlib import Path
import stat
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import local_staging_owned_tmp as owned


@unittest.skipUnless(os.name == 'posix' and hasattr(os, 'geteuid') and os.geteuid() != 0,
                     'non-root POSIX ownership required')
class OwnedStagingRootTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name) / 'root'
        self.root.mkdir(mode=0o700)

    def test_creates_unique_private_directory_under_owned_root(self):
        first, second = owned.create(str(self.root)), owned.create(str(self.root))
        self.assertNotEqual(first, second)
        for path in (first, second):
            self.assertEqual(path.parent, self.root)
            self.assertTrue(path.name.startswith('voice-staging-pg-ci.'))
            info = path.stat()
            self.assertEqual(info.st_uid, os.geteuid())
            self.assertEqual(stat.S_IMODE(info.st_mode), 0o700)

    def test_refuses_noncanonical_symlink_missing_and_public_roots(self):
        link = Path(self.temp.name) / 'link'
        link.symlink_to(self.root, target_is_directory=True)
        for value in (str(link), str(self.root / '..' / 'root'), str(self.root / 'missing'), '/'):
            with self.subTest(value=value), self.assertRaises(ValueError):
                owned.create(value)
        self.root.chmod(0o755)
        with self.assertRaises(ValueError):
            owned.create(str(self.root))
        self.assertEqual(list(self.root.iterdir()), [])

    def test_refuses_root_and_foreign_ownership(self):
        with patch.object(owned.os, 'geteuid', return_value=0):
            with self.assertRaises(ValueError):
                owned.create(str(self.root))
        with patch.object(owned.os, 'geteuid', return_value=os.geteuid() + 10000):
            with self.assertRaises(ValueError):
                owned.create(str(self.root))
        self.assertEqual(list(self.root.iterdir()), [])

    def test_existing_name_is_never_removed_on_collision(self):
        existing = self.root / ('voice-staging-pg-ci.' + 'a' * 32)
        existing.mkdir()
        with patch.object(owned.uuid, 'uuid4') as generated:
            generated.return_value.hex = 'a' * 32
            with self.assertRaises(FileExistsError):
                owned.create(str(self.root))
        self.assertTrue(existing.is_dir())

    def test_failed_creation_settlement_removes_only_new_empty_directory(self):
        with patch.object(owned.os, 'fchmod', side_effect=OSError('chmod failed')):
            with self.assertRaisesRegex(OSError, 'chmod failed'):
                owned.create(str(self.root))
        self.assertEqual(list(self.root.iterdir()), [])

    def test_preservation_script_default_and_owned_root_dispatch(self):
        binary = Path(self.temp.name) / 'bin'
        binary.mkdir()
        log = Path(self.temp.name) / 'sudo.log'
        fake_sudo = '''#!/bin/sh
printf '%s\\n' "$*" >> "$SUDO_LOG"
[ "$1" = -n ] || exit 3
shift
case "$1" in
  true) exit 0 ;;
  mktemp) exec "$@" ;;
  chmod|sh) exit 0 ;;
  tar) cat >/dev/null; exit 0 ;;
  rm)
    shift
    [ "$1" = -rf ] && [ "$2" = -- ] || exit 4
    case "$3" in /var/tmp/voice-staging-pg-ci.*|"$OWNED_ROOT"/voice-staging-pg-ci.*) exec rm "$@" ;; *) exit 5 ;; esac ;;
  *) exit 6 ;;
esac
'''
        for name, content in {'sudo': fake_sudo, 'git': '#!/bin/sh\nexit 0\n',
                              'docker': '#!/bin/sh\nexit 0\n'}.items():
            path = binary / name
            path.write_text(content, encoding='utf-8')
            path.chmod(0o755)
        env = dict(os.environ, PATH=str(binary) + os.pathsep + os.environ['PATH'],
                   SUDO_LOG=str(log), OWNED_ROOT=str(self.root))
        script = Path(__file__).resolve().parent / 'nats-rollout-preservation_test.sh'
        for args in ([], ['--owned-temp-root', str(self.root)]):
            log.unlink(missing_ok=True)
            result = subprocess.run(['bash', str(script), *args], env=env, capture_output=True, timeout=30)
            self.assertEqual(result.returncode, 0, result.stderr)
            calls = log.read_text(encoding='utf-8')
            if args:
                self.assertNotIn('mktemp -d /var/tmp/voice-staging-pg-ci.', calls)
            else:
                self.assertIn('mktemp -d /var/tmp/voice-staging-pg-ci.', calls)
            self.assertEqual(list(self.root.iterdir()), [])

    def test_unknown_script_option_fails_before_dispatch(self):
        script = Path(__file__).resolve().parent / 'nats-rollout-preservation_test.sh'
        result = subprocess.run(['bash', str(script), '--unexpected'], capture_output=True, timeout=10)
        self.assertEqual(result.returncode, 2)


if __name__ == '__main__':
    unittest.main()
