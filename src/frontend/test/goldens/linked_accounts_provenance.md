# Linked accounts golden provenance

The root horizontal and vertical references remain the non-Linux captures. The
Linux CI horizontal comparison uses `linux/linked_accounts_h.png` because the
captured Linux render differs only at bundled-font/icon glyph edges while the
1280 x 800 layout and control geometry match the root reference. No tolerance or
widget behavior was changed.

The Linux horizontal image is the actual widget capture from GitHub Actions run
`40246736763`, job `121330825040`, at source `db7bdf3cf74c3ebaa010950b18dcb018fc3e7622`
on hosted Linux with Flutter 3.41.7. The CI expected image was byte-identical to
the committed root `linked_accounts_h.png` (SHA-256
`c73a8e070885318b3ce49fc60f945fe0dcefe1b00fa3e992732768dfbfdb9771`). The Linux
capture SHA-256 is `45fe338907dff76dca51acddc351f8767ee6c79120d40a276a3b53cc88339639`.

The vertical Linux comparison passed against the shared reference in that run,
so it continues to use the root vertical image. The failure was isolated to
horizontal text/icon rasterization; there was no overflow, clipping, position,
or size difference. This documents the observed CI capture and does not claim
the original root image's capture host.
