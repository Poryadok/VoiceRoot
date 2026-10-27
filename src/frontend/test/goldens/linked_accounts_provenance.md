# Linked accounts golden provenance

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
