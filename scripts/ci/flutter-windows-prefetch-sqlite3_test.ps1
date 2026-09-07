# Regression tests for the Windows sqlite3mc prefetch helper. No Flutter or network required.
$ErrorActionPreference = "Stop"

$RepositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
$ProductionScript = Join-Path $RepositoryRoot "scripts/ci/flutter-windows-prefetch-sqlite3.ps1"
$TempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("voice-prefetch-sqlite3-test-" + [Guid]::NewGuid())

function Fail([string]$Message) {
  throw "FAIL: $Message"
}

function Assert-True([bool]$Condition, [string]$Message) {
  if (-not $Condition) { Fail $Message }
}

function New-FixtureRoot([string]$Name, [string]$FixtureContent) {
  $root = Join-Path $TempRoot $Name
  $frontend = Join-Path $root "src/frontend"
  New-Item -ItemType Directory -Force -Path $frontend | Out-Null
  [System.IO.File]::WriteAllText((Join-Path $frontend "sqlite3mc.x64.windows.dll"), $FixtureContent)
  return $root
}

function New-CandidateScript([string]$FixtureContent) {
  $sha256 = [System.Security.Cryptography.SHA256]::Create()
  try {
    $fixtureHash = ([BitConverter]::ToString($sha256.ComputeHash([System.Text.Encoding]::UTF8.GetBytes($FixtureContent)))).Replace("-", "").ToLowerInvariant()
  } finally {
    $sha256.Dispose()
  }
  $source = Get-Content -Raw -Encoding UTF8 $ProductionScript
  $source = $source.Replace(
    '$ExpectedHash = "1c8f8715063410769c0e6f67e0fec984621f7d7b1ca3572d21a71e62e2650988"',
    ('$ExpectedHash = "' + $fixtureHash + '"')
  )
  if ($source -notmatch [regex]::Escape($fixtureHash)) { Fail "test fixture could not replace the production SHA-256" }

  # This command must never be invoked. It models a Get-FileHash cmdlet that
  # is unavailable while retaining unrelated utility cmdlets used by the script.
  # The candidate is still launched exactly like production: -NoProfile -File.
  $preamble = @'
Import-Module Microsoft.PowerShell.Management -ErrorAction Stop
function global:Get-FileHash {
  throw "Get-FileHash is unavailable in this PowerShell environment"
}
function Invoke-WebRequest {
  throw "network access is disabled for this regression test"
}
function Start-Sleep {}
'@
  $source = $source.Replace('$ErrorActionPreference = "Stop"', ('$ErrorActionPreference = "Stop"' + [Environment]::NewLine + $preamble))
  $candidate = Join-Path $TempRoot "candidate.ps1"
  Set-Content -Encoding UTF8 -Path $candidate -Value $source
  return $candidate
}

function Invoke-Candidate([string]$Candidate, [string]$RootDir) {
  & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $Candidate -RootDir $RootDir | Out-Host
  return $LASTEXITCODE
}

try {
  New-Item -ItemType Directory -Force -Path $TempRoot | Out-Null
  $fixture = "verified sqlite3 fixture`n"
  $candidate = New-CandidateScript $fixture

  $bin = Join-Path $TempRoot "bin"
  New-Item -ItemType Directory -Force -Path $bin | Out-Null
  $flutterLog = Join-Path $TempRoot "flutter.log"
  @'
@echo off
echo %* >> "%VOICE_PREFETCH_FLUTTER_LOG%"
exit /b 0
'@ | Set-Content -Encoding ASCII -Path (Join-Path $bin "flutter.cmd")

  $originalPath = $env:PATH
  $originalTemp = $env:TEMP
  $env:PATH = "$bin;$originalPath"
  $env:TEMP = $TempRoot
  $env:VOICE_PREFETCH_FLUTTER_LOG = $flutterLog
  try {
    Write-Host "== verified cached asset succeeds without Get-FileHash =="
    $validRoot = New-FixtureRoot "valid" $fixture
    [System.IO.File]::WriteAllText((Join-Path $TempRoot "voice-sqlite3mc.x64.windows.dll"), $fixture)
    $exitCode = Invoke-Candidate $candidate $validRoot
    Assert-True ($exitCode -eq 0) "powershell -NoProfile -File must succeed for a valid cached asset when Get-FileHash is unavailable"
    Assert-True (Test-Path $flutterLog) "successful verification must continue to flutter pub get"
    Assert-True ((Get-Content -Raw $flutterLog) -match '(?m)^pub get\s*$') "successful verification must invoke flutter pub get"

    Write-Host "== tampered cached asset remains rejected =="
    Remove-Item -Force $flutterLog
    $tamperedRoot = New-FixtureRoot "tampered" "tampered sqlite3 fixture`n"
    [System.IO.File]::WriteAllText((Join-Path $TempRoot "voice-sqlite3mc.x64.windows.dll"), "tampered sqlite3 fixture`n")
    $exitCode = Invoke-Candidate $candidate $tamperedRoot
    Assert-True ($exitCode -ne 0) "tampered cached asset must fail SHA-256 verification"
    Assert-True (-not (Test-Path $flutterLog)) "Flutter must not run after strict SHA-256 verification fails"
  } finally {
    $env:PATH = $originalPath
    $env:TEMP = $originalTemp
    Remove-Item Env:VOICE_PREFETCH_FLUTTER_LOG -ErrorAction SilentlyContinue
  }

  Write-Host "All Windows sqlite3mc prefetch tests passed."
} finally {
  Remove-Item -Recurse -Force $TempRoot -ErrorAction SilentlyContinue
}
