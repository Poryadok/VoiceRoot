"""Bound both child output streams before they reach supervisor capture files."""
import os
import subprocess
import sys
import threading

LIMIT = 1024 * 1024


def _drain(stream, target, child, overflow, failure):
    try:
        while True:
            block = stream.read(65536)
            if not block:
                break
            room = LIMIT - len(target)
            target.extend(block[:max(room, 0)])
            if len(block) > room:
                overflow.set()
                try:
                    child.kill()
                except OSError:
                    pass
                break
    except BaseException:
        failure.set()
        try:
            child.kill()
        except OSError:
            pass
    finally:
        try:
            stream.close()
        except BaseException:
            failure.set()
            try:
                child.kill()
            except OSError:
                pass


def main(command):
    if not command:
        return 2
    try:
        child = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE, bufsize=0)
    except OSError:
        print('cannot start lint command', file=sys.stderr)
        return 126
    buffers = [bytearray(), bytearray()]
    overflow = threading.Event()
    failure = threading.Event()
    readers = [threading.Thread(target=_drain, args=(stream, target, child, overflow, failure), daemon=True)
               for stream, target in zip((child.stdout, child.stderr), buffers)]
    try:
        for reader in readers:
            reader.start()
        code = child.wait()
        for reader in readers:
            reader.join(3)
        if any(reader.is_alive() for reader in readers):
            os._exit(124)
        sys.stdout.buffer.write(buffers[0])
        sys.stderr.buffer.write(buffers[1])
        if failure.is_set():
            return 126
        if overflow.is_set():
            return 125
        return code
    except BaseException:
        try:
            child.kill()
        except OSError:
            pass
        return 126


if __name__ == '__main__':
    raise SystemExit(main(sys.argv[1:]))
