#!/usr/bin/env python3
"""One-off helper used to wrap RPC returns for buf STANDARD.

Do **not** re-run on protos already migrated: it would double-wrap.
"""
from __future__ import annotations

import re
import sys
from pathlib import Path


def camel_to_snake(name: str) -> str:
    s = re.sub(r"([A-Z]+)([A-Z][a-z])", r"\1_\2", name)
    s = re.sub(r"([a-z\d])([A-Z])", r"\1_\2", s)
    return s.lower()


def field_name_for_return(rtype: str) -> str:
    base = rtype.split(".")[-1]
    return camel_to_snake(base)


RPC_LINE = re.compile(
    r"^(\s*)rpc\s+(\w+)\s*\(\s*([\w.]+)\s*\)\s+returns\s+\(\s*(?:(stream)\s+)?([\w.]+)\s*\)\s*;\s*$"
)


def transform_service_block(block: str) -> tuple[str, list[str]]:
    lines = block.splitlines(keepends=True)
    out: list[str] = []
    extras: list[str] = []
    seen_resp: set[str] = set()

    for line in lines:
        m = RPC_LINE.match(line)
        if not m:
            out.append(line)
            continue
        indent, rpc_name, req, stream_kw, res_type = m.groups()
        stream_kw = stream_kw or ""

        if res_type == "google.protobuf.Empty":
            new_res = f"{rpc_name}Response"
            line = f"{indent}rpc {rpc_name}({req}) returns ({new_res});\n"
            if new_res not in seen_resp:
                extras.append(f"message {new_res} {{}}\n")
                seen_resp.add(new_res)
            out.append(line)
            continue

        if res_type == f"{rpc_name}Response":
            out.append(line)
            continue

        new_res = f"{rpc_name}Response"
        field = field_name_for_return(res_type)
        if stream_kw:
            line = f"{indent}rpc {rpc_name}({req}) returns (stream {new_res});\n"
            body = f"  {res_type} {field} = 1;\n"
        else:
            line = f"{indent}rpc {rpc_name}({req}) returns ({new_res});\n"
            body = f"  {res_type} {field} = 1;\n"

        if new_res not in seen_resp:
            extras.append(f"message {new_res} {{\n{body}}}\n")
            seen_resp.add(new_res)
        out.append(line)

    return "".join(out), extras


def process_file(path: Path) -> bool:
    rel = path.as_posix()
    if rel.endswith("voice/auth/v1/auth.proto") or rel.endswith("voice/analytics/v1/analytics.proto"):
        return False
    text = path.read_text(encoding="utf-8")
    if "service " not in text:
        return False

    m = re.search(r"(service\s+\w+\s*\{)", text)
    if not m:
        return False

    all_extras: list[str] = []
    out_parts: list[str] = []
    pos = 0
    for sm in re.finditer(r"service\s+\w+\s*\{", text):
        start = sm.start()
        out_parts.append(text[pos:start])
        depth = 0
        i = sm.start()
        while i < len(text):
            if text[i] == "{":
                depth += 1
            elif text[i] == "}":
                depth -= 1
                if depth == 0:
                    i += 1
                    break
            i += 1
        block = text[sm.start() : i]
        new_block, extras = transform_service_block(block)
        out_parts.append(new_block)
        all_extras.extend(extras)
        pos = i

    out_parts.append(text[pos:])
    new_text = "".join(out_parts)

    if new_text == text and not all_extras:
        return False

    if "google.protobuf.Empty" not in new_text:
        new_text = re.sub(
            r'^import\s+"google/protobuf/empty\.proto";\s*\n',
            "",
            new_text,
            flags=re.MULTILINE,
        )

    # Dedupe extra messages by name (first occurrence wins)
    deduped: list[str] = []
    seen: set[str] = set()
    for chunk in all_extras:
        name_m = re.match(r"message\s+(\w+)\s", chunk)
        if name_m and name_m.group(1) in seen:
            continue
        if name_m:
            seen.add(name_m.group(1))
        deduped.append(chunk)

    new_text = new_text.rstrip() + "\n\n" + "".join(deduped).rstrip() + "\n"
    path.write_text(new_text, encoding="utf-8")
    return True


def main() -> int:
    root = Path(__file__).resolve().parents[1] / "protos" / "voice"
    n = 0
    for p in sorted(root.rglob("*.proto")):
        if "events" in str(p):
            continue
        try:
            if process_file(p):
                print(p.relative_to(root.parents[1]))
                n += 1
        except Exception as e:
            print("FAIL", p, e, file=sys.stderr)
            return 1
    print("updated", n, "files")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
