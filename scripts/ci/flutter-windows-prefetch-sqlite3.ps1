# Prefetch sqlite3mc native asset on Windows CI (network/cert flakiness).
param(
  [string]$FrontendDir = "src/frontend"
)

$ErrorActionPreference = "Stop"
Set-Location $FrontendDir

$url = "https://github.com/simolus3/sqlite3.dart/releases/download/sqlite3-3.3.3/sqlite3mc.x64.windows.dll"
$warmup = Join-Path $env:TEMP "sqlite3mc.x64.windows.dll"

for ($i = 1; $i -le 5; $i++) {
  Write-Host "Warming GitHub release download ($i/5): $url"
  try {
    Invoke-WebRequest -Uri $url -OutFile $warmup -TimeoutSec 180
    break
  } catch {
    if ($i -eq 5) { throw }
    Start-Sleep -Seconds 20
  }
}

for ($i = 1; $i -le 5; $i++) {
  Write-Host "flutter pub get for sqlite3 hook ($i/5)"
  flutter pub get
  if ($LASTEXITCODE -eq 0) { exit 0 }
  Start-Sleep -Seconds 20
}

exit 1
