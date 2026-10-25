# Match history Windows golden provenance

The root `match_history_horizontal.png` and `match_history_vertical.png`
references were captured on Windows x64 (Windows build `10.0.26200`) from
source `c45c8b876af647fd9a96c6395dcf26b093a60732` on 2026-10-26.
They serve the existing non-Linux reference paths in
`test/match_history_screen_test.dart`; Linux retains its separate `linux/`
references. These captures establish Windows coverage only.

Toolchain: Flutter `3.41.7` stable, framework revision
`cc0734ac716fbb8b90f3f9db8020958b1553afa7`, Dart `3.11.5`, engine revision
`59aa584fdf100e6c78c785d8a5b565d1de4b48ab`, engine content hash
`7a53c052bc4b472cf780b199087e1368e4a9aa8c`. This matches the repository's
CI Flutter version. The unchanged fixture loads bundled Noto Sans and
MaterialIcons, uses English and device pixel ratio 1, and checks action
semantics and keyboard navigation before the portrait comparison.

From `src/frontend`, after the repository Windows SQLite prefetch step:

```powershell
flutter test --no-pub test/match_history_screen_test.dart --plain-name "MatchHistory actions remain usable and labeled in portrait and landscape" --update-goldens --reporter expanded
flutter test --no-pub test/match_history_screen_test.dart --reporter expanded
```

The initial ordinary comparison failed because the horizontal reference was
absent. After generating both Windows references, temporarily withholding only
the new vertical reference let the horizontal comparison pass and reproduced
the missing-vertical failure. Restoring that exact file made all 21 tests in
the file pass without `--update-goldens`.

| Viewport | Reference | SHA-256 |
| --- | --- | --- |
| 1280 x 800 | `match_history_horizontal.png` | `7d52ed097a37f8573696a610f1780c9369f3f974196a4bf5f393a3cec4321ae9` |
| 390 x 844 | `match_history_vertical.png` | `da6bdff3a475a507fba6b79088b474e7fd33c805622522e681c0c17e0bb27f98` |

The horizontal image shows the collapsed completed match and Find players.
The portrait image shows the expanded match, wrapped teammate name, Rate,
Report, Add friend and Find players. Both were visually inspected.
No Linux reference, test assertion, comparator tolerance or MatchSquad asset
was changed.
