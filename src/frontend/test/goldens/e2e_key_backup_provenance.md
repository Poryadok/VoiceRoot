# E2E key backup platform golden provenance

These references were captured by the source-triggered golden producer in run
`37853521307` from source `9ccc3c11a9c4414bf01b13fbd3ba0f20d7fb9427`, using
`flutter test test/e2e_key_backup_settings_test.dart`. Both initial runs
reported the references as missing and preserved the pre-comparison PNGs as
actual captures. The routed capture harness includes the documented Back
control and asserts it in both viewport cases.

Both runners used Flutter `3.41.7` stable (framework revision `cc0734ac71`),
Dart `3.11.5`, engine hash `7a53c052bc4b472cf780b199087e1368e4a9aa8c`
(revision `59aa584fdf`). The Linux runner was `linux_x64`; the Windows runner
was `windows_x64`.

| Platform | Viewport | Reference | SHA-256 |
| --- | ---: | --- | --- |
| Linux | 1280×800 | `linux/e2e_key_backup_h.png` | `4e56480d255ec889154f62dcb64e3d5d5ad22b97bcac1881685b5b8943b0489d` |
| Linux | 390×844 | `linux/e2e_key_backup_v.png` | `4e2a0201a656d37ab1d35ac1834829c811d4cdd5832a20f3ab81a26daa894b3b` |
| Windows | 1280×800 | `windows/e2e_key_backup_h.png` | `33de2bcc765f5b1cedb63c54c030fe1e61b583bce069cfc985b63c856758d485` |
| Windows | 390×844 | `windows/e2e_key_backup_v.png` | `4a9b06d9ecdfd1221007fd703b4c0d7164f37a2f65a20b3d69d832ad922c08bf` |

The producer's retained provenance files are under
`RUN/CR23-captures-9ccc-linux/` and `RUN/CR23-captures-9ccc-windows/`. Recompute
hashes with `sha256sum` on Linux or `Get-FileHash -Algorithm SHA256` on Windows.
The subsequent source-triggered Linux and Windows comparisons, including the
changed-hint negative case, remain the acceptance check.
