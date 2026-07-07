# Apply file renames from scripts/dev/phase-rename-map.tsv
$ErrorActionPreference = 'Stop'
$root = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
if (-not (Test-Path (Join-Path $root 'Makefile'))) { $root = (Get-Location).Path }
Set-Location $root

$mapFile = Join-Path $PSScriptRoot 'phase-rename-map.tsv'
$lines = Get-Content $mapFile -Encoding UTF8 | Where-Object { $_ -and -not $_.StartsWith('#') }

foreach ($line in $lines) {
    $parts = $line -split "`t"
    if ($parts.Count -lt 2) { continue }
    $old = $parts[0].Trim()
    $new = $parts[1].Trim()
    $action = if ($parts.Count -ge 3) { $parts[2].Trim() } else { 'RENAME' }

    $oldPath = Join-Path $root $old
    if (-not (Test-Path $oldPath)) {
        Write-Host "SKIP missing: $old"
        continue
    }

    if ($action -eq 'DELETE') {
        Remove-Item -Force $oldPath
        Write-Host "DELETE $old"
        continue
    }

    $newPath = Join-Path $root $new
    $newDir = Split-Path $newPath -Parent
    if (-not (Test-Path $newDir)) { New-Item -ItemType Directory -Path $newDir -Force | Out-Null }
    git mv $old $new 2>$null
    if ($LASTEXITCODE -ne 0) {
        Move-Item -Force $oldPath $newPath
        Write-Host "MV (no git) $old -> $new"
    } else {
        Write-Host "GIT MV $old -> $new"
    }
}

Write-Host "Done renames."
