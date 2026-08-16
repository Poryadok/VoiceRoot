#!/usr/bin/env bash
# A11y web landmark contract (Axe analog) — docs/features/accessibility.md §Тестирование.
# Inventory A11Y-03: beyond Flutter widget semantics.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
FIXTURE="$ROOT/scripts/ci/a11y-web/fixtures/voice-a11y-landmarks.html"
python3 - "$FIXTURE" <<'PY'
import html.parser
import sys
from pathlib import Path

path = Path(sys.argv[1])
raw = path.read_text(encoding="utf-8")


class A11yCollector(html.parser.HTMLParser):
    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.landmarks = []  # (tag, aria_label)
        self.buttons = []  # aria-label or text
        self.images = []  # alt
        self.live_regions = []  # (aria-live, aria-relevant)
        self.inputs = []  # (id, has_label)
        self.labels_for = set()
        self.headings = []  # (level, text)
        self._capture_heading = None
        self._heading_buf = []
        self._button_stack = []
        self.errors = []

    def handle_starttag(self, tag, attrs):
        attrs = dict(attrs)
        role = attrs.get("role", "")
        label = attrs.get("aria-label", "")
        if tag in {"nav", "main", "section"} or role in {
            "navigation",
            "main",
            "region",
        }:
            self.landmarks.append((tag if tag != "div" else role, label))
        if tag == "button":
            self._button_stack.append({"label": label, "text": []})
        if tag == "img":
            self.images.append(attrs.get("alt", ""))
        if "aria-live" in attrs:
            self.live_regions.append(
                (attrs.get("aria-live", ""), attrs.get("aria-relevant", ""))
            )
        if tag == "label" and "for" in attrs:
            self.labels_for.add(attrs["for"])
        if tag == "input":
            self.inputs.append(attrs.get("id", ""))
        if tag in {"h1", "h2", "h3", "h4", "h5", "h6"}:
            self._capture_heading = int(tag[1])
            self._heading_buf = []

    def handle_endtag(self, tag):
        if tag == "button" and self._button_stack:
            btn = self._button_stack.pop()
            text = "".join(btn["text"]).strip()
            self.buttons.append(btn["label"] or text)
        if tag in {"h1", "h2", "h3", "h4", "h5", "h6"} and self._capture_heading:
            self.headings.append(
                (self._capture_heading, "".join(self._heading_buf).strip())
            )
            self._capture_heading = None
            self._heading_buf = []

    def handle_data(self, data):
        if self._button_stack:
            self._button_stack[-1]["text"].append(data)
        if self._capture_heading is not None:
            self._heading_buf.append(data)


collector = A11yCollector()
collector.feed(raw)
collector.close()

failures = []

landmark_labels = {label for _, label in collector.landmarks if label}
for required in ("Navigation", "Chat list", "Conversation"):
    if required not in landmark_labels:
        failures.append(f"missing landmark aria-label={required!r}")

if not any(tag == "nav" or tag == "navigation" for tag, _ in collector.landmarks):
    failures.append("missing <nav> / navigation landmark")
if not any(tag == "main" for tag, _ in collector.landmarks):
    failures.append("missing <main> landmark")

if not any(live == "polite" for live, _ in collector.live_regions):
    failures.append("missing aria-live=polite region for new messages")

button_labels = [b for b in collector.buttons if b]
for required in ("Chats", "Friends", "Open settings", "Send"):
    if not any(required.lower() in b.lower() for b in button_labels):
        failures.append(f"missing labeled control containing {required!r}")

if not any(alt.strip() for alt in collector.images):
    failures.append("avatar/image missing non-empty alt text")

for input_id in collector.inputs:
    if not input_id:
        failures.append("composer input missing id")
    elif input_id not in collector.labels_for:
        failures.append(f"input id={input_id!r} has no associated <label for>")

if not any(level == 1 for level, _ in collector.headings):
    failures.append("missing h1 heading")

# Visible focus rule documented in accessibility.md — fixture must declare :focus-visible.
if ":focus-visible" not in raw:
    failures.append("fixture CSS missing :focus-visible outline rule")

if failures:
    print("a11y-web-axe: FAIL", file=sys.stderr)
    for item in failures:
        print(f"  - {item}", file=sys.stderr)
    sys.exit(1)

print(
    f"a11y-web-axe: OK ({path.name}; "
    f"{len(collector.landmarks)} landmarks, "
    f"{len(button_labels)} labeled controls, "
    f"{len(collector.live_regions)} live regions)"
)
PY
