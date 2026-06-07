$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$OutDir = Join-Path $Root ".local"
$OutFile = Join-Path $OutDir "compose.log"
$Services = @("gateway", "messaging", "chat", "realtime", "user", "social", "voice", "file", "auth")

New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
Set-Location $Root

$args = @("compose", "logs", "--no-color", "--timestamps") + $Services
docker @args | Set-Content -Encoding utf8 $OutFile

Write-Host "Wrote $OutFile"
