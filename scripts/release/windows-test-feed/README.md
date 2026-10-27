# Disposable Windows update feed preparation

This standalone Go 1.26 stdlib tool prepares the package/feed input for the
isolated Windows A5 runner in `docs/PLAN.md`. Its consumer is that runner, not
the production Gateway or a user's installed Voice. It never launches an
installer, imports certificates, starts a server, modifies sessions or changes
the Flutter updater. Its output is **offline test package integrity only**.

Use an already built Inno installer from `scripts/release/windows-build.ps1`.
Resolve the exact source independently from the build provenance; the tool
cannot establish that installer bytes came from the declared commit. For two
versions, retain their separate source SHAs and reuse one disposable test key.
All commands below run from the repository root. Keep the seed outside the feed
directory, under the ignored session directory. The tool refuses key overwrite.
Windows file modes do not establish an ACL: use a disposable runner account and
discard the runner/key afterward; never provide a production key.

```powershell
New-Item -ItemType Directory -Force tmp/windows-feed | Out-Null
Push-Location scripts/release/windows-test-feed
go build -o ../../../tmp/windows-feed/windows-test-feed.exe .
Pop-Location
$feedTool = Resolve-Path tmp/windows-feed/windows-test-feed.exe
$testPublicKey = & $feedTool keygen --key tmp/windows-feed/test.seed
# Supply independently verified paths/source values for each already-built version.
$bundle = & $feedTool build --key tmp/windows-feed/test.seed `
  --installer $InstallerPath --out tmp/windows-feed/public `
  --version $Version --source-sha $BuildSourceSha `
  --base-url http://127.0.0.1:18948
& $feedTool verify --bundle $bundle --source-sha $BuildSourceSha `
  --public-key $testPublicKey
# In the isolated runner only; bind explicitly to loopback, never serve the seed.
python -m http.server 18948 --bind 127.0.0.1 --directory tmp/windows-feed/public
```

Each bundle occupies `<installer-sha256>/` exclusively and contains
`VoiceSetup.exe`, `appcast.xml`, canonical `manifest.json`, and detached base64
`manifest.sig`. The manifest records exact source/version/architecture, installer
URL/size/hash/Ed25519 signature and appcast hash. The manifest itself is signed.
Verification requires an independently pinned public key and source SHA; it
rejects tampered members, unknown/noncanonical metadata, wrong identity/source,
architecture changes, and feed substitution. The public key is printed by keygen
and must be pinned before package consumption, never trusted from a fetched feed.
An existing hash directory is never overwritten, including an incomplete failed
publication. Stage and verify the complete directory before serving it; the
tool provides content addressing and overwrite prevention, not filesystem/WORM
access control or atomic visibility to a running web server.

The appcast enclosure uses `sparkle:edSignature`, the signature of installer
bytes, and `sparkle:os="windows-x64"`. See the primary
[WinSparkle setup guide](https://winsparkle.org/guides/getting-started/),
[integration guide](https://winsparkle.org/guides/integrating-winsparkle/), and
[publishing guide](https://winsparkle.org/guides/publishing-updates/).
The bundle format includes hashes/source identity suitable for release evidence;
this test CLI deliberately permits only loopback HTTP origins. Production feeds
require HTTPS, protected immutable storage and managed production signing.
Ed25519 update signing does not confer Windows Authenticode trust.

The current Flutter scaffold passes Gateway `update_url` to WinSparkle. This CLI
does not embed its test public key in that app, activate that URL, or assert the
pinned plugin's native signature compatibility. Configure the public key in the
isolated test build before a native update test, using that plugin's verified
WinSparkle version and native key resource/API. Do not publish the loopback URL
as a product destination. Synthetic unit fixture bytes are not installers.

Required remaining native evidence: exact Windows OS/build/arch and previous/new
artifact identities; disposable account/trust custody; real installer signature
verification by pinned WinSparkle; clean install/cold start; old-to-new update;
corrupted download rejection/fallback to the usable old install; preserved
session and network reconnect, using actual app/backend observations. Record
each failure honestly. A Go test PASS does not satisfy PL-A5-01/A5/G1 acceptance
or publicly trusted zero-warning installation. Never import a test identity into
the host's trust store or update the host's Voice to collect this evidence.

```powershell
Push-Location scripts/release/windows-test-feed
go test ./...
go vet ./...
Pop-Location
```
