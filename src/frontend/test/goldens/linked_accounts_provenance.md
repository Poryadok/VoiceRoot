# Linked accounts golden provenance

## BATCH-C2 composed destination capture

The current references represent the explicit composition of #906's linked-account
panel and #913's implemented verification-status destination. The former disabled
status placeholder becomes an enabled action with its status label; organization
verification remains unavailable. Actual and isolated-diff inspection on Windows
and Linux showed the changed destination labels/control rendering, preserving
the panel bounds, profile label, linked account, unlink and connect controls.

Captured locally in owned WT96 at composed source `d4486634a589` plus the reviewed
composition delta, using Flutter 3.41.7. Windows uses the local SDK; Linux uses
`ghcr.io/cirruslabs/flutter:3.41.7`. Both retain actual MaterialIcons/Noto Sans
loading. Root H/V and Linux H/V were captured separately, not copied across hosts.
The exact comparator, test assertions and timeouts remain unchanged.

Before updating references, the four-file Windows run passed 75 checks and failed
only the two screenshots. Original actual/expected/isolated images and raw logs
are preserved in `tmp/swarm/anton148-batch-c2/windows-golden-red/` and
`linux-golden-red/`. The Linux full focused run subsequently passed all 77 tests;
Windows H/V capture passed both tests. These are component results, not aggregate
CI, provider or runtime acceptance.

| Current reference | SHA-256 |
| --- | --- |
| `linked_accounts_h.png` | `0b16c31bfb40af1245d23133479c1b5c7e2581876568988a17a6f0291226ea5c` |
| `linked_accounts_v.png` | `328777943f8e2f3af06ad9a6deabc085106b0b639e726899430e9e43a4a66052` |
| `linux/linked_accounts_h.png` | `db77c8f5bc5700122cd04d52737a23c8f0b6cccab7421102c66f2747b2ab7b23` |
| `linux/linked_accounts_v.png` | `87bc8c7c0b67e25b97bee7a5519aa7e33abf31be951d7b861e7559956a07a215` |

## Historical predecessor captures (superseded references)

The following records describe the original #906 platform qualification. Their
hashes identify historical references, not the composed current images above.


The root horizontal and vertical references remain unchanged; this evidence does
not establish their capture host. Linux comparisons use explicitly captured
`linux/linked_accounts_h.png` and `linux/linked_accounts_v.png`. Their observed
text/icon rendering differs from the root references while layout and control
geometry match. No tolerance, comparator, assertion, timeout or widget behavior
was changed. A specific lower-level font/engine/host cause is not established.

The Linux horizontal image is the actual widget capture from GitHub Actions run
`40246736763`, job `121330825040`, at source `db7bdf3cf74c3ebaa010950b18dcb018fc3e7622`
on hosted Linux with Flutter 3.41.7. The CI expected image was byte-identical to
the committed root `linked_accounts_h.png` (SHA-256
`c73a8e070885318b3ce49fc60f945fe0dcefe1b00fa3e992732768dfbfdb9771`). The Linux
capture SHA-256 is `45fe338907dff76dca51acddc351f8767ee6c79120d40a276a3b53cc88339639`.

The vertical Linux comparison was not reached in that earlier run. Horizontal and
vertical golden assertions were inside one widget-test callback, so the
horizontal mismatch aborted the callback before the vertical assertion. No
vertical result is inferred from that job. Subsequent separate widget tests and
the per-test font Future fix allowed the current Linux vertical comparator to
run; the earlier Windows successes did not qualify a Linux vertical reference.

## Vertical Linux capture

`linux/linked_accounts_v.png` is byte-identical to the inspected actual capture
`linked_accounts_v_testImage.png` from GitHub Actions run `40400058452`, job
`121875971189`, artifact `12696643322`
(`flutter-golden-failures-0-87b07a8be29732363d573b2abc8f2fb0022edcca`).
The source head was `5a83e107b13d1c9367b19e315f95c0a51cb462f5`, base
`667a2aebb77902d22f1286ce669d7a212acd8859`, CI merge
`87b07a8be29732363d573b2abc8f2fb0022edcca`, with Flutter 3.41.7 on hosted Linux.
The vertical comparison reached `LocalFileComparator` and reported
`2.96%, 9736px` against the root reference; it did not time out.

Both inspected images are 390 x 844 with matching control bounds and text
wrapping, and no observed missing controls or overflow. Differences are
concentrated in text/icon rendering and centered label metrics. The test, UI,
font, theme fixture and pubspec entries were verified identical between that
source head and the CI merge. The captured Linux vertical SHA-256 is
`06860b96f6f30988d62f043db7ea636588eb50c3e3dc664579b0d3aef3120759`.
The unchanged root vertical SHA-256 is
`4593abf4d603b35ca9ea210b1da2c2b584fae1c1b1edfcc3fb6597364243f7db`.
This records capture provenance, not a successful whole-shard or whole-PR result.
