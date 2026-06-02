# Compare canonical design tokens with Flutter asset copy.
$ErrorActionPreference = "Stop"
$Root = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
if (-not $Root) { $Root = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path }
$Canon = Join-Path $Root "design\tokens\voice.tokens.json"
$Asset = Join-Path $Root "src\frontend\assets\design\voice.tokens.json"
if (-not (Test-Path $Canon)) { throw "missing canonical tokens: $Canon" }
if (-not (Test-Path $Asset)) { throw "missing Flutter asset tokens: $Asset" }
$HashCanon = (Get-FileHash -Algorithm SHA256 -Path $Canon).Hash
$HashAsset = (Get-FileHash -Algorithm SHA256 -Path $Asset).Hash
if ($HashCanon -ne $HashAsset) {
  Write-Error @"
design tokens out of sync:
  canonical: $Canon
  asset:     $Asset
Copy: Copy-Item design\tokens\voice.tokens.json src\frontend\assets\design\voice.tokens.json
"@
}
Write-Host "design tokens: canonical and asset match"
