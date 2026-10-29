"""Source-bound Space native installer identity; no deployment or DB access.

The source inventory helper validates bytes but does not authenticate its input.
Production admission must obtain source from the protected canonical package.
"""
import hashlib
from pathlib import Path
import re

import migrations


class ToolIdentityError(RuntimeError): pass


def source_inventory(root,snapshot):
    """Cross-check the fixed installer inputs against a verified source snapshot."""
    try:
        root=Path(root); files=snapshot['files']; source=snapshot['source_sha']
        if (not isinstance(source,str) or not re.fullmatch(r'[0-9a-f]{40}',source) or source=='0'*40
            or not isinstance(files,dict)): raise ValueError()
        def read(relative):
            path=root/relative
            if path.is_symlink() or not path.is_file() or not path.resolve().is_relative_to(root.resolve()): raise ValueError()
            raw=path.read_bytes()
            if hashlib.sha256(raw).hexdigest()!=files.get(relative): raise ValueError()
            return raw
        backend=root/'src/backend'
        gos={p.relative_to(root).as_posix() for p in backend.rglob('*.go') if not p.name.endswith('_test.go')}
        expected_gos={p for p in files if p.startswith('src/backend/') and p.endswith('.go') and not p.endswith('_test.go')}
        if not gos or gos!=expected_gos: raise ValueError()
        for name in gos: read(name)
        sql_dir=root/'src/backend/migrations/space_db'
        sql_paths={p.relative_to(root).as_posix() for p in sql_dir.glob('*.sql')}
        expected_sql={p for p in files if p.startswith('src/backend/migrations/space_db/') and p.endswith('.sql')}
        if not sql_paths or sql_paths!=expected_sql: raise ValueError()
        sql={Path(path).name:hashlib.sha256(read(path)).hexdigest() for path in sql_paths}
        if len(sql)!=len(sql_paths): raise ValueError()
        manifest_path='src/backend/space/internal/nativeinstaller/source_manifest.go'
        manifest=read(manifest_path).decode('utf-8')
        found=re.findall(r'^\s*"([^"/]+\.sql)":\s*"([a-f0-9]{64})",\s*$',manifest,re.M)
        inventory=re.findall(r'^const InventorySHA256 = "([a-f0-9]{64})"$',manifest,re.M)
        if len(inventory)!=1 or len(found)!=len(dict(found)) or dict(found)!=sql: raise ValueError()
        dockerfile='src/backend/space/Dockerfile.native-migrate'; read(dockerfile)
        return {'source_sha':source,'build_source_sha256':migrations.space_source_digest(root),
                'sql_inventory_sha256':inventory[0],'dockerfile_sha256':files[dockerfile],
                'source_manifest_sha256':files[manifest_path],'sql_files':dict(sorted(sql.items()))}
    except Exception: raise ToolIdentityError('Space native source inventory not qualified') from None
