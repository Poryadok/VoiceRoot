# MatchSquad native golden provenance

Windows references live in `windows/`; other hosts retain the original root
references. Only `Platform.isWindows` changes the lookup path. The comparator,
font loader, theme, locale, viewports and behavioral assertions are unchanged.

## Diagnosis

On Windows 11 x64 build 26200, three bounded tests reproduced the same failures
on current master `258b48d0b4506e118b833a194642142fb5847c2e` and PR925
`bacfefc84e54e7c7c59e41a1b923040667394615`: horizontal 7097 pixels (0.69%),
vertical 6964 pixels (2.12%), keyboard focus 6295 pixels (0.61%). Each actual
PNG is byte-identical across those heads and the independent PR925 full-suite
failure artifact pack. Thus these failures are not a PR925 application regression.

The shared reference comparison differs around glyph edges/advances and the
resulting button outlines, with the same overall card, roster and footer
geometry. All three Windows captures were visually inspected: readable labels,
no clipped controls, vertical action wrapping, and the visible Join voice focus
ring. The composition retains the controls in `docs/design/screen-controls.md`
section 6.5; the canonical screen is registered in `docs/design/screens.md`.
This is platform fixture repair, not a new design or full product acceptance.

The original references passed these same three comparisons on Ubuntu24.04 in
[PR858's Flutter job](https://github.com/Poryadok/VoiceRoot/actions/runs/39127552497/job/117675820545).
That is historical Linux evidence. Current-master Flutter was skipped in run
40253446279; no fresh Linux execution is claimed here. Existing root image
bytes remain unchanged, preserving the Linux gate and its subsequent CI check.

## Capture environment

Captured 2026-10-26 from master258b48d0 with only Windows reference routing
added. Flutter3.41.7 stable, framework
`cc0734ac716fbb8b90f3f9db8020958b1553afa7`, Dart3.11.5,
engine `59aa584fdf100e6c78c785d8a5b565d1de4b48ab`, engine content hash
`7a53c052bc4b472cf780b199087e1368e4a9aa8c`.
Fixture: English, DPR1, bundled Noto Sans and SDK MaterialIcons, unchanged
`voiceTestTheme` and deterministic profiles. No live network is involved.

| Windows file | Dimensions | SHA-256 |
| --- | --- | --- |
| `windows/match_squad_horizontal.png` | 1280x800 | `9a6e0c2d81c6a7ebd6626fb872992477a1baeec18c25a4fb56e41aec39776c0f` |
| `windows/match_squad_vertical.png` | 390x844 | `6b4ee96f1095e976d4a865e13c3d92849585620de0931dc1257a94dfbd664b2d` |
| `windows/match_squad_keyboard_focus.png` | 1280x800 | `b22b7efda6bb0653d88dd9ef3a20e825162174585d00d465cdee7491baca1230` |

Font SHA-256:

| Font | SHA-256 |
| --- | --- |
| NotoSans-Regular.ttf | `b85c38ecea8a7cfb39c24e395a4007474fa5a4fc864f6ee33309eb4948d232d5` |
| NotoSans-Medium.ttf | `7bbe267354704c6ad18bde24b1dbc756c8e4380ca1c3f3c25c45ec5c4471510b` |
| NotoSans-SemiBold.ttf | `87a8b90ece1e89746b544e4e086f85a3710e41485a8078f9be874837dfad45d5` |
| NotoSans-Bold.ttf | `c976e4b1b99edc88775377fcc21692ca4bfa46b6d6ca6522bfda505b28ff9d6a` |
| MaterialIcons-Regular.otf | `d9865b671a09d683d13a863089d8825e0f61a37696ce5d7d448bc8023aa62453` |

## Reproduction

From `src/frontend`, run the bounded three-test probe:

```powershell
flutter test --no-pub --reporter expanded --concurrency=1 test/match_squad_screen_test.dart --name "layout matches its screenshot|controls expose labels and keyboard focus"
```

Before repair: 0 passed, 3 golden failures on both heads. After the explicit
Windows capture using the same command with `--update-goldens`, the ordinary
command passed all3 without that flag. Newly captured files exactly match both
pre-repair actual renders. No skip, tolerance increase, semantic assertion
removal, application change or Linux baseline replacement was used.
