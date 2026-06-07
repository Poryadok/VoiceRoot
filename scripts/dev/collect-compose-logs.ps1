$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$OutDir = Join-Path $Root ".local"
$OutFile = Join-Path $OutDir "compose.log"
$NdjsonFile = Join-Path $OutDir "dev.ndjson"
$Services = @("gateway", "messaging", "chat", "realtime", "user", "social", "voice", "file", "auth")

New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
Set-Location $Root

$args = @("compose", "logs", "--no-color", "--timestamps") + $Services
docker @args | Set-Content -Encoding utf8 $OutFile

$ndjsonLines = Get-Content $OutFile | ForEach-Object {
    $idx = $_.IndexOf('| {')
    if ($idx -ge 0) {
        $_.Substring($idx + 2).TrimStart()
    }
}
$ndjsonLines | Set-Content -Encoding utf8 $NdjsonFile

Write-Host "Wrote $OutFile"
Write-Host "Wrote $NdjsonFile"
