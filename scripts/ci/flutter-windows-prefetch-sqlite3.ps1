# Prefetch sqlite3mc native asset on Windows (local make flutter-ci + CI job flutter-windows).
# Uses pubspec hook source test-sqlite3mc -> src/frontend/.dart_tool/sqlite3-prefetch/
param(
  [string]$RootDir = ""
)

$ErrorActionPreference = "Stop"

if (-not $RootDir) {
  $RootDir = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
}

$FrontendDir = Join-Path $RootDir "src/frontend"
$ReleaseTag = "sqlite3-3.3.3"
$AssetName = "sqlite3mc.x64.windows.dll"
$ExpectedHash = "1c8f8715063410769c0e6f67e0fec984621f7d7b1ca3572d21a71e62e2650988"
$Url = "https://github.com/simolus3/sqlite3.dart/releases/download/$ReleaseTag/$AssetName"
$PrefetchDir = $FrontendDir
$DestAsset = Join-Path $PrefetchDir $AssetName
$CachedDownload = Join-Path $env:TEMP "voice-$AssetName"

function Test-FileSha256([string]$Path, [string]$Expected) {
  $actual = (Get-FileHash -Path $Path -Algorithm SHA256).Hash.ToLower()
  return $actual -eq $Expected.ToLower()
}

function Get-VerifiedSqliteDll {
  if ((Test-Path $CachedDownload) -and (Test-FileSha256 $CachedDownload $ExpectedHash)) {
    Write-Host "Reusing verified download: $CachedDownload"
    return $CachedDownload
  }

  for ($i = 1; $i -le 5; $i++) {
    Write-Host "Downloading sqlite3mc ($i/5): $Url"
    try {
      if (Test-Path $CachedDownload) { Remove-Item $CachedDownload -Force }
      Invoke-WebRequest -Uri $Url -OutFile $CachedDownload -TimeoutSec 180
      if (Test-FileSha256 $CachedDownload $ExpectedHash) {
        return $CachedDownload
      }
      Write-Host "Hash mismatch on downloaded $AssetName (attempt $i/5)"
    } catch {
      Write-Host "Download failed (attempt $i/5): $_"
      if ($i -eq 5) { throw }
    }
    Start-Sleep -Seconds 20
  }

  throw "Failed to download valid $AssetName after 5 attempts"
}

New-Item -ItemType Directory -Force -Path $PrefetchDir | Out-Null
$verifiedDll = Get-VerifiedSqliteDll
if ((Test-Path $DestAsset) -and (Test-FileSha256 $DestAsset $ExpectedHash)) {
  Write-Host "sqlite3 prefetch cache hit: $DestAsset"
} else {
  Copy-Item $verifiedDll $DestAsset -Force
  Write-Host "Seeded $DestAsset"
}

Push-Location $FrontendDir
try {
  flutter pub get
  exit $LASTEXITCODE
} finally {
  Pop-Location
}
