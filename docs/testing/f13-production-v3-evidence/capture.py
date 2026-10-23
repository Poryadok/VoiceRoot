import hashlib
import json
import pathlib
import subprocess

root = pathlib.Path(r'D:/Git/Voice/.treehouse/Voice-451d38/35/Voice')
out = pathlib.Path(__file__).parent
client = root / 'src/frontend/lib/backend/spaces_client.dart'
original = client.read_bytes()
old = subprocess.check_output(['rtk', 'proxy', 'git', 'show', 'f81e989de^:src/frontend/lib/backend/spaces_client.dart'], cwd=root)
receipt = {'commit': 'f81e989de096c1af20d7f21152a0a018a19a8822', 'note': 'Controlled recapture, not the original development run. RED uses parent client with final regression; finally restores exact published working bytes.', 'runs': []}

def run(name, args):
    command = ['rtk', 'proxy', 'flutter'] + args
    result = subprocess.run(command, cwd=root / 'src/frontend', stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    path = out / (name + '.log')
    path.write_bytes(result.stdout)
    receipt['runs'].append({'name': name, 'command': command, 'exit_code': result.returncode, 'path': str(path), 'sha256': hashlib.sha256(result.stdout).hexdigest()})
    print(name, result.returncode, str(path), flush=True)
    return result.returncode

try:
    client.write_bytes(old)
    receipt['red_client_sha256'] = hashlib.sha256(old).hexdigest()
    red = run('red', ['test', '--no-pub', 'test/spaces_client_invites_test.dart', '--plain-name', 'joinByInvite posts empty body and decodes membership'])
finally:
    client.write_bytes(original)

receipt['restored_client_sha256'] = hashlib.sha256(client.read_bytes()).hexdigest()
assert client.read_bytes() == original
green = run('green', ['test', '--no-pub', 'test/spaces_client_invites_test.dart', 'test/spaces_client_tree_test.dart'])
analyze = run('analyze', ['analyze', '--no-pub', 'lib/backend/spaces_client.dart', 'test/spaces_client_invites_test.dart', 'test/support/bot_live_harness.dart'])
(out / 'receipt.json').write_text(json.dumps(receipt, indent=2) + '\n', encoding='utf-8')
assert red != 0 and green == 0 and analyze == 0
