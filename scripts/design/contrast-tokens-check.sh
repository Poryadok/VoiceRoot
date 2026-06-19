#!/usr/bin/env bash
# WCAG AA contrast sanity check for token text/background pairs (docs/features/accessibility.md).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
TOKENS="$ROOT/design/tokens/voice.tokens.json"
python3 - "$TOKENS" <<'PY'
import json
import sys

path = sys.argv[1]
with open(path, encoding="utf-8") as f:
    data = json.load(f)

def hex_to_rgb(value: str):
    value = value.lstrip("#")
    return tuple(int(value[i : i + 2], 16) for i in (0, 2, 4))

def relative_luminance(rgb):
    def channel(c):
        c = c / 255.0
        return c / 12.92 if c <= 0.03928 else ((c + 0.055) / 1.055) ** 2.4

    r, g, b = rgb
    return 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b)

def contrast(a, b):
    l1, l2 = relative_luminance(a), relative_luminance(b)
    if l1 < l2:
        l1, l2 = l2, l1
    return (l1 + 0.05) / (l2 + 0.05)

pairs = [
    ("color.text.primary", "color.background.canvas", 4.5),
    ("color.text.primary", "color.background.surface", 4.5),
    ("color.text.secondary", "color.background.canvas", 4.5),
    ("color.text.secondary", "color.background.surface", 4.5),
    ("color.semantic.error", "color.background.canvas", 3.0),
]

themes = data.get("themes", {})
failed = []
for theme_name, theme in themes.items():
    for fg_key, bg_key, min_ratio in pairs:
        fg = theme.get(fg_key)
        bg = theme.get(bg_key)
        if not fg or not bg:
            continue
        ratio = contrast(hex_to_rgb(fg), hex_to_rgb(bg))
        if ratio < min_ratio:
            failed.append((theme_name, fg_key, bg_key, ratio, min_ratio))

if failed:
    print("contrast check failed:", file=sys.stderr)
    for item in failed:
        print(
            f"  theme={item[0]} {item[1]} on {item[2]}: {item[3]:.2f} < {item[4]}",
            file=sys.stderr,
        )
    sys.exit(1)

print(f"contrast tokens: {len(themes)} themes, {len(pairs)} pairs OK")
PY
