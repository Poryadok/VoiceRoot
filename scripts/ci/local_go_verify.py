"""Validate Go's module verifier, including its local module-path edge case."""
import json
from pathlib import PurePosixPath, PureWindowsPath


def is_local_failure(code, stdout, stderr, windows=False):
    # Go 1.26.10's verify command attempts a cache path for this local-only
    # module, whose first path element has no dot. CachePath rejects it and
    # verifyMod consequently tries to open the literal path "hash". Accept
    # only this exact diagnostic and a separately inspected local replacement.
    suffix = (b'The system cannot find the file specified.' if windows
              else b'no such file or directory')
    expected = b'voice/backend/pkg v0.0.0: missing ziphash: open hash: ' + suffix
    return code == 1 and not stdout.strip() and stderr.strip() == expected


def verify_result(code, stdout, stderr, local_module, expected_dir, windows=False):
    if code == 0 and stdout.strip() == b'all modules verified' and not stderr.strip():
        return
    if not is_local_failure(code, stdout, stderr, windows):
        raise ValueError('Go module cache verification failed')
    try:
        module = json.loads(local_module)
        replacement = module['Replace']
        path_type = PureWindowsPath if windows else PurePosixPath
        expected_path = path_type(expected_dir)
        if (module['Path'] != 'voice/backend/pkg' or module['Version'] != 'v0.0.0' or
                replacement['Path'] != '../pkg' or replacement.get('Version') or
                path_type(module['Dir']) != expected_path or
                path_type(replacement['Dir']) != expected_path or
                path_type(replacement['GoMod']) != expected_path / 'go.mod'):
            raise ValueError()
    except (KeyError, TypeError, ValueError, json.JSONDecodeError):
        raise ValueError('Go local replacement verification failed') from None
